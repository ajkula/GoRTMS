package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type ConsumerGroupRepository struct {
	logger outbound.Logger
	// Map Domain -> Queue -> GroupID -> Consumer Group
	groups      map[string]map[string]map[string]*model.ConsumerGroup
	messageRepo outbound.MessageRepository
	mu          sync.RWMutex
}

// Makes a repository
func NewConsumerGroupRepository(
	logger outbound.Logger,
	messageRepo outbound.MessageRepository,
) outbound.ConsumerGroupRepository {
	return &ConsumerGroupRepository{
		logger:      logger,
		groups:      make(map[string]map[string]map[string]*model.ConsumerGroup),
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

	// find or create
	if _, exists := r.groups[domainName]; !exists {
		return errors.New("consumer group not found")
	}
	if _, exists := r.groups[domainName][queueName]; !exists {
		return errors.New("consumer group not found")
	}

	group, exists := r.groups[domainName][queueName][groupID]
	if !exists {
		return errors.New("consumer group not found")
	}

	// Store Pos
	group.UpdatePosition(position)
	return nil
}

func (r *ConsumerGroupRepository) GetPosition(
	ctx context.Context,
	domainName, queueName, groupID string,
) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// if Pos exists
	if _, exists := r.groups[domainName]; !exists {
		return 0, nil
	}
	if _, exists := r.groups[domainName][queueName]; !exists {
		return 0, nil
	}

	group, exists := r.groups[domainName][queueName][groupID]
	if !exists {
		return 0, nil
	}

	return group.Position, nil
}

func (r *ConsumerGroupRepository) RegisterConsumer(
	ctx context.Context,
	domainName, queueName, groupID, consumerID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize maps if needed
	if _, exists := r.groups[domainName]; !exists {
		r.groups[domainName] = make(map[string]map[string]*model.ConsumerGroup)
	}
	if _, exists := r.groups[domainName][queueName]; !exists {
		r.groups[domainName][queueName] = make(map[string]*model.ConsumerGroup)
	}

	// Get or create group
	group, exists := r.groups[domainName][queueName][groupID]
	if !exists {
		// Create new group
		now := time.Now()
		group = &model.ConsumerGroup{
			DomainName:   domainName,
			QueueName:    queueName,
			GroupID:      groupID,
			Position:     0,
			CreatedAt:    now,
			ConsumerIDs:  []string{},
			TTL:          0,
			LastActivity: now,
			MessageCount: 0,
		}
		r.groups[domainName][queueName][groupID] = group

		// Add group to ackMatrix
		matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
		matrix.RegisterGroup(groupID)
	}

	// Add consumer if provided
	if consumerID != "" {
		group.AddConsumer(consumerID)
	}

	return nil
}

