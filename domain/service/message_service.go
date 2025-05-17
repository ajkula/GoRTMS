package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrDomainNotFound     = errors.New("domain not found")
	ErrQueueNotFound      = errors.New("queue not found")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrSubscriptionFailed = errors.New("subscription failed")
)

var _ model.MessageProvider = (*MessageServiceImpl)(nil)

// MessageServiceImpl implémente le service des messages
type MessageServiceImpl struct {
	domainRepo        outbound.DomainRepository
	messageRepo       outbound.MessageRepository
	consumerGroupRepo outbound.ConsumerGroupRepository
	subscriptionReg   outbound.SubscriptionRegistry
	queueService      inbound.QueueService
	statsService      inbound.StatsService
	rootCtx           context.Context

	// Compteur pour le nettoyage périodique
	messageCountSinceLastCleanup int
	cleanupMu                    sync.Mutex
}

// NewMessageService crée un nouveau service de messages
func NewMessageService(
	domainRepo outbound.DomainRepository,
	messageRepo outbound.MessageRepository,
	consumerGroupRepo outbound.ConsumerGroupRepository,
	subscriptionReg outbound.SubscriptionRegistry,
	queueService inbound.QueueService,
	rootCtx context.Context,
	statsService ...inbound.StatsService,
) inbound.MessageService {
	impl := &MessageServiceImpl{
		domainRepo:        domainRepo,
		messageRepo:       messageRepo,
		consumerGroupRepo: consumerGroupRepo,
		subscriptionReg:   subscriptionReg,
		queueService:      queueService,
		rootCtx:           rootCtx,
	}

	if len(statsService) > 0 {
		impl.statsService = statsService[0]
	}

	// Démarrer les tâches de nettoyage
	impl.startCleanupTasks(rootCtx)

	return impl
}

// PublishMessage publie un message dans une file d'attente
func (s *MessageServiceImpl) PublishMessage(
	domainName, queueName string,
	message *model.Message,
) error {
	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(s.rootCtx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	// Vérifier si la file d'attente existe
	channelQueue, err := s.queueService.GetChannelQueue(s.rootCtx, domainName, queueName)
	if err != nil {
		return ErrQueueNotFound
	}

	// Valider le message si un schéma est défini
	if domain.Schema != nil && domain.Schema.Validation != nil {
		if err := domain.Schema.Validation(message.Payload); err != nil {
			return ErrInvalidMessage
		}
	} else if domain.Schema != nil && len(domain.Schema.Fields) > 0 {
		// Validation basée sur les champs si aucune fonction de validation personnalisée
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return ErrInvalidMessage
		}

		// Vérifier chaque champ requis
		for fieldName, fieldType := range domain.Schema.Fields {
			fieldValue, exists := payload[fieldName]
			if !exists {
				return ErrInvalidMessage
			}

			// Vérification simplifiée du type
			switch fieldType {
			case model.StringType:
				if _, ok := fieldValue.(string); !ok {
					return ErrInvalidMessage
				}
			case model.NumberType:
				if _, ok := fieldValue.(float64); !ok {
					return ErrInvalidMessage
				}
			case model.BooleanType:
				if _, ok := fieldValue.(bool); !ok {
					return ErrInvalidMessage
				}
			}
		}
	}

	// Ajouter les métadonnées
	if message.Metadata == nil {
		message.Metadata = make(map[string]interface{})
	}
	message.Metadata["domain"] = domainName
	message.Metadata["queue"] = queueName

	// Définir l'horodatage
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// Stocker le message dans le repository pour persistance/historique
	if err := s.messageRepo.StoreMessage(s.rootCtx, domainName, queueName, message); err != nil {
		return err
	}

	// Collecter des statistiques
	if s.statsService != nil {
		s.statsService.TrackMessagePublished(domainName, queueName)
	}

	// Enqueue le message dans la chan queue
	// Essayer d'insérer sans bloquer, ignorer les erreurs de queue pleine
	_ = channelQueue.Enqueue(s.rootCtx, message)

	// Notifier les abonnés via le registry existant
	_ = s.subscriptionReg.NotifySubscribers(domainName, queueName, message)

	// Appliquer les règles de routage si nécessaire
	if routes, exists := domain.Routes[queueName]; exists {
		for destQueue, rule := range routes {
			// Convertir le prédicat selon son type
			var match bool

			switch pred := rule.Predicate.(type) {
			case model.PredicateFunc:
				// Utiliser directement la fonction
				match = pred(message)
			case model.JSONPredicate:
				// Évaluer le prédicat JSON
				match = s.evaluateJSONPredicate(pred, message)
			case map[string]any:
				// Convertir la map en JSONPredicate
				jsonPred := model.JSONPredicate{
					Type:  fmt.Sprintf("%v", pred["type"]),
					Field: fmt.Sprintf("%v", pred["field"]),
					Value: pred["value"],
				}
				match = s.evaluateJSONPredicate(jsonPred, message)
			default:
				log.Printf("Unknown predicate type: %T", rule.Predicate)
			}

			if match {
				// Publier le message dans la file de destination
				destMsg := *message // Copie du message
				if err := s.PublishMessage(domainName, destQueue, &destMsg); err != nil {
					return err
				}
			}
		}
	} else {
		log.Printf("No routes found for queue %s", queueName)
	}

	return nil
}

