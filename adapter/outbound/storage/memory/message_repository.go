package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"slices"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrMessageNotFound = errors.New("message not found")
	ErrQueueNotFound   = errors.New("queue not found")
)

type MessageRepository struct {
	// Map of domains -> queues -> messages
	messages         map[string]map[string]map[string]*model.Message
	indexToID        map[string]map[string]map[int64]string
	nextIndexCounter map[string]map[string]int64
	mu               sync.RWMutex

	// Map of acknowledgment matrices per queue
	ackMatrices map[string]*model.AckMatrix
	ackMu       sync.RWMutex

	logger outbound.Logger
}

func NewMessageRepository(logger outbound.Logger) outbound.MessageRepository {
	return &MessageRepository{
		messages:         make(map[string]map[string]map[string]*model.Message),
		indexToID:        make(map[string]map[string]map[int64]string),
		nextIndexCounter: make(map[string]map[string]int64),
		ackMatrices:      make(map[string]*model.AckMatrix),
		logger:           logger,
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

func (r *MessageRepository) StoreMessage(
	ctx context.Context,
	domainName, queueName string,
	message *model.Message,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create maps if necessary
	if _, exists := r.messages[domainName]; !exists {
		r.messages[domainName] = make(map[string]map[string]*model.Message)
		r.indexToID[domainName] = make(map[string]map[int64]string)
		r.nextIndexCounter[domainName] = make(map[string]int64)
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		r.messages[domainName][queueName] = make(map[string]*model.Message)
		r.indexToID[domainName][queueName] = make(map[int64]string)
		r.nextIndexCounter[domainName][queueName] = 0
	}

	// Use and increment the atomic counter
	nextIndex := r.nextIndexCounter[domainName][queueName]
	r.nextIndexCounter[domainName][queueName]++

	// Store the message
	r.messages[domainName][queueName][message.ID] = message

	// Associate the index with the message ID
	r.indexToID[domainName][queueName][nextIndex] = message.ID

	return nil
}

func (r *MessageRepository) GetMessage(
	ctx context.Context,
	domainName, queueName, messageID string,
) (*model.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.messages[domainName]; !exists {
		return nil, ErrQueueNotFound
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		return nil, ErrQueueNotFound
	}

	message, exists := r.messages[domainName][queueName][messageID]
	if !exists {
		return nil, ErrMessageNotFound
	}

	return message, nil
}

func (r *MessageRepository) GetMessagesAfterIndex(
	ctx context.Context,
	domainName, queueName string,
	startIndex int64,
	limit int,
) ([]*model.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.indexToID[domainName]; !exists {
		return []*model.Message{}, nil
	}
	if _, exists := r.indexToID[domainName][queueName]; !exists {
		return []*model.Message{}, nil
	}

	var indexes []int64
	for idx := range r.indexToID[domainName][queueName] {
		if idx >= startIndex {
			indexes = append(indexes, idx)
		}
	}
	slices.Sort(indexes)

	// Retrieve the corresponding messages, ignoring those that were deleted
	messages := make([]*model.Message, 0, limit)
	obsoleteIndexes := []int64{} // To keep track of indexes to delete

	for _, idx := range indexes {
		messageID := r.indexToID[domainName][queueName][idx]

		// Check that the message still exists
		if message, exists := r.messages[domainName][queueName][messageID]; exists {
			messages = append(messages, message)
			if len(messages) >= limit {
				break
			}
		} else {
			// Mark for deletion (do not modify the map during iteration)
			obsoleteIndexes = append(obsoleteIndexes, idx)
		}
	}

	// Delete obsolete indexes after the iteration
	for _, idx := range obsoleteIndexes {
		r.logger.Debug("Suppression de l'index obsolète", "index", idx)
		delete(r.indexToID[domainName][queueName], idx)
	}

	return messages, nil
}

func (r *MessageRepository) GetIndexByMessageID(
	ctx context.Context,
	domainName, queueName, messageID string,
) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.indexToID[domainName]; !exists {
		return 0, ErrQueueNotFound
	}
	if _, exists := r.indexToID[domainName][queueName]; !exists {
		return 0, ErrQueueNotFound
	}

	// Iterate over the indexes to find the one pointing to our messageID
	for index, id := range r.indexToID[domainName][queueName] {
		if id == messageID {
			return index, nil
		}
	}

	return 0, ErrMessageNotFound
}

func (r *MessageRepository) DeleteMessage(
	ctx context.Context,
	domainName, queueName, messageID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if the domain and queue exist
	if _, exists := r.messages[domainName]; !exists {
		return ErrQueueNotFound
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		return ErrQueueNotFound
	}

	// delete message
	if _, exists := r.messages[domainName][queueName][messageID]; !exists {
		return ErrMessageNotFound
	}

	delete(r.messages[domainName][queueName], messageID)
	return nil
}

func (r *MessageRepository) GetQueueMessageCount(domainName string, queueName string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// check if domain and queue exist
	if _, exists := r.messages[domainName]; !exists {
		return 0
	}
	if queueMessages, exists := r.messages[domainName][queueName]; exists {
		return len(queueMessages)
	}
	return 0
}

func (r *MessageRepository) ClearQueueIndices(
	ctx context.Context,
	domainName, queueName string,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// check if maps exist
	if _, exists := r.indexToID[domainName]; !exists {
		return
	}
	if _, exists := r.indexToID[domainName][queueName]; !exists {
		return
	}

	// Delete all indexToID entries for this queue
	if domainIndices, exists := r.indexToID[domainName]; exists {
		// Reset the map for this queue
		domainIndices[queueName] = make(map[int64]string)
		r.logger.Debug("Indices réinitialisés",
			"domain", domainName,
			"queue", queueName)
	}
}

func (r *MessageRepository) CleanupMessageIndices(
	ctx context.Context,
	domainName, queueName string,
	minPosition int64,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// check if maps exist
	if _, exists := r.indexToID[domainName]; !exists {
		return
	}

	indexMap, exists := r.indexToID[domainName][queueName]
	if !exists {
		return
	}

	initialSize := len(indexMap)

	// Delete all indexes lower than minPosition
	for idx := range indexMap {
		if idx < minPosition {
			delete(indexMap, idx)
		}
	}

	removedCount := initialSize - len(indexMap)
	if removedCount > 0 {
		r.logger.Debug("Nettoyage incrémental des indices",
			"domain", domainName,
			"queue", queueName,
			"removedCount", removedCount,
			"minPosition", minPosition,
			"remaining", len(indexMap))
	}
}
