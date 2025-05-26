package service

import (
	"context"
	"errors"
	"log"
	"time"

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

	allGroups := []*model.ConsumerGroup{}
	return allGroups, nil
}

func (s *ConsumerGroupServiceImpl) CreateConsumerGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
	ttl time.Duration,
) error {
	// Register consumer group (creates the instance)
	if err := s.consumerGroupRepo.RegisterConsumer(ctx, domainName, queueName, groupID, ""); err != nil {
		return err
	}

	// Set TTL if provided
	if ttl > 0 {
		if repo, ok := s.consumerGroupRepo.(interface {
			GetGroupDetails(
				ctx context.Context,
				domainName, queueName, groupID string,
			) (*model.ConsumerGroup, error)
		}); ok {
			group, err := repo.GetGroupDetails(ctx, domainName, queueName, groupID)
			if err != nil {
				return err
			}
			group.SetTTL(ttl)
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
	return s.consumerGroupRepo.SetGroupTTL(ctx, domainName, queueName, groupID, ttl)
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

	empty := &model.ConsumerGroup{}
	return empty, errors.New("could not get group details")
}

func (s *ConsumerGroupServiceImpl) UpdateLastActivity(
	ctx context.Context,
	domainName, queueName, groupID, consumerID string,
) error {
	return s.consumerGroupRepo.UpdateLastActivity(ctx, domainName, queueName, groupID)
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
