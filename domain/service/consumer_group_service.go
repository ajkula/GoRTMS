package service

import (
	"context"
	"errors"
	"log"
	"time"

	"slices"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrConsumerGroupNotFound = errors.New("consumer group not found")
	ErrInvalidTTL            = errors.New("invalid TTL")
)

type ConsumerGroupServiceImpl struct {
	consumerGroupRepo outbound.ConsumerGroupRepository
	messageRepo       outbound.MessageRepository
	rootCtx           context.Context
}

func NewConsumerGroupService(
	consumerGroupRepo outbound.ConsumerGroupRepository,
	messageRepo outbound.MessageRepository,
	rootCtx context.Context,
) inbound.ConsumerGroupService {
	service := &ConsumerGroupServiceImpl{
		consumerGroupRepo: consumerGroupRepo,
		messageRepo:       messageRepo,
		rootCtx:           rootCtx,
	}

	// Start the clean interval task
	service.startCleanupTask(rootCtx)

	return service
}

func (s *ConsumerGroupServiceImpl) ListConsumerGroups(
	ctx context.Context,
	domainName, queueName string,
) ([]*model.ConsumerGroup, error) {
	groupIDs, err := s.consumerGroupRepo.ListGroups(ctx, domainName, queueName)
	if err != nil {
		return nil, err
	}

	// Convert obj to ConsumerGroup
	groups := make([]*model.ConsumerGroup, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		group, err := s.GetGroupDetails(ctx, domainName, queueName, groupID)
		if err != nil {
			log.Printf("Error getting details for group %s: %v", groupID, err)
			continue
		}
		groups = append(groups, group)
	}

	return groups, nil
}

func (s *ConsumerGroupServiceImpl) ListAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error) {

	if repo, ok := s.consumerGroupRepo.(interface {
		GetAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error)
	}); ok {
		return repo.GetAllGroups(ctx)
	}

	// add GetAllGroups method to repository.

	allGroups := []*model.ConsumerGroup{}

	return allGroups, nil
}

func (s *ConsumerGroupServiceImpl) CreateConsumerGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
	ttl time.Duration,
) error {
	// register consumer group
	if err := s.consumerGroupRepo.RegisterConsumer(ctx, domainName, queueName, groupID, ""); err != nil {
		return err
	}

	// store TTL if exists
	if ttl > 0 {
		if storeTTLRepo, ok := s.consumerGroupRepo.(interface {
			StoreTTL(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error
		}); ok {
			if err := storeTTLRepo.StoreTTL(ctx, domainName, queueName, groupID, ttl); err != nil {
				return err
			}
		} else {
			// Fallback
			log.Printf("WARNING: StoreTTL not implemented in repository, TTL will not be stored")
		}
	}

	return nil
}

func (s *ConsumerGroupServiceImpl) DeleteConsumerGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
) error {
	// Clean AckMatrix
	ackMatrix := s.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	if ackMatrix != nil {
		messageIDs := ackMatrix.RemoveGroup(groupID)
		for _, msgID := range messageIDs {
			if err := s.messageRepo.DeleteMessage(ctx, domainName, queueName, msgID); err != nil {
				log.Printf("[WARN] Error deleting message %s after group removal: %v", msgID, err)
			}
		}
	}

	return s.consumerGroupRepo.DeleteGroup(ctx, domainName, queueName, groupID)
}