func (r *ConsumerGroupRepository) RemoveConsumer(
	ctx context.Context,
	domainName, queueName, groupID, consumerID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if group exists
	if _, exists := r.groups[domainName]; !exists {
		return nil
	}
	if _, exists := r.groups[domainName][queueName]; !exists {
		return nil
	}

	group, exists := r.groups[domainName][queueName][groupID]
	if !exists {
		return nil
	}

	// Remove consumer using model method
	isEmpty := group.RemoveConsumer(consumerID)

	// If last consumer removed, clean up ackMatrix but keep group (respect TTL)
	if isEmpty {
		matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
		messagesToDelete := matrix.RemoveGroup(groupID)

		// Delete fully ack messages
		for _, msgID := range messagesToDelete {
			r.messageRepo.DeleteMessage(ctx, domainName, queueName, msgID)
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

	if _, exists := r.groups[domainName]; !exists {
		return result, nil
	}
	if _, exists := r.groups[domainName][queueName]; !exists {
		return result, nil
	}

	for groupID := range r.groups[domainName][queueName] {
		result = append(result, groupID)
	}

	return result, nil
}

func (r *ConsumerGroupRepository) DeleteGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete group instance
	if _, exists := r.groups[domainName]; !exists {
		if _, exists := r.groups[domainName][queueName]; !exists {
			delete(r.groups[domainName][queueName], groupID)
		}
	}

	return nil
}

func (r *ConsumerGroupRepository) CleanupStaleGroups(ctx context.Context, olderThan time.Duration) error {
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	lockAcquired := make(chan struct{}, 1)
	go func() {
		r.mu.Lock()
		select {
		case lockAcquired <- struct{}{}:
		case <-cleanupCtx.Done():
			r.mu.Unlock()
		}
	}()

	select {
	case <-lockAcquired:
		defer r.mu.Unlock()
	case <-cleanupCtx.Done():
		warning := fmt.Errorf("timeout while acquiring lock for cleanup")
		r.logger.Warn(warning.Error())
		return warning
	}

	r.logger.Debug("Starting cleanup of stale consumer groups", "olderThan", olderThan.String())

	cleanupCount := 0

	for domainName, domainGroups := range r.groups {
		for queueName, queueGroups := range domainGroups {
			for groupID, group := range queueGroups {
				if group.IsExpired(olderThan) {
					r.logger.Info("Removing stale consumer group " + domainName + "." + queueName + "." + groupID)

					// Clean AckMatrix
					ackMatrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
					if ackMatrix != nil {
						messageIDs := ackMatrix.RemoveGroup(groupID)
						for _, msgID := range messageIDs {
							r.messageRepo.DeleteMessage(ctx, domainName, queueName, msgID)
						}
					}

					// Delete group
					delete(queueGroups, groupID)
					cleanupCount++

					if cleanupCtx.Err() != nil {
						return cleanupCtx.Err()
					}
				}
			}
		}
	}

	r.logger.Info("Cleanup completed removing inactive groups", "cleanupCount", cleanupCount)
	return nil
}

func (r *ConsumerGroupRepository) GetGroupDetails(
	ctx context.Context,
	domainName, queueName, groupID string,
) (*model.ConsumerGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.groups[domainName]; !exists {
		return nil, errors.New("consumer group not found")
	}
	if _, exists := r.groups[domainName][queueName]; !exists {
		return nil, errors.New("consumer group not found")
	}
	group, exists := r.groups[domainName][queueName][groupID]
	if !exists {
		return nil, errors.New("consumer group not found")
	}

	// Calculate waiting messages count
	matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	group.MessageCount = matrix.GetPendingMessageCount(groupID)

	return group, nil
}

func (r *ConsumerGroupRepository) GetAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allGroups []*model.ConsumerGroup

	for domainName, domainGroups := range r.groups {
		for queueName, queueGroups := range domainGroups {
			for _, group := range queueGroups {
				// Update message count
				matrix := r.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
				group.MessageCount = matrix.GetPendingMessageCount(group.GroupID)

				allGroups = append(allGroups, group)
			}
		}
	}

	return allGroups, nil
}

func (r *ConsumerGroupRepository) UpdateLastActivity(
	ctx context.Context,
	domainName, queueName, groupID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if group exists
	if _, exists := r.groups[domainName]; !exists {
		return errors.New("consumer group not found")
	}
	if _, exists := r.groups[domainName][queueName]; !exists {
		return errors.New("consumer group not found")
	}

	group, exists := r.groups[domainName][queueName][groupID]
	if !exists {
		return errors.New("consumer group not found")
	}

	group.UpdateActivity()
	return nil
}

func (r *ConsumerGroupRepository) SetGroupTTL(
	ctx context.Context,
	domainName, queueName, groupID string,
	ttl time.Duration,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[domainName]; !exists {
		return errors.New("consumer group not found")
	}
	if _, exists := r.groups[domainName][queueName]; !exists {
		return errors.New("consumer group not found")
	}

	group, exists := r.groups[domainName][queueName][groupID]
	if !exists {
		return errors.New("consumer group not found")
	}
	group.SetTTL(ttl)

	return nil
}
