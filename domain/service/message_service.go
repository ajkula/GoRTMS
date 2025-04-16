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

// MessageServiceImpl implémente le service des messages
type MessageServiceImpl struct {
	domainRepo      outbound.DomainRepository
	messageRepo     outbound.MessageRepository
	subscriptionReg outbound.SubscriptionRegistry
	queueService    inbound.QueueService
	statsService    inbound.StatsService
}

// NewMessageService crée un nouveau service de messages
func NewMessageService(
	domainRepo outbound.DomainRepository,
	messageRepo outbound.MessageRepository,
	subscriptionReg outbound.SubscriptionRegistry,
	queueService inbound.QueueService,
	statsService ...inbound.StatsService,
) inbound.MessageService {
	impl := &MessageServiceImpl{
		domainRepo:      domainRepo,
		messageRepo:     messageRepo,
		subscriptionReg: subscriptionReg,
		queueService:    queueService,
	}

	if len(statsService) > 0 {
		impl.statsService = statsService[0]
	}

	return impl
}

// PublishMessage publie un message dans une file d'attente
func (s *MessageServiceImpl) PublishMessage(
	ctx context.Context,
	domainName, queueName string,
	message *model.Message,
) error {
	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	// Vérifier si la file d'attente existe
	channelQueue, err := s.queueService.GetChannelQueue(ctx, domainName, queueName)
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
	if err := s.messageRepo.StoreMessage(ctx, domainName, queueName, message); err != nil {
		return err
	}

	// Collecter des statistiques
	if s.statsService != nil {
		s.statsService.TrackMessagePublished(domainName, queueName)
	}

	// Enqueue le message dans la chan queue
	if err := channelQueue.Enqueue(ctx, message); err != nil {
		return err
	}

	// Notifier les abonnés via le registry existant
	if err := s.subscriptionReg.NotifySubscribers(ctx, domainName, queueName, message); err != nil {
		return err
	}

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
				if err := s.PublishMessage(ctx, domainName, destQueue, &destMsg); err != nil {
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
func (s *MessageServiceImpl) ConsumeMessage(
	ctx context.Context,
	domainName, queueName string,
) (*model.Message, error) {
	// Récupérer la channelQueue
	channelQueue, err := s.queueService.GetChannelQueue(ctx, domainName, queueName)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	// Tenter de consommer un message via la channelQueue
	message, err := channelQueue.Dequeue(ctx)
	if err != nil {
		return nil, err
	}

	// Si aucun message n'est dispo dans la channelQueue, essayer le repo
	if message == nil {
		messages, err := s.messageRepo.GetMessages(ctx, domainName, queueName, 1)
		if err != nil {
			return nil, err
		}

		if len(messages) == 0 {
			return nil, nil // rien
		}

		message = messages[0]

		// mode non persistent supprime le message du repo
		queue := channelQueue.GetQueue()
		if !queue.Config.IsPersistent || queue.Config.DeliveryMode == model.SingleConsumerMode {
			if err := s.messageRepo.DeleteMessage(ctx, domainName, queueName, message.ID); err != nil {
				return nil, err
			}
		}
	}

	// Collecter les stats
	if s.statsService != nil {
		s.statsService.TrackMessageConsumed(domainName, queueName)
	}

	return message, nil
}

// SubscribeToQueue s'abonne à une file d'attente
func (s *MessageServiceImpl) SubscribeToQueue(
	ctx context.Context,
	domainName, queueName string,
	handler model.MessageHandler,
) (string, error) {
	// Récupérer le domaine
	domain, err := s.domainRepo.GetDomain(ctx, domainName)
	if err != nil {
		return "", ErrDomainNotFound
	}

	// Vérifier si la file d'attente existe
	if _, exists := domain.Queues[queueName]; !exists {
		return "", ErrQueueNotFound
	}

	// Enregistrer l'abonnement
	subscriptionID, err := s.subscriptionReg.RegisterSubscription(ctx, domainName, queueName, handler)
	if err != nil {
		return "", ErrSubscriptionFailed
	}

	return subscriptionID, nil
}

// UnsubscribeFromQueue se désinscrit d'une file d'attente
func (s *MessageServiceImpl) UnsubscribeFromQueue(
	ctx context.Context,
	domainName, queueName string,
	subscriptionID string,
) error {
	// Supprimer l'abonnement
	return s.subscriptionReg.UnregisterSubscription(ctx, subscriptionID)
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

func (s *MessageServiceImpl) Cleanup() {
	log.Println("Cleaning up message service ressource...")
	// géré dans le QueueService
}
