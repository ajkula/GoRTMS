package service

import (
	"context"
	"errors"
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
	rootCtx           context.Context
	logger            outbound.Logger
	consumerGroupRepo outbound.ConsumerGroupRepository
	messageRepo       outbound.MessageRepository
}

func NewConsumerGroupService(
	rootCtx context.Context,
	logger outbound.Logger,
	consumerGroupRepo outbound.ConsumerGroupRepository,
	messageRepo outbound.MessageRepository,
) inbound.ConsumerGroupService {
	service := &ConsumerGroupServiceImpl{
		rootCtx:           rootCtx,
		logger:            logger,
		consumerGroupRepo: consumerGroupRepo,
		messageRepo:       messageRepo,
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
			s.logger.Error("Error getting group details",
				"group", groupID,
				"ERROR", err)
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
				s.logger.Warn("Error deleting message after group removal",
					"message", msgID,
					"ERROR", err)
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
	s.logger.Debug("Getting pending messages for group " + domainName + "." + queueName + "." + groupID)

	_, err := s.GetGroupDetails(ctx, domainName, queueName, groupID)
	if err != nil {
		return nil, err
	}

	// Find pending mesages using ackMatrix
	matrix := s.messageRepo.GetOrCreateAckMatrix(domainName, queueName)
	if matrix == nil {
		s.logger.Info("No acknowledgment matrix found for " + domainName + "." + queueName)
		return []*model.Message{}, nil
	}

	// Get pending message IDs
	pendingIDs := matrix.GetPendingMessageIDs(groupID)
	if len(pendingIDs) == 0 {
		s.logger.Debug("No pending message IDs found", "group", groupID)
		return []*model.Message{}, nil
	}

	s.logger.Debug("Found pending messages",
		"count", len(pendingIDs),
		"group", groupID)

	// Get matching messages
	messages := make([]*model.Message, 0, len(pendingIDs))
	for _, msgID := range pendingIDs {
		msg, err := s.messageRepo.GetMessage(ctx, domainName, queueName, msgID)
		if err != nil {
			s.logger.Warn("Could not retrieve message",
				"message", msgID,
				"ERROR", err)
			continue
		}
		messages = append(messages, msg)
	}

	s.logger.Debug("Returning pending messages",
		"count", len(messages),
		"group", groupID)
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
				s.logger.Debug("Starting cleanup of stale consumer groups...")
				if err := s.CleanupStaleGroups(ctx, 4*time.Hour); err != nil {
					s.logger.Error("Error cleaning up stale consumer groups",
						"ERROR", err)
				} else {
					s.logger.Debug("Cleanup of stale consumer groups completed successfully")
				}
			}
		}
	}()
}