func (s *ConsumerGroupServiceImpl) UpdateConsumerGroupTTL(
	ctx context.Context,
	domainName, queueName, groupID string,
	ttl time.Duration,
) error {
	_, err := s.GetGroupDetails(ctx, domainName, queueName, groupID)
	if err != nil {
		return err
	}

	// Update TTL from repo
	if storeTTLRepo, ok := s.consumerGroupRepo.(interface {
		StoreTTL(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error
	}); ok {
		return storeTTLRepo.StoreTTL(ctx, domainName, queueName, groupID, ttl)
	}

	return errors.New("TTL update not supported by repository")
}

func (s *ConsumerGroupServiceImpl) CleanupStaleGroups(
	ctx context.Context,
	olderThan time.Duration,
) error {
	return s.consumerGroupRepo.CleanupStaleGroups(ctx, olderThan)
}

func (s *ConsumerGroupServiceImpl) GetGroupDetails(
	ctx context.Context,
	domainName, queueName, groupID string,
) (*model.ConsumerGroup, error) {
	// from repository
	if repo, ok := s.consumerGroupRepo.(interface {
		GetGroupDetails(ctx context.Context, domainName, queueName, groupID string) (*model.ConsumerGroup, error)
	}); ok {
		group, err := repo.GetGroupDetails(ctx, domainName, queueName, groupID)
		if err != nil {
			return nil, err
		}

		return group, nil
	}

	// Or build manually from available informations
	position, err := s.consumerGroupRepo.GetPosition(ctx, domainName, queueName, groupID)
	if err != nil {
		groups, err := s.consumerGroupRepo.ListGroups(ctx, domainName, queueName)
		if err != nil {
			return nil, err
		}

		groupExists := slices.Contains(groups, groupID)

		if !groupExists {
			return nil, ErrConsumerGroupNotFound
		}
	}

	// Consumer IDs
	consumerIDs := []string{}
	if repo, ok := s.consumerGroupRepo.(interface {
		GetConsumerIDs(ctx context.Context, domainName, queueName, groupID string) ([]string, error)
	}); ok {
		ids, err := repo.GetConsumerIDs(ctx, domainName, queueName, groupID)
		if err != nil {
			log.Printf("Warning: Could not retrieve consumer IDs: %v", err)
		} else {
			consumerIDs = ids
		}
	}

	// TTL
	var ttl time.Duration
	if repo, ok := s.consumerGroupRepo.(interface {
		GetTTL(ctx context.Context, domainName, queueName, groupID string) (time.Duration, error)
	}); ok {
		t, err := repo.GetTTL(ctx, domainName, queueName, groupID)
		if err != nil {
			log.Printf("Warning: Could not retrieve TTL: %v", err)
		} else {
			ttl = t
		}
	}

	// timestamps
	var createdAt, lastActivity time.Time
	if repo, ok := s.consumerGroupRepo.(interface {
		GetCreationTime(ctx context.Context, domainName, queueName, groupID string) (time.Time, error)
		GetLastActivity(ctx context.Context, domainName, queueName, groupID string) (time.Time, error)
	}); ok {
		createdAt, err = repo.GetCreationTime(ctx, domainName, queueName, groupID)
		if err != nil {
			log.Printf("[ERROR] Getting creation date: %s", err)
		}
		lastActivity, err = repo.GetLastActivity(ctx, domainName, queueName, groupID)
		if err != nil {
			log.Printf("[ERROR] Getting last activity: %s", err)
		}
	}

	// Count pending messages
	messageCount := 0
	matrix := s.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	if matrix != nil {
		messageCount = matrix.GetPendingMessageCount(groupID)
	}

	group := &model.ConsumerGroup{
		DomainName:   domainName,
		QueueName:    queueName,
		GroupID:      groupID,
		Position:     position,
		ConsumerIDs:  consumerIDs,
		TTL:          ttl,
		CreatedAt:    createdAt,
		LastActivity: lastActivity,
		MessageCount: messageCount,
	}

	// Finally update last activity
	if updater, ok := s.consumerGroupRepo.(interface {
		UpdateLastActivity(ctx context.Context, domainName, queueName, groupID string, t time.Time) error
	}); ok {
		now := time.Now()
		if err := updater.UpdateLastActivity(ctx, domainName, queueName, groupID, now); err != nil {
			log.Printf("Warning: Failed to update last activity: %v", err)
		} else {
			group.LastActivity = now
		}
	}

	return group, nil
}

func (s *ConsumerGroupServiceImpl) UpdateLastActivity(
	ctx context.Context,
	domainName, queueName, groupID, consumerID string,
) error {
	if updater, ok := s.consumerGroupRepo.(interface {
		UpdateLastActivity(ctx context.Context, domainName, queueName, groupID string, t time.Time) error
	}); ok {
		return updater.UpdateLastActivity(ctx, domainName, queueName, groupID, time.Now())
	}
	return nil // Don't Error
}

func (s *ConsumerGroupServiceImpl) GetPendingMessages(ctx context.Context, domainName, queueName, groupID string) ([]*model.Message, error) {
	log.Printf("Getting pending messages for group %s.%s.%s", domainName, queueName, groupID)

	_, err := s.GetGroupDetails(ctx, domainName, queueName, groupID)
	if err != nil {
		return nil, err
	}

	// Find pending mesages using ackMatrix
	matrix := s.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	if matrix == nil {
		log.Printf("No acknowledgment matrix found for %s.%s", domainName, queueName)
		return []*model.Message{}, nil
	}

	// Get pending message IDs
	pendingIDs := matrix.GetPendingMessageIDs(groupID)
	if len(pendingIDs) == 0 {
		log.Printf("No pending message IDs found for group %s", groupID)
		return []*model.Message{}, nil
	}

	log.Printf("Found %d pending message IDs for group %s", len(pendingIDs), groupID)

	// Get matching messages
	messages := make([]*model.Message, 0, len(pendingIDs))
	for _, msgID := range pendingIDs {
		msg, err := s.messageRepo.GetMessage(ctx, domainName, queueName, msgID)
		if err != nil {
			log.Printf("Warning: Could not retrieve message %s: %v", msgID, err)
			continue
		}
		messages = append(messages, msg)
	}

	log.Printf("Returning %d pending messages for group %s", len(messages), groupID)
	return messages, nil
}

func (s *ConsumerGroupServiceImpl) startCleanupTask(ctx context.Context) {
	go func() {
		// Cleanup every 5 minutes
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Printf("Starting cleanup of stale consumer groups...")
				if err := s.CleanupStaleGroups(ctx, 4*time.Hour); err != nil {
					log.Printf("Error cleaning up stale consumer groups: %v", err)
				} else {
					log.Printf("Cleanup of stale consumer groups completed successfully")
				}
			}
		}
	}()
}