// ConsumeMessageWithGroup consomme avec gestion des groupes
func (s *MessageServiceImpl) ConsumeMessageWithGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
	options *inbound.ConsumeOptions,
) (*model.Message, error) {
	if options == nil {
		options = &inbound.ConsumeOptions{}
	}

	// Récupérer la channelQueue
	channelQueue, err := s.queueService.GetChannelQueue(ctx, domainName, queueName)
	if err != nil {
		return nil, err
	}

	// Cast vers ChannelQueue pour accéder aux méthodes spécifiques
	chQueue, ok := channelQueue.(*model.ChannelQueue)
	if !ok {
		return nil, errors.New("unexpected queue type")
	}

	// Obtenir la position
	position, err := s.consumerGroupRepo.GetPosition(ctx, domainName, queueName, groupID)
	if err != nil {
		log.Printf("Error getting position: %v", err)
	}

	// Enregistrer le consumer group dans la channel queue
	chQueue.AddConsumerGroup(groupID, position)

	// Enregistrer le consommateur dans le repository
	if options != nil && options.ConsumerID != "" {
		_ = s.consumerGroupRepo.RegisterConsumer(ctx, domainName, queueName, groupID, options.ConsumerID)
	}

	// Définir le timeout
	timeout := 1 * time.Second
	if options != nil && options.Timeout > 0 {
		timeout = options.Timeout
	}

	// Vérifier si des messages sont déjà dans le canal du groupe
	message, err := chQueue.ConsumeMessage(groupID, 50*time.Millisecond)
	if err != nil {
		log.Printf("chQueue.ConsumeMessage(%s, 50*time.Millisecond) ERR: %v", groupID, err)
	}
	if message == nil {
		// Si aucun message dans le canal, demander d'en récupérer
		channelQueue.RequestMessages(groupID, 5)

		// Puis attendre un message avec le timeout complet
		message, err = chQueue.ConsumeMessage(groupID, timeout)
		if err != nil {
			log.Printf("chQueue.ConsumeMessage(%s, %d) ERR: %v", groupID, timeout, err)
		}
	}

	// Si toujours aucun message, essayer directement depuis le repository
	if message == nil {
		messages, err := s.messageRepo.GetMessagesAfterIndex(ctx, domainName, queueName, position, 1)
		if err != nil || len(messages) == 0 {
			return nil, err
		}
		message = messages[0]

		// Alimenter le canal du groupe avec d'autres messages
		if len(messages) > 1 {
			_ = chQueue.RequestMessages(groupID, 10)
		}
	}

	// Si un message a été trouvé, l'acquitter et mettre à jour la position
	if message != nil {
		// Mettre à jour le timestamp d'activité du groupe
		if repo, ok := s.consumerGroupRepo.(interface {
			UpdateLastActivity(ctx context.Context, domainName, queueName, groupID string) error
		}); ok {
			if err = repo.UpdateLastActivity(ctx, domainName, queueName, groupID); err != nil {
				log.Printf("Error updating last activity: %v", err)
			}
		}

		// Trouver l'index du message
		index, err := s.messageRepo.GetIndexByMessageID(ctx, domainName, queueName, message.ID)
		if err != nil {
			log.Printf("domainName=%s queueName=%s message.ID=%s Err: %v", domainName, queueName, message.ID, err)
		} else {
			// Stocker l'index du prochain message comme position
			newPosition := index + 1
			if err := s.consumerGroupRepo.StorePosition(ctx, domainName, queueName, groupID, newPosition); err != nil {
				log.Printf("Position not stored for group %s: %d, Err: %v", groupID, newPosition, err)
				return nil, err
			}

			// IMPORTANT: Mettre à jour la position interne APRÈS avoir stocké dans le repository
			chQueue.UpdateConsumerGroupPosition(groupID, newPosition)
		}

		// Acquitter automatiquement
		fullyAcked, err := s.messageRepo.AcknowledgeMessage(ctx, domainName, queueName, groupID, message.ID)
		if err != nil {
			log.Printf("Ack problem: %s", message.ID)
		}

		// Si complètement acquitté, supprimer
		if fullyAcked {
			if err := s.messageRepo.DeleteMessage(ctx, domainName, queueName, message.ID); err != nil {
				// Ignorer spécifiquement l'erreur "message not found"
				if err.Error() == "message not found" {
					log.Printf("Message already deleted: %s", message.ID)
				} else {
					log.Printf("Message not deleted: %s, Err: %v", message.ID, err)
				}
			} else {
				log.Printf("Message deleted: %s", message.ID)
			}
		}

		// Mettre à jour les statistiques
		if s.statsService != nil {
			s.statsService.TrackMessageConsumed(domainName, queueName)
		}

		// Incrémenter le compteur de manière thread-safe
		s.cleanupMu.Lock()
		s.messageCountSinceLastCleanup++
		shouldCleanup := s.messageCountSinceLastCleanup >= 100 // Intervalle de nettoyage
		if shouldCleanup {
			s.messageCountSinceLastCleanup = 0
		}
		s.cleanupMu.Unlock()

		// Si le seuil est atteint, nettoyer les indices
		if shouldCleanup {
			// Trouver la position minimum parmi tous les groupes
			minPosition := int64(math.MaxInt64)
			groups, err := s.consumerGroupRepo.ListGroups(ctx, domainName, queueName)
			if err == nil && len(groups) > 0 {
				for _, gID := range groups {
					pos, err := s.consumerGroupRepo.GetPosition(ctx, domainName, queueName, gID)
					if err == nil && pos < minPosition && pos > 0 {
						minPosition = pos
					}
				}

				// Si une position minimum valide a été trouvée
				if minPosition < int64(math.MaxInt64) {
					// Garder une marge de sécurité
					safePosition := minPosition - 10
					if safePosition > 0 {
						s.messageRepo.CleanupMessageIndices(ctx, domainName, queueName, safePosition)
					}
				}
			}
		}
	}

	return message, nil
}

