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

type ConsumerGroupRepository struct {
	// Map Domain -> Queue -> GroupID -> Position
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

// Makes a repository
func NewConsumerGroupRepository(messageRepo outbound.MessageRepository) outbound.ConsumerGroupRepository {
	return &ConsumerGroupRepository{
		positions:   make(map[string]map[string]map[string]int64),
		timestamps:  make(map[string]map[string]map[string]time.Time),
		consumers:   make(map[string]map[string]map[string][]string),
		ttls:        make(map[string]map[string]map[string]time.Duration),
		messageRepo: messageRepo,
	}
}

func (r *ConsumerGroupRepository) StorePosition(
	ctx context.Context,
	domainName, queueName, groupID string, position int64,
) error {
	if domainName == "" || queueName == "" || groupID == "" {
		return errors.New("all parameters must be non-empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Protection vs nil maps
	if r.positions == nil {
		log.Printf("[WARNING]: r.positions was nil in StorePosition - this should never happen!")
		r.positions = make(map[string]map[string]map[string]int64)
	}
	if r.timestamps == nil {
		log.Printf("[WARNING]: r.timestamps was nil in StorePosition - this should never happen!")
		r.timestamps = make(map[string]map[string]map[string]time.Time)
	}

	// find or create
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

	currentPosition, exists := r.positions[domainName][queueName][groupID]
	if exists && position <= currentPosition {
		return nil
	}

	// Store Pos and timestamp
	r.positions[domainName][queueName][groupID] = position
	r.timestamps[domainName][queueName][groupID] = time.Now()

	return nil
}

func (r *ConsumerGroupRepository) GetPosition(
	ctx context.Context,
	domainName, queueName, groupID string,
) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// if Pos exists
	if _, exists := r.positions[domainName]; !exists {
		log.Printf("Domain not found in positions: %v", domainName)
		return 0, nil
	}
	if _, exists := r.positions[domainName][queueName]; !exists {
		log.Printf("Queue not found in domain: %v", queueName)
		return 0, nil
	}

	position, exists := r.positions[domainName][queueName][groupID]
	if !exists {
		log.Printf("Group not found in queue: %v  [%v]", groupID, position)
		return 0, nil
	}

	return position, nil
}

func (r *ConsumerGroupRepository) RegisterConsumer(
	ctx context.Context,
	domainName, queueName, groupID, consumerID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// find or create
	if _, exists := r.consumers[domainName]; !exists {
		r.consumers[domainName] = make(map[string]map[string][]string)
	}
	if _, exists := r.consumers[domainName][queueName]; !exists {
		r.consumers[domainName][queueName] = make(map[string][]string)
	}

	// if exists
	consumerList, exists := r.consumers[domainName][queueName][groupID]
	if !exists {
		// New consumers list
		r.consumers[domainName][queueName][groupID] = []string{}
		return nil
	}

	// if exists
	if slices.Contains(consumerList, consumerID) {
		return nil
	}

	// Add consumer
	r.consumers[domainName][queueName][groupID] = append(consumerList, consumerID)

	// Update activity timestamp
	if _, exists := r.timestamps[domainName]; !exists {
		r.timestamps[domainName] = make(map[string]map[string]time.Time)
	}
	if _, exists := r.timestamps[domainName][queueName]; !exists {
		r.timestamps[domainName][queueName] = make(map[string]time.Time)
	}
	r.timestamps[domainName][queueName][groupID] = time.Now()

	// Add group to ackMatrix
	matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	matrix.RegisterGroup(groupID)

	return nil
}

func (r *ConsumerGroupRepository) RemoveConsumer(
	ctx context.Context,
	domainName, queueName, groupID, consumerID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.consumers[domainName]; !exists {
		return nil
	}
	if _, exists := r.consumers[domainName][queueName]; !exists {
		return nil
	}

	consumerList, exists := r.consumers[domainName][queueName][groupID]
	if !exists {
		return nil
	}

	// Find and remove consumer
	for i, id := range consumerList {
		if id == consumerID {
			// remove consumer preserving order
			r.consumers[domainName][queueName][groupID] = append(
				consumerList[:i],
				consumerList[i+1:]...,
			)
			break
		}
	}

	// update activity timestamp
	if _, exists := r.timestamps[domainName]; exists {
		if _, exists := r.timestamps[domainName][queueName]; exists {
			r.timestamps[domainName][queueName][groupID] = time.Now()
		}
	}

	// if last consumer, remove group from matrix
	if _, exists := r.consumers[domainName]; exists {
		if queueConsumers, exists := r.consumers[domainName][queueName]; exists {
			if consumerList, exists := queueConsumers[groupID]; exists && len(consumerList) == 0 {
				// remove from ackMatrix
				matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
				messagesToDelete := matrix.RemoveGroup(groupID)

				// delete fully ack msgs
				for _, msgID := range messagesToDelete {
					r.messageRepo.DeleteMessage(ctx, domainName, queueName, msgID)
				}
			}
		}
	}

	return nil
}

func (r *ConsumerGroupRepository) ListGroups(
	ctx context.Context,
	domainName, queueName string,
) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0)

	// Check offsets
	if position, exists := r.positions[domainName]; exists {
		if queuePos, exists := position[queueName]; exists {
			for groupID := range queuePos {
				result = append(result, groupID)
			}
			return result, nil
		}
	}

	// Check consumers if no pos
	if consumers, exists := r.consumers[domainName]; exists {
		if queueConsumers, exists := consumers[queueName]; exists {
			for groupID := range queueConsumers {
				// check if not already in results
				found := slices.Contains(result, groupID)
				if !found {
					result = append(result, groupID)
				}
			}
		}
	}

	return result, nil
}

