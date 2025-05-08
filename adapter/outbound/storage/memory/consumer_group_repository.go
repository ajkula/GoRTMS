package memory

import (
	"context"
	"sync"
	"time"

	"slices"

	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// ConsumerGroupRepository implémente le repository en mémoire
type ConsumerGroupRepository struct {
	// Map Domain -> Queue -> GroupID -> Offset
	offsets map[string]map[string]map[string]string
	// Map Domain -> Queue -> GroupID -> LastTime
	timestamps map[string]map[string]map[string]time.Time
	// Map Domain -> Queue -> GroupID -> ConsumerIDs
	consumers   map[string]map[string]map[string][]string
	messageRepo outbound.MessageRepository
	mu          sync.RWMutex
}

// NewConsumerGroupRepository crée un nouveau repository
func NewConsumerGroupRepository(messageRepo outbound.MessageRepository) outbound.ConsumerGroupRepository {
	return &ConsumerGroupRepository{
		offsets:     make(map[string]map[string]map[string]string),
		timestamps:  make(map[string]map[string]map[string]time.Time),
		consumers:   make(map[string]map[string]map[string][]string),
		messageRepo: messageRepo,
	}
}

// StoreOffset enregistre un offset pour un groupe
func (r *ConsumerGroupRepository) StoreOffset(
	ctx context.Context,
	domainName, queueName, groupID, messageID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Créer les maps si nécessaire
	if _, exists := r.offsets[domainName]; !exists {
		r.offsets[domainName] = make(map[string]map[string]string)
		r.timestamps[domainName] = make(map[string]map[string]time.Time)
	}
	if _, exists := r.offsets[domainName][queueName]; !exists {
		r.offsets[domainName][queueName] = make(map[string]string)
		r.timestamps[domainName][queueName] = make(map[string]time.Time)
	}

	// Stocker l'offset et le timestamp
	r.offsets[domainName][queueName][groupID] = messageID
	r.timestamps[domainName][queueName][groupID] = time.Now()

	return nil
}

// GetOffset récupère le dernier offset d'un groupe
func (r *ConsumerGroupRepository) GetOffset(
	ctx context.Context,
	domainName, queueName, groupID string,
) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Vérifier si l'offset existe
	if _, exists := r.offsets[domainName]; !exists {
		return "", nil
	}
	if _, exists := r.offsets[domainName][queueName]; !exists {
		return "", nil
	}

	offset, exists := r.offsets[domainName][queueName][groupID]
	if !exists {
		return "", nil
	}

	return offset, nil
}

// RegisterConsumer enregistre un consommateur dans un groupe
func (r *ConsumerGroupRepository) RegisterConsumer(
	ctx context.Context,
	domainName, queueName, groupID, consumerID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Créer les maps si nécessaire
	if _, exists := r.consumers[domainName]; !exists {
		r.consumers[domainName] = make(map[string]map[string][]string)
	}
	if _, exists := r.consumers[domainName][queueName]; !exists {
		r.consumers[domainName][queueName] = make(map[string][]string)
	}

	// Vérifier si le consommateur existe déjà
	consumerList, exists := r.consumers[domainName][queueName][groupID]
	if !exists {
		// Créer une nouvelle liste de consommateurs
		r.consumers[domainName][queueName][groupID] = []string{consumerID}
		return nil
	}

	// Vérifier si le consommateur est déjà enregistré
	if slices.Contains(consumerList, consumerID) {
		return nil // Déjà enregistré
	}

	// Ajouter le consommateur
	r.consumers[domainName][queueName][groupID] = append(consumerList, consumerID)

	// Mettre à jour le timestamp d'activité
	if _, exists := r.timestamps[domainName]; !exists {
		r.timestamps[domainName] = make(map[string]map[string]time.Time)
	}
	if _, exists := r.timestamps[domainName][queueName]; !exists {
		r.timestamps[domainName][queueName] = make(map[string]time.Time)
	}
	r.timestamps[domainName][queueName][groupID] = time.Now()

	// Enregistrer avec la matrice d'acquittement
	matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	matrix.RegisterGroup(groupID)

	return nil
}