func (s *MessageServiceImpl) GetMessagesAfterIndex(
	ctx context.Context,
	domainName, queueName string,
	startIndex int64,
	limit int,
) ([]*model.Message, error) {
	// Simplement déléguer au repository
	return s.messageRepo.GetMessagesAfterIndex(ctx, domainName, queueName, startIndex, limit)
}

// SubscribeToQueue s'abonne à une file d'attente
func (s *MessageServiceImpl) SubscribeToQueue(
	domainName, queueName string,
	handler model.MessageHandler,
) (string, error) {
	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(s.rootCtx, domainName)
	if err != nil {
		return "", ErrDomainNotFound
	}

	// Vérifier si la file d'attente existe
	if _, exists := domain.Queues[queueName]; !exists {
		return "", ErrQueueNotFound
	}

	// Enregistrer l'abonnement
	subscriptionID, err := s.subscriptionReg.RegisterSubscription(domainName, queueName, handler)
	if err != nil {
		return "", ErrSubscriptionFailed
	}

	return subscriptionID, nil
}

// UnsubscribeFromQueue se désinscrit d'une file d'attente
func (s *MessageServiceImpl) UnsubscribeFromQueue(
	domainName, queueName string,
	subscriptionID string,
) error {
	// Supprimer l'abonnement
	return s.subscriptionReg.UnregisterSubscription(subscriptionID)
}

