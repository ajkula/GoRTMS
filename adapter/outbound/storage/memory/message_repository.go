package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrMessageNotFound = errors.New("message not found")
	ErrQueueNotFound   = errors.New("queue not found")
)

// MessageRepository implémente un repository de messages en mémoire
type MessageRepository struct {
	// Map de domaines -> files d'attente -> messages
	messages map[string]map[string]map[string]*model.Message
	mu       sync.RWMutex
}

// NewMessageRepository crée un nouveau repository de messages en mémoire
func NewMessageRepository() outbound.MessageRepository {
	return &MessageRepository{
		messages: make(map[string]map[string]map[string]*model.Message),
	}
}

// StoreMessage stocke un message
func (r *MessageRepository) StoreMessage(
	ctx context.Context,
	domainName, queueName string,
	message *model.Message,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Créer les maps si nécessaire
	if _, exists := r.messages[domainName]; !exists {
		r.messages[domainName] = make(map[string]map[string]*model.Message)
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		r.messages[domainName][queueName] = make(map[string]*model.Message)
	}

	// Stocker le message
	r.messages[domainName][queueName][message.ID] = message

	// Démarrer une goroutine pour nettoyer les messages expirés si TTL > 0
	if ttl, ok := message.Metadata["ttl"].(int64); ok && ttl > 0 {
		go func(msgID string) {
			time.Sleep(time.Duration(ttl) * time.Millisecond)
			r.mu.Lock()
			defer r.mu.Unlock()

			if queues, exists := r.messages[domainName]; exists {
				if queue, exists := queues[queueName]; exists {
					delete(queue, msgID)
				}
			}
		}(message.ID)
	}

	return nil
}

// GetMessage récupère un message par son ID
func (r *MessageRepository) GetMessage(
	ctx context.Context,
	domainName, queueName, messageID string,
) (*model.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Vérifier si le domaine et la file d'attente existent
	if _, exists := r.messages[domainName]; !exists {
		return nil, ErrQueueNotFound
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		return nil, ErrQueueNotFound
	}

	// Récupérer le message
	message, exists := r.messages[domainName][queueName][messageID]
	if !exists {
		return nil, ErrMessageNotFound
	}

	return message, nil
}

// GetMessages récupère plusieurs messages
func (r *MessageRepository) GetMessages(
	ctx context.Context,
	domainName, queueName string,
	limit int,
) ([]*model.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Vérifier si le domaine et la file d'attente existent
	if _, exists := r.messages[domainName]; !exists {
		return []*model.Message{}, nil
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		return []*model.Message{}, nil
	}

	// Récupérer les messages
	queue := r.messages[domainName][queueName]
	messages := make([]*model.Message, 0, limit)

	// Trier par horodatage (plus ancien en premier)
	var oldestTime time.Time
	var oldestID string

	for i := 0; i < limit; i++ {
		oldestTime = time.Now().Add(1 * time.Hour) // Futur
		oldestID = ""

		for id, msg := range queue {
			if oldestID == "" || msg.Timestamp.Before(oldestTime) {
				oldestTime = msg.Timestamp
				oldestID = id
			}
		}

		if oldestID == "" {
			break // Plus de messages
		}

		messages = append(messages, queue[oldestID])
		delete(queue, oldestID) // Supprimer le message de la map (FIFO)
	}

	return messages, nil
}

// DeleteMessage supprime un message
func (r *MessageRepository) DeleteMessage(
	ctx context.Context,
	domainName, queueName, messageID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Vérifier si le domaine et la file d'attente existent
	if _, exists := r.messages[domainName]; !exists {
		return ErrQueueNotFound
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		return ErrQueueNotFound
	}

	// Supprimer le message
	if _, exists := r.messages[domainName][queueName][messageID]; !exists {
		return ErrMessageNotFound
	}

	delete(r.messages[domainName][queueName], messageID)
	return nil
}