// RemoveConsumer supprime un consommateur d'un groupe
func (r *ConsumerGroupRepository) RemoveConsumer(
	ctx context.Context,
	domainName, queueName, groupID, consumerID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Vérifier si le groupe existe
	if _, exists := r.consumers[domainName]; !exists {
		return nil // Rien à faire
	}
	if _, exists := r.consumers[domainName][queueName]; !exists {
		return nil
	}

	consumerList, exists := r.consumers[domainName][queueName][groupID]
	if !exists {
		return nil
	}

	// Rechercher et supprimer le consommateur
	for i, id := range consumerList {
		if id == consumerID {
			// Supprimer le consommateur en préservant l'ordre
			r.consumers[domainName][queueName][groupID] = append(
				consumerList[:i],
				consumerList[i+1:]...,
			)
			break
		}
	}

	// Mettre à jour le timestamp d'activité
	if _, exists := r.timestamps[domainName]; exists {
		if _, exists := r.timestamps[domainName][queueName]; exists {
			r.timestamps[domainName][queueName][groupID] = time.Now()
		}
	}

	// Si c'est le dernier consommateur du groupe, supprimer le groupe de la matrice
	if _, exists := r.consumers[domainName]; exists {
		if queueConsumers, exists := r.consumers[domainName][queueName]; exists {
			if consumerList, exists := queueConsumers[groupID]; exists && len(consumerList) == 0 {
				// Supprimer de la matrice d'acquittement
				matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
				messagesToDelete := matrix.RemoveGroup(groupID)

				// Supprimer les messages entièrement acquittés
				for _, msgID := range messagesToDelete {
					r.messageRepo.DeleteMessage(ctx, domainName, queueName, msgID)
				}
			}
		}
	}

	return nil
}

// ListGroups liste tous les groupes pour une queue
func (r *ConsumerGroupRepository) ListGroups(
	ctx context.Context,
	domainName, queueName string,
) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0)

	// Vérifier les offsets
	if offsets, exists := r.offsets[domainName]; exists {
		if queueOffsets, exists := offsets[queueName]; exists {
			for groupID := range queueOffsets {
				result = append(result, groupID)
			}
			return result, nil
		}
	}

	// Vérifier les consumers si aucun offset
	if consumers, exists := r.consumers[domainName]; exists {
		if queueConsumers, exists := consumers[queueName]; exists {
			for groupID := range queueConsumers {
				// Vérifier si ce groupe n'est pas déjà dans le résultat
				found := false
				for _, id := range result {
					if id == groupID {
						found = true
						break
					}
				}
				if !found {
					result = append(result, groupID)
				}
			}
		}
	}

	return result, nil
}

// CleanupStaleGroups nettoie les groupes inactifs
func (r *ConsumerGroupRepository) CleanupStaleGroups(
	ctx context.Context,
	olderThan time.Duration,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	threshold := now.Add(-olderThan)

	// Parcourir les timestamps pour trouver les groupes inactifs
	for domainName, domainTimestamps := range r.timestamps {
		for queueName, queueTimestamps := range domainTimestamps {
			for groupID, lastActive := range queueTimestamps {
				if lastActive.Before(threshold) {
					// Supprimer le groupe des offsets
					if _, exists := r.offsets[domainName]; exists {
						if _, exists := r.offsets[domainName][queueName]; exists {
							delete(r.offsets[domainName][queueName], groupID)
						}
					}

					// Supprimer le groupe des consumers
					if _, exists := r.consumers[domainName]; exists {
						if _, exists := r.consumers[domainName][queueName]; exists {
							delete(r.consumers[domainName][queueName], groupID)
						}
					}

					// Supprimer le timestamp
					delete(queueTimestamps, groupID)
				}
			}

			// Nettoyer les maps vides
			if len(queueTimestamps) == 0 {
				delete(domainTimestamps, queueName)
			}
		}

		if len(domainTimestamps) == 0 {
			delete(r.timestamps, domainName)
		}
	}

	return nil
}
