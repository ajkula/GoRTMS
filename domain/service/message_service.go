package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
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

// ConsumeMessage consomme un message d'une file d'attente
// Simplifier ConsumeMessage pour utiliser la même logique que ConsumeMessageWithGroup
func (s *MessageServiceImpl) ConsumeMessage(
	domainName, queueName string,
) (*model.Message, error) {
	// Utiliser un ID de groupe temporaire pour compatibilité
	tempGroupID := "temp-" + time.Now().Format("20060102-150405.999999999")
	options := &inbound.ConsumeOptions{
		ResetOffset: true, // Toujours lire depuis le début
	}

	return s.ConsumeMessageWithGroup(s.rootCtx, domainName, queueName, tempGroupID, options)
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

	// Obtenir le dernier offset
	lastOffset, _ := s.consumerGroupRepo.GetOffset(ctx, domainName, queueName, groupID)

	// Enregistrer le consumer group dans la channel queue si nécessaire
	chQueue.AddConsumerGroup(groupID, lastOffset)

	// Enregistrer le consommateur dans le repository
	if options != nil && options.ConsumerID != "" {
		_ = s.consumerGroupRepo.RegisterConsumer(ctx, domainName, queueName, groupID, options.ConsumerID)
	}

	// Toujours demander 5 messages par défaut
	channelQueue.RequestMessages(groupID, 5)

	// Définir le timeout
	timeout := 1 * time.Second
	if options != nil && options.Timeout > 0 {
		timeout = options.Timeout
	}

	// Consommer un message
	message, err := chQueue.ConsumeMessage(groupID, timeout)
	if err != nil {
		return nil, err
	}

	// Si aucun message n'est trouvé, essayer directement depuis le repository
	if message == nil {
		messages, err := s.messageRepo.GetMessagesAfterID(ctx, domainName, queueName, lastOffset, 1)
		if err != nil || len(messages) == 0 {
			return nil, err
		}
		message = messages[0]

		// Alimenter le canal du groupe avec d'autres messages pour les prochaines demandes
		if len(messages) > 1 {
			_ = chQueue.RequestMessages(groupID, 10) // Demander plus de messages en arrière-plan
		}
	}

	// Si un message a été trouvé, l'acquitter
	if message != nil {
		// Mettre à jour l'offset
		_ = s.consumerGroupRepo.StoreOffset(ctx, domainName, queueName, groupID, message.ID)

		// Acquitter automatiquement
		fullyAcked, _ := s.messageRepo.AcknowledgeMessage(ctx, domainName, queueName, groupID, message.ID)

		// Si complètement acquitté, supprimer
		if fullyAcked {
			_ = s.messageRepo.DeleteMessage(ctx, domainName, queueName, message.ID)
		}

		// Mettre à jour les statistiques
		if s.statsService != nil {
			s.statsService.TrackMessageConsumed(domainName, queueName)
		}
	}

	return message, nil
}

func (s *MessageServiceImpl) GetMessagesAfterID(
	ctx context.Context,
	domainName, queueName, startMessageID string,
	limit int,
) ([]*model.Message, error) {
	return s.messageRepo.GetMessagesAfterID(ctx, domainName, queueName, startMessageID, limit)
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
	// Nettoyer les messages orphelins périodiquement
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Parcourir les domaines et queues
				domains, err := s.domainRepo.ListDomains(ctx)
				if err != nil {
					continue
				}

				for _, domain := range domains {
					for queueName := range domain.Queues {
						// Obtenir la matrice d'acquittement
						matrix := s.messageRepo.GetOrCreateAckMatrix(domain.Name, queueName)

						// Si aucun consumer group actif, supprimer tous les messages
						// ou si la matrice a des messages sans groupes, les nettoyer
						// (Ceci devrait être implémenté dans AckMatrix)
						if matrix.GetActiveGroupCount() == 0 {
							// Obtenir tous les messages et les supprimer
							messages, _ := s.messageRepo.GetMessages(ctx, domain.Name, queueName, 1000)
							for _, msg := range messages {
								_ = s.messageRepo.DeleteMessage(ctx, domain.Name, queueName, msg.ID)
							}
						}
					}
				}
			}
		}
	}()
}

func (s *MessageServiceImpl) Cleanup() {
	log.Println("Cleaning up message service ressource...")
	// géré dans le QueueService
}
