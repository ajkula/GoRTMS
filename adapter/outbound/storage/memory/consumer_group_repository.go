package memory

import (
	"context"
	"errors"
	"fmt"
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
		return nil
	}

	// Stocker l'offset et le timestamp
	r.positions[domainName][queueName][groupID] = position
	r.timestamps[domainName][queueName][groupID] = time.Now()

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
func (r *ConsumerGroupRepository) CleanupStaleGroups(ctx context.Context, olderThan time.Duration) error {
	// Ajouter un timeout explicite à l'opération
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// IMPORTANT: Utiliser un verrou avec timeout pour éviter les blocages indéfinis
	lockAcquired := make(chan struct{}, 1)
	go func() {
		r.mu.Lock()
		select {
		case lockAcquired <- struct{}{}:
			// Le verrou a été envoyé au canal
		case <-cleanupCtx.Done():
			// Si le contexte est terminé mais qu'on a le verrou, le libérer
			r.mu.Unlock()
		}
	}()

	select {
	case <-lockAcquired:
		// Le verrou a été acquis, assurez-vous de le libérer
		defer r.mu.Unlock()
	case <-cleanupCtx.Done():
		// Timeout lors de l'acquisition du verrou
		log.Printf("[WARN] Timeout while acquiring lock for CleanupStaleGroups, operation aborted")
		return fmt.Errorf("timeout while acquiring lock for cleanup")
	}

	log.Printf("[DEBUG] Starting cleanup of stale consumer groups (older than %s)", olderThan)

	// Utiliser timestamps pour nettoyer les groupes inactifs
	cleanupCount := 0
	now := time.Now()

	// Faire une copie des domaines à parcourir pour éviter les problèmes de concurrence
	domainsList := make([]string, 0, len(r.timestamps))
	for domain := range r.timestamps {
		domainsList = append(domainsList, domain)
	}

	// Parcourir tous les domaines
	for _, domain := range domainsList {
		domainMap, exists := r.timestamps[domain]
		if !exists {
			continue
		}

		// Copier les queues pour ce domaine
		queuesList := make([]string, 0, len(domainMap))
		for queue := range domainMap {
			queuesList = append(queuesList, queue)
		}

		// Parcourir toutes les queues du domaine
		for _, queue := range queuesList {
			queueMap, exists := domainMap[queue]
			if !exists {
				continue
			}

			// Copier les groupes pour cette queue
			groupsList := make([]string, 0, len(queueMap))
			for group := range queueMap {
				groupsList = append(groupsList, group)
			}

			// Parcourir tous les groupes de la queue
			for _, group := range groupsList {
				lastActivity, exists := queueMap[group]
				if !exists {
					continue
				}

				// Vérifier si le groupe est inactif depuis trop longtemps
				if lastActivity.Add(olderThan).Before(now) {
					log.Printf("[INFO] Removing stale consumer group %s.%s.%s (last activity: %v)",
						domain, queue, group, lastActivity)

					// Supprimer le groupe de toutes les maps
					if domainPositions, ok := r.positions[domain]; ok {
						if queuePositions, ok := domainPositions[queue]; ok {
							delete(queuePositions, group)
						}
					}

					if domainTTLs, ok := r.ttls[domain]; ok {
						if queueTTLs, ok := domainTTLs[queue]; ok {
							delete(queueTTLs, group)
						}
					}

					if domainConsumers, ok := r.consumers[domain]; ok {
						if queueConsumers, ok := domainConsumers[queue]; ok {
							delete(queueConsumers, group)
						}
					}

					// Supprimer de timestamps en dernier pour la cohérence
					delete(queueMap, group)
					cleanupCount++

					// Nettoyer aussi la matrice d'acquittement si nécessaire
					ackMatrix := r.messageRepo.GetOrCreateAckMatrix(domain, queue)
					if ackMatrix != nil {
						messageIDs := ackMatrix.RemoveGroup(group)
						for _, msgID := range messageIDs {
							if err := r.messageRepo.DeleteMessage(ctx, domain, queue, msgID); err != nil {
								log.Printf("[WARN] Error deleting message %s after group removal: %v", msgID, err)
							}
						}
					}

					// Vérifier périodiquement si le contexte est terminé
					if cleanupCtx.Err() != nil {
						log.Printf("[WARN] Cleanup interrupted by context cancellation after processing %d groups", cleanupCount)
						return cleanupCtx.Err()
					}
				}
			}
		}
	}

	log.Printf("[INFO] Cleanup of stale consumer groups completed successfully: removed %d inactive groups", cleanupCount)
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
				log.Printf("[DEBUG] getting: %s.%s.%s", domainName, queueName, groupID)

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
	if len(allGroups) > 0 {
		log.Printf("[DEBUG] getting groups: %+v", allGroups[0])
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