func (r *ConsumerGroupRepository) DeleteGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete from all maps
	if domainPositions, ok := r.positions[domainName]; ok {
		if queuePositions, ok := domainPositions[queueName]; ok {
			delete(queuePositions, groupID)
		}
	}

	if domainTTL, ok := r.ttls[domainName]; ok {
		if queueTTLs, ok := domainTTL[queueName]; ok {
			delete(queueTTLs, groupID)
		}
	}

	if domainConsumers, ok := r.consumers[domainName]; ok {
		if queueConsumers, ok := domainConsumers[queueName]; ok {
			delete(queueConsumers, groupID)
		}
	}

	if domainTimestamps, ok := r.timestamps[domainName]; ok {
		if queueTimestamps, ok := domainTimestamps[queueName]; ok {
			delete(queueTimestamps, groupID)
		}
	}

	return nil
}

func (r *ConsumerGroupRepository) CleanupStaleGroups(ctx context.Context, olderThan time.Duration) error {
	// Add explicit timeout
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// IMPORTANT: Use deadlock safe lock with timeout
	lockAcquired := make(chan struct{}, 1)
	go func() {
		r.mu.Lock()
		select {
		case lockAcquired <- struct{}{}:
			// Lock sent to chan
		case <-cleanupCtx.Done():
			// free lock if still closed when ctx closed
			r.mu.Unlock()
		}
	}()

	select {
	case <-lockAcquired:
		defer r.mu.Unlock()
	case <-cleanupCtx.Done():
		log.Printf("[WARN] Timeout while acquiring lock for CleanupStaleGroups, operation aborted")
		return fmt.Errorf("timeout while acquiring lock for cleanup")
	}

	log.Printf("[DEBUG] Starting cleanup of stale consumer groups (older than %s)", olderThan)

	cleanupCount := 0
	now := time.Now()

	// copy domains to avoid concurrency
	domainsList := make([]string, 0, len(r.timestamps))
	for domain := range r.timestamps {
		domainsList = append(domainsList, domain)
	}

	for _, domain := range domainsList {
		domainMap, exists := r.timestamps[domain]
		if !exists {
			continue
		}

		queuesList := make([]string, 0, len(domainMap))
		for queue := range domainMap {
			queuesList = append(queuesList, queue)
		}

		for _, queue := range queuesList {
			queueMap, exists := domainMap[queue]
			if !exists {
				continue
			}

			// Copy all groups from queue
			groupsList := make([]string, 0, len(queueMap))
			for group := range queueMap {
				groupsList = append(groupsList, group)
			}

			// each group from queue
			for _, group := range groupsList {
				lastActivity, exists := queueMap[group]
				if !exists {
					continue
				}

				// Check group TTL
				if lastActivity.Add(olderThan).Before(now) {
					log.Printf("[INFO] Removing stale consumer group %s.%s.%s (last activity: %v)",
						domain, queue, group, lastActivity)

					// delete group from all maps
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

					// delete timestamps lastly
					delete(queueMap, group)
					cleanupCount++

					// Clean AckMatrix
					ackMatrix := r.messageRepo.GetOrCreateAckMatrix(domain, queue)
					if ackMatrix != nil {
						messageIDs := ackMatrix.RemoveGroup(group)
						for _, msgID := range messageIDs {
							if err := r.messageRepo.DeleteMessage(ctx, domain, queue, msgID); err != nil {
								log.Printf("[WARN] Error deleting message %s after group removal: %v", msgID, err)
							}
						}
					}

					// Check if ctx closed
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

func (r *ConsumerGroupRepository) GetGroupDetails(
	ctx context.Context,
	domainName, queueName, groupID string,
) (*model.ConsumerGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

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

	var lastPosition int64
	if _, exists := r.positions[domainName]; exists {
		if _, exists := r.positions[domainName][queueName]; exists {
			lastPosition = r.positions[domainName][queueName][groupID]
		}
	}

	// Activity timestamp
	var lastActivity time.Time
	if _, exists := r.timestamps[domainName]; exists {
		if _, exists := r.timestamps[domainName][queueName]; exists {
			lastActivity = r.timestamps[domainName][queueName][groupID]
		}
	}

	group := &model.ConsumerGroup{
		DomainName:   domainName,
		QueueName:    queueName,
		GroupID:      groupID,
		Position:     lastPosition,
		ConsumerIDs:  make([]string, len(consumerList)),
		LastActivity: lastActivity,
		// Other fields todo...
	}

	copy(group.ConsumerIDs, consumerList)

	// Récupérer le TTL si disponible
	ttl, err := r.GetTTL(ctx, domainName, queueName, groupID)
	if err == nil {
		group.TTL = ttl
	}

	// Calculate waiting messages count
	matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	group.MessageCount = matrix.GetPendingMessageCount(groupID)

	return group, nil
}

func (r *ConsumerGroupRepository) StoreTTL(
	ctx context.Context,
	domainName, queueName, groupID string,
	ttl time.Duration,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Init maps
	if _, exists := r.ttls[domainName]; !exists {
		r.ttls = make(map[string]map[string]map[string]time.Duration)
		r.ttls[domainName] = make(map[string]map[string]time.Duration)
	}
	if _, exists := r.ttls[domainName][queueName]; !exists {
		r.ttls[domainName][queueName] = make(map[string]time.Duration)
	}

	r.ttls[domainName][queueName][groupID] = ttl
	return nil
}

func (r *ConsumerGroupRepository) GetTTL(
	ctx context.Context,
	domainName, queueName, groupID string,
) (time.Duration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if TTL is defined
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

func (r *ConsumerGroupRepository) GetAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allGroups []*model.ConsumerGroup

	for domainName, domainConsumers := range r.consumers {
		for queueName, queueConsumers := range domainConsumers {
			for groupID := range queueConsumers {
				log.Printf("[DEBUG] getting: %s.%s.%s", domainName, queueName, groupID)

				r.mu.RUnlock()
				group, err := r.GetGroupDetails(ctx, domainName, queueName, groupID)
				r.mu.RLock()

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

func (r *ConsumerGroupRepository) UpdateLastActivity(
	ctx context.Context,
	domainName, queueName, groupID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.timestamps[domainName]; !exists {
		r.timestamps[domainName] = make(map[string]map[string]time.Time)
	}
	if _, exists := r.timestamps[domainName][queueName]; !exists {
		r.timestamps[domainName][queueName] = make(map[string]time.Time)
	}

	r.timestamps[domainName][queueName][groupID] = time.Now()
	return nil
}
