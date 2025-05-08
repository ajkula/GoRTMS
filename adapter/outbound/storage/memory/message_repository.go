package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

	// Map des matrices d'acquittement par queue
	ackMatrices map[string]*model.AckMatrix
	ackMu       sync.RWMutex
}

// NewMessageRepository crée un nouveau repository de messages en mémoire
func NewMessageRepository() outbound.MessageRepository {
	return &MessageRepository{
		messages:    make(map[string]map[string]map[string]*model.Message),
		ackMatrices: make(map[string]*model.AckMatrix),
	}
}

func (r *MessageRepository) GetOrCreateAckMatrix(domainName, queueName string) *model.AckMatrix {
	r.ackMu.Lock()
	defer r.ackMu.Unlock()

	key := fmt.Sprintf("%s:%s", domainName, queueName)
	matrix, exists := r.ackMatrices[key]
	if !exists {
		matrix = model.NewAckMatrix()
		r.ackMatrices[key] = matrix
	}

	return matrix
}

func (r *MessageRepository) AcknowledgeMessage(
	ctx context.Context,
	domainName, queueName, groupID, messageID string,
) (bool, error) {
	matrix := r.GetOrCreateAckMatrix(domainName, queueName)
	return matrix.Acknowledge(messageID, groupID), nil
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

// GetMessagesAfterID récupère les messages après un ID spécifique
func (r *MessageRepository) GetMessagesAfterID(
	ctx context.Context,
	domainName, queueName, startMessageID string,
	limit int,
) ([]*model.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Vérifier si le domaine et la queue existent
	if _, exists := r.messages[domainName]; !exists {
		return []*model.Message{}, nil
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		return []*model.Message{}, nil
	}

	queue := r.messages[domainName][queueName]

	// Récupérer tous les messages et les trier par timestamp
	allMessages := make([]*model.Message, 0, len(queue))
	for _, msg := range queue {
		allMessages = append(allMessages, msg)
	}

	// Trier par timestamp (plus ancien en premier)
	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].Timestamp.Before(allMessages[j].Timestamp)
	})

	// Si pas d'ID de départ, retourner les premiers messages
	if startMessageID == "" {
		if len(allMessages) <= limit {
			return allMessages, nil
		}
		return allMessages[:limit], nil
	}

	// Trouver la position du message de départ
	startIndex := -1
	for i, msg := range allMessages {
		if msg.ID == startMessageID {
			startIndex = i
			break
		}
	}

	// Si message de départ non trouvé, retourner vide
	if startIndex == -1 {
		return []*model.Message{}, nil
	}

	// Retourner les messages qui suivent
	startIndex++ // Commencer après le message de départ

	if startIndex >= len(allMessages) {
		return []*model.Message{}, nil
	}

	endIndex := startIndex + limit
	if endIndex > len(allMessages) {
		endIndex = len(allMessages)
	}

	return allMessages[startIndex:endIndex], nil
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
