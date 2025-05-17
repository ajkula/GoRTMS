package memory

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"slices"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

// ConsumerGroupRepository implémente le repository en mémoire
type ConsumerGroupRepository struct {
	// Map Domain -> Queue -> GroupID -> Offset
	positions map[string]map[string]map[string]int64
	// Map Domain -> Queue -> GroupID -> LastTime
	timestamps map[string]map[string]map[string]time.Time
	// ttls stocke les TTL pour chaque groupe
	ttls map[string]map[string]map[string]time.Duration
	// Map Domain -> Queue -> GroupID -> ConsumerIDs
	consumers   map[string]map[string]map[string][]string
	messageRepo outbound.MessageRepository
	mu          sync.RWMutex
}

// NewConsumerGroupRepository crée un nouveau repository
func NewConsumerGroupRepository(messageRepo outbound.MessageRepository) outbound.ConsumerGroupRepository {
	return &ConsumerGroupRepository{
		positions:   make(map[string]map[string]map[string]int64),
		timestamps:  make(map[string]map[string]map[string]time.Time),
		consumers:   make(map[string]map[string]map[string][]string),
		ttls:        make(map[string]map[string]map[string]time.Duration),
		messageRepo: messageRepo,
	}
}

// StorePosition enregistre un offset pour un groupe
func (r *ConsumerGroupRepository) StorePosition(
	ctx context.Context,
	domainName, queueName, groupID string, position int64,
) error {
	if domainName == "" || queueName == "" || groupID == "" {
		return errors.New("all parameters must be non-empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Protection supplémentaire contre nil maps
	if r.positions == nil {
		log.Printf("WARNING: r.positions was nil in StorePosition - this should never happen!")
		r.positions = make(map[string]map[string]map[string]int64)
	}
	if r.timestamps == nil {
		log.Printf("WARNING: r.timestamps was nil in StorePosition - this should never happen!")
		r.timestamps = make(map[string]map[string]map[string]time.Time)
	}

	// Créer les maps si nécessaire - code existant
	if _, exists := r.positions[domainName]; !exists {
		r.positions[domainName] = make(map[string]map[string]int64)
	}
	if _, exists := r.positions[domainName][queueName]; !exists {
		r.positions[domainName][queueName] = make(map[string]int64)
	}

	if _, exists := r.timestamps[domainName]; !exists {
		r.timestamps[domainName] = make(map[string]map[string]time.Time)
	}
	if _, exists := r.timestamps[domainName][queueName]; !exists {
		r.timestamps[domainName][queueName] = make(map[string]time.Time)
	}

	// Ajouter cette vérification:
	currentPosition, exists := r.positions[domainName][queueName][groupID]
	if exists && position <= currentPosition {
		log.Printf("[WARN] Ignoring position regression for group=%s: current=%d, attempted=%d",
			groupID, currentPosition, position)
		return nil
	}

	// Stocker l'offset et le timestamp
	r.positions[domainName][queueName][groupID] = position
	r.timestamps[domainName][queueName][groupID] = time.Now()
	log.Printf("[DEBUG] Storing position for group=%s: value=%d", groupID, position)

	return nil
}

// GetPosition récupère le dernier offset d'un groupe
func (r *ConsumerGroupRepository) GetPosition(
	ctx context.Context,
	domainName, queueName, groupID string,
) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Vérifier si l'offset existe
	if _, exists := r.positions[domainName]; !exists {
		log.Printf("Domain not found in offsets: %v", domainName)
		return 0, nil
	}
	if _, exists := r.positions[domainName][queueName]; !exists {
		log.Printf("Queue not found in domain: %v", queueName)
		return 0, nil
	}

	offset, exists := r.positions[domainName][queueName][groupID]
	if !exists {
		log.Printf("Group not found in queue: %v  [%v]", groupID, offset)
		return 0, nil
	}
	log.Printf("[DEBUG] Getting offset for group=%s: returning %d", groupID, offset)

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
		r.consumers[domainName][queueName][groupID] = []string{}
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
	if offsets, exists := r.positions[domainName]; exists {
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
	defaultInactivityPeriod time.Duration,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	groupsRemoved := 0

	// Parcourir les timestamps pour trouver les groupes inactifs
	for domainName, domainTimestamps := range r.timestamps {
		for queueName, queueTimestamps := range domainTimestamps {
			for groupID, lastActive := range queueTimestamps {
				// Déterminer la période d'inactivité pour ce groupe
				inactivityPeriod := defaultInactivityPeriod

				// Si le groupe a un TTL configuré, l'utiliser à la place
				if ttl, err := r.GetTTL(ctx, domainName, queueName, groupID); err == nil && ttl > 0 {
					inactivityPeriod = ttl
				}

				// Calculer le seuil avec la période appropriée
				threshold := now.Add(-inactivityPeriod)

				if lastActive.Before(threshold) {
					log.Printf("Nettoyage du consumer group %s.%s.%s (inactif depuis %v)",
						domainName, queueName, groupID, now.Sub(lastActive))

					// Supprimer le groupe des offsets
					if _, exists := r.positions[domainName]; exists {
						if _, exists := r.positions[domainName][queueName]; exists {
							delete(r.positions[domainName][queueName], groupID)
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

					// Supprimer de la matrice d'acquittement et traiter les messages
					matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
					messagesToDelete := matrix.RemoveGroup(groupID)

					// Supprimer les messages entièrement acquittés
					for _, msgID := range messagesToDelete {
						r.messageRepo.DeleteMessage(ctx, domainName, queueName, msgID)
					}

					groupsRemoved++
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

	if groupsRemoved > 0 {
		log.Printf("Nettoyage terminé: %d consumer groups supprimés", groupsRemoved)
	}

	return nil
}

// GetGroupDetails récupère les détails d'un groupe spécifique
func (r *ConsumerGroupRepository) GetGroupDetails(
	ctx context.Context,
	domainName, queueName, groupID string,
) (*model.ConsumerGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Vérifier si le groupe existe
	if _, exists := r.consumers[domainName]; !exists {
		return nil, errors.New("consumer group not found")
	}
	if _, exists := r.consumers[domainName][queueName]; !exists {
		return nil, errors.New("consumer group not found")
	}
	consumerList, exists := r.consumers[domainName][queueName][groupID]
	if !exists {
		return nil, errors.New("consumer group not found")
	}

	// Récupérer l'offset
	var lastPosition int64
	if _, exists := r.positions[domainName]; exists {
		if _, exists := r.positions[domainName][queueName]; exists {
			lastPosition = r.positions[domainName][queueName][groupID]
		}
	}

	// Récupérer le timestamp d'activité
	var lastActivity time.Time
	if _, exists := r.timestamps[domainName]; exists {
		if _, exists := r.timestamps[domainName][queueName]; exists {
			lastActivity = r.timestamps[domainName][queueName][groupID]
		}
	}

	// Créer l'objet ConsumerGroup
	group := &model.ConsumerGroup{
		DomainName:   domainName,
		QueueName:    queueName,
		GroupID:      groupID,
		Position:     lastPosition,
		ConsumerIDs:  make([]string, len(consumerList)),
		LastActivity: lastActivity,
		// Les autres champs ne sont pas disponibles dans l'implémentation actuelle
	}

	// Copier la liste des consumers
	copy(group.ConsumerIDs, consumerList)

	// Récupérer le TTL si disponible
	ttl, err := r.GetTTL(ctx, domainName, queueName, groupID)
	if err == nil {
		group.TTL = ttl
	}

	// Calculer le nombre de messages en attente
	matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	group.MessageCount = matrix.GetPendingMessageCount(groupID)

	return group, nil
}

// StoreTTL enregistre le TTL pour un groupe
func (r *ConsumerGroupRepository) StoreTTL(
	ctx context.Context,
	domainName, queueName, groupID string,
	ttl time.Duration,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialiser les maps si nécessaire
	if _, exists := r.ttls[domainName]; !exists {
		r.ttls = make(map[string]map[string]map[string]time.Duration)
		r.ttls[domainName] = make(map[string]map[string]time.Duration)
	}
	if _, exists := r.ttls[domainName][queueName]; !exists {
		r.ttls[domainName][queueName] = make(map[string]time.Duration)
	}

	// Stocker le TTL
	r.ttls[domainName][queueName][groupID] = ttl
	return nil
}

// GetTTL récupère le TTL d'un groupe
func (r *ConsumerGroupRepository) GetTTL(
	ctx context.Context,
	domainName, queueName, groupID string,
) (time.Duration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Vérifier si le TTL est défini
	if _, exists := r.ttls[domainName]; !exists {
		return 0, errors.New("TTL not found")
	}
	if _, exists := r.ttls[domainName][queueName]; !exists {
		return 0, errors.New("TTL not found")
	}
	ttl, exists := r.ttls[domainName][queueName][groupID]
	if !exists {
		return 0, errors.New("TTL not found")
	}

	return ttl, nil
}

// GetAllGroups récupère tous les consumer groups de tous les domaines
func (r *ConsumerGroupRepository) GetAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allGroups []*model.ConsumerGroup

	// Parcourir toutes les entrées
	for domainName, domainConsumers := range r.consumers {
		for queueName, queueConsumers := range domainConsumers {
			for groupID := range queueConsumers {
				// Libérer le mutex pour appeler GetGroupDetails
				r.mu.RUnlock()
				group, err := r.GetGroupDetails(ctx, domainName, queueName, groupID)
				r.mu.RLock() // Reprendre le mutex

				if err != nil {
					log.Printf("Error getting details for group %s: %v", groupID, err)
					continue
				}

				allGroups = append(allGroups, group)
			}
		}
	}

	return allGroups, nil
}

// UpdateLastActivity met à jour le timestamp d'activité d'un groupe
func (r *ConsumerGroupRepository) UpdateLastActivity(
	ctx context.Context,
	domainName, queueName, groupID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Créer les maps si nécessaire
	if _, exists := r.timestamps[domainName]; !exists {
		r.timestamps[domainName] = make(map[string]map[string]time.Time)
	}
	if _, exists := r.timestamps[domainName][queueName]; !exists {
		r.timestamps[domainName][queueName] = make(map[string]time.Time)
	}

	// Mettre à jour le timestamp
	r.timestamps[domainName][queueName][groupID] = time.Now()
	return nil
}
