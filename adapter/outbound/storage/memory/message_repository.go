package memory

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"slices"

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
	messages  map[string]map[string]map[string]*model.Message
	indexToID map[string]map[string]map[int64]string
	mu        sync.RWMutex

	// Map des matrices d'acquittement par queue
	ackMatrices map[string]*model.AckMatrix
	ackMu       sync.RWMutex
}

// NewMessageRepository crée un nouveau repository de messages en mémoire
func NewMessageRepository() outbound.MessageRepository {
	return &MessageRepository{
		messages:    make(map[string]map[string]map[string]*model.Message),
		indexToID:   make(map[string]map[string]map[int64]string),
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

// StoreMessage stocke un message et lui attribue un index séquentiel
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
		r.indexToID[domainName] = make(map[string]map[int64]string)
	}
	if _, exists := r.messages[domainName][queueName]; !exists {
		r.messages[domainName][queueName] = make(map[string]*model.Message)
		r.indexToID[domainName][queueName] = make(map[int64]string)
	}

	// Déterminer le prochain index
	nextIndex := int64(0)
	if _, exists := r.indexToID[domainName][queueName]; exists {
		for idx := range r.indexToID[domainName][queueName] {
			if idx >= nextIndex {
				nextIndex = idx + 1
			}
		}
	}

	// Stocker le message
	r.messages[domainName][queueName][message.ID] = message

	// Associer l'index au message ID
	r.indexToID[domainName][queueName][nextIndex] = message.ID

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

// GetMessagesAfterIndex récupère les messages à partir d'un index donné
// Remplace les anciennes méthodes GetMessages et GetMessagesAfterID:
// - Pour l'équivalent de GetMessages, utiliser startIndex=0
// - Pour l'équivalent de GetMessagesAfterID, convertir l'ID en index d'abord
func (r *MessageRepository) GetMessagesAfterIndex(
	ctx context.Context,
	domainName, queueName string,
	startIndex int64,
	limit int,
) ([]*model.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	log.Printf("[TRACE] GetMessagesAfterIndex appelé avec startIndex=%d",
		startIndex)

	// Vérifier si le domaine et la queue existent
	if _, exists := r.indexToID[domainName]; !exists {
		return []*model.Message{}, nil
	}
	if _, exists := r.indexToID[domainName][queueName]; !exists {
		return []*model.Message{}, nil
	}

	// AJOUT: Debug pour voir tous les index disponibles
	log.Printf("Index disponibles pour %s.%s: %v", domainName, queueName, r.indexToID[domainName][queueName])

	// Collecter tous les index disponibles et les trier
	var indexes []int64
	for idx := range r.indexToID[domainName][queueName] {
		if idx >= startIndex {
			indexes = append(indexes, idx)
		}
	}
	slices.Sort(indexes)

	// AJOUT: Debug pour voir les index qui seront utilisés
	log.Printf("Utilisation des index: %v (startIndex: %d)", indexes, startIndex)

	// Récupérer les messages correspondants, en ignorant ceux qui ont été supprimés
	messages := make([]*model.Message, 0, limit)
	obsoleteIndexes := []int64{} // Pour enregistrer les index à supprimer

	for _, idx := range indexes {
		messageID := r.indexToID[domainName][queueName][idx]

		// AJOUT: Debug pour comprendre le processus
		log.Printf("Vérification de l'index %d avec messageID %s", idx, messageID)

		// Vérifier que le message existe toujours
		if message, exists := r.messages[domainName][queueName][messageID]; exists {
			messages = append(messages, message)
			log.Printf("Message trouvé et ajouté, total: %d/%d", len(messages), limit)
			if len(messages) >= limit {
				break // Nous avons assez de messages
			}
		} else {
			// Marquer pour suppression (ne pas modifier la map pendant l'itération)
			obsoleteIndexes = append(obsoleteIndexes, idx)
			log.Printf("Message non trouvé, marqué pour suppression: index %d", idx)
		}
	}

	// Supprimer les index obsolètes après l'itération
	for _, idx := range obsoleteIndexes {
		log.Printf("Suppression de l'index obsolète %d", idx)
		delete(r.indexToID[domainName][queueName], idx)
	}

	return messages, nil
}

// Cette méthode parcourt la map indexToID pour trouver quel index correspond à un ID
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

	// Parcourir les index pour trouver celui qui pointe vers notre messageID
	for index, id := range r.indexToID[domainName][queueName] {
		if id == messageID {
			return index, nil
		}
	}

	return 0, ErrMessageNotFound
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

// ClearQueueIndices nettoie toutes les références d'index pour une queue spécifique
func (r *MessageRepository) ClearQueueIndices(
	ctx context.Context,
	domainName, queueName string,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Vérifier que les maps existent
	if _, exists := r.indexToID[domainName]; !exists {
		return
	}
	if _, exists := r.indexToID[domainName][queueName]; !exists {
		return
	}

	// Supprimer toutes les entrées de indexToID pour cette queue
	if domainIndices, exists := r.indexToID[domainName]; exists {
		// Réinitialiser la map pour cette queue
		domainIndices[queueName] = make(map[int64]string)
		log.Printf("Indices réinitialisés pour %s.%s", domainName, queueName)
	}
}

// CleanupMessageIndices supprime les entrées d'index jusqu'à une position minimum
func (r *MessageRepository) CleanupMessageIndices(
	ctx context.Context,
	domainName, queueName string,
	minPosition int64,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Vérifier que les maps existent
	if _, exists := r.indexToID[domainName]; !exists {
		return
	}

	indexMap, exists := r.indexToID[domainName][queueName]
	if !exists {
		return
	}

	initialSize := len(indexMap)

	// Supprimer tous les index inférieurs à minPosition
	for idx := range indexMap {
		if idx < minPosition {
			delete(indexMap, idx)
		}
	}

	removedCount := initialSize - len(indexMap)
	if removedCount > 0 {
		log.Printf("[DEBUG] Nettoyage incrémental des indices pour %s.%s: %d indices supprimés (< %d), %d restants",
			domainName, queueName, removedCount, minPosition, len(indexMap))
	}
}