// evaluateJSONPredicate évalue un prédicat JSON sur un message
func (s *MessageServiceImpl) evaluateJSONPredicate(predicate model.JSONPredicate, message *model.Message) bool {
	// Décoder le payload
	var payload map[string]interface{}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return false
	}

	// Récupérer la valeur du champ
	fieldValue, exists := payload[predicate.Field]
	if !exists {
		return false
	}

	// Évaluer selon le type d'opération
	switch predicate.Type {
	case "eq": // Égalité
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", predicate.Value)
	case "ne": // Inégalité
		return fieldValue != predicate.Value
	case "gt": // Supérieur
		switch v := fieldValue.(type) {
		case float64:
			if pv, ok := predicate.Value.(float64); ok {
				return v > pv
			}
		}
	case "lt": // Inférieur
		switch v := fieldValue.(type) {
		case float64:
			if pv, ok := predicate.Value.(float64); ok {
				return v < pv
			}
		}
	case "contains": // Contient (pour les chaînes)
		switch v := fieldValue.(type) {
		case string:
			if pv, ok := predicate.Value.(string); ok {
				return strings.Contains(v, pv)
			}
		}
	}

	return false
}

func (s *MessageServiceImpl) startCleanupTasks(ctx context.Context) {
	// Structure pour suivre la dernière fois qu'une queue a été vue sans consumer groups
	type QueueInactivity struct {
		firstEmptyTime time.Time
		checked        bool
	}

	queueInactivity := make(map[string]map[string]*QueueInactivity) // domainName -> queueName -> inactivity
	var inactivityMu sync.Mutex

	// Nettoyer les messages orphelins périodiquement
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				inactivityMu.Lock()

				// Parcourir les domaines et queues
				domains, err := s.domainRepo.ListDomains(ctx)
				if err != nil {
					inactivityMu.Unlock()
					continue
				}

				for _, domain := range domains {
					// Initialiser la map pour ce domaine si nécessaire
					if _, exists := queueInactivity[domain.Name]; !exists {
						queueInactivity[domain.Name] = make(map[string]*QueueInactivity)
					}

					for queueName := range domain.Queues {
						// Initialiser l'entrée pour cette queue si nécessaire
						if _, exists := queueInactivity[domain.Name][queueName]; !exists {
							queueInactivity[domain.Name][queueName] = &QueueInactivity{}
						}

						inactivityInfo := queueInactivity[domain.Name][queueName]

						// Vérifier s'il existe des consumer groups
						groupIDs, err := s.consumerGroupRepo.ListGroups(ctx, domain.Name, queueName)

						if err == nil && len(groupIDs) > 0 {
							// Il y a des consumer groups, réinitialiser le tracking
							inactivityInfo.firstEmptyTime = time.Time{} // Zero time
							inactivityInfo.checked = false
							continue
						}

						// Pas de consumer groups, vérifier depuis combien de temps
						now := time.Now()

						if inactivityInfo.firstEmptyTime.IsZero() {
							// Premier constat d'absence de consumer groups
							inactivityInfo.firstEmptyTime = now
							log.Printf("Queue %s.%s sans consumer groups, début du tracking", domain.Name, queueName)
						} else if now.Sub(inactivityInfo.firstEmptyTime) > 24*time.Hour && !inactivityInfo.checked {
							// Sans consumer groups depuis plus de 24h, nettoyer
							log.Printf("Nettoyage de la queue %s.%s (inactive depuis >24h)", domain.Name, queueName)

							// Obtenir tous les messages et les supprimer
							messages, _ := s.messageRepo.GetMessagesAfterIndex(ctx, domain.Name, queueName, 0, 1000)
							for _, msg := range messages {
								_ = s.messageRepo.DeleteMessage(ctx, domain.Name, queueName, msg.ID)
							}

							// Nettoyer aussi indexToID
							s.messageRepo.ClearQueueIndices(ctx, domain.Name, queueName)

							// Marquer comme vérifiée pour éviter de nettoyer à chaque cycle
							inactivityInfo.checked = true
						}
					}
				}

				inactivityMu.Unlock()
			}
		}
	}()
}

func (s *MessageServiceImpl) Cleanup() {
	log.Println("Cleaning up message service ressource...")
	// géré dans le QueueService
}
