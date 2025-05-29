package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

var (
	ErrDomainNotFound     = errors.New("domain not found")
	ErrQueueNotFound      = errors.New("queue not found")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrSubscriptionFailed = errors.New("subscription failed")
)

var _ model.MessageProvider = (*MessageServiceImpl)(nil)

type MessageServiceImpl struct {
	rootCtx           context.Context
	logger            outbound.Logger
	domainRepo        outbound.DomainRepository
	messageRepo       outbound.MessageRepository
	consumerGroupRepo outbound.ConsumerGroupRepository
	subscriptionReg   outbound.SubscriptionRegistry
	queueService      inbound.QueueService
	statsService      inbound.StatsService

	// Periodic clean counter
	messageCountSinceLastCleanup int
	cleanupMu                    sync.Mutex
}

func NewMessageService(
	rootCtx context.Context,
	logger outbound.Logger,
	domainRepo outbound.DomainRepository,
	messageRepo outbound.MessageRepository,
	consumerGroupRepo outbound.ConsumerGroupRepository,
	subscriptionReg outbound.SubscriptionRegistry,
	queueService inbound.QueueService,
	statsService ...inbound.StatsService,
) inbound.MessageService {
	impl := &MessageServiceImpl{
		rootCtx:           rootCtx,
		logger:            logger,
		domainRepo:        domainRepo,
		messageRepo:       messageRepo,
		consumerGroupRepo: consumerGroupRepo,
		subscriptionReg:   subscriptionReg,
		queueService:      queueService,
	}

	if len(statsService) > 0 {
		impl.statsService = statsService[0]
	}

	// Start clean tasks
	impl.startCleanupTasks(rootCtx)

	return impl
}

func (s *MessageServiceImpl) PublishMessage(
	domainName, queueName string,
	message *model.Message,
) error {
	domain, err := s.domainRepo.GetDomain(s.rootCtx, domainName)
	if err != nil {
		return ErrDomainNotFound
	}

	channelQueue, err := s.queueService.GetChannelQueue(s.rootCtx, domainName, queueName)
	if err != nil {
		return ErrQueueNotFound
	}

	// Validate schema for message
	if domain.Schema != nil && domain.Schema.Validation != nil {
		if err := domain.Schema.Validation(message.Payload); err != nil {
			return ErrInvalidMessage
		}
	} else if domain.Schema != nil && len(domain.Schema.Fields) > 0 {
		// if no custom func, use field validation
		var payload map[string]interface{}
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return ErrInvalidMessage
		}

		for fieldName, fieldType := range domain.Schema.Fields {
			fieldValue, exists := payload[fieldName]
			if !exists {
				return ErrInvalidMessage
			}

			// Simplified type validation
			switch fieldType {
			case model.StringType:
				if _, ok := fieldValue.(string); !ok {
					return ErrInvalidMessage
				}
			case model.NumberType:
				if _, ok := fieldValue.(float64); !ok {
					return ErrInvalidMessage
				}
			case model.BooleanType:
				if _, ok := fieldValue.(bool); !ok {
					return ErrInvalidMessage
				}
			}
		}
	}

	// Add metadata
	if message.Metadata == nil {
		message.Metadata = make(map[string]interface{})
	}
	message.Metadata["domain"] = domainName
	message.Metadata["queue"] = queueName

	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// Send to repository
	if err := s.messageRepo.StoreMessage(s.rootCtx, domainName, queueName, message); err != nil {
		return err
	}

	// Collect statistics
	if s.statsService != nil {
		s.statsService.TrackMessagePublished(domainName, queueName)
	}

	// Enqueue message in chan queue
	_ = channelQueue.Enqueue(s.rootCtx, message)

	// Notify websockets
	_ = s.subscriptionReg.NotifySubscribers(domainName, queueName, message)

	// Apply routing rules
	if routes, exists := domain.Routes[queueName]; exists {
		for destQueue, rule := range routes {
			// Convert predicate to correct type
			var match bool

			switch pred := rule.Predicate.(type) {
			case model.PredicateFunc:
				// Use function directly
				match = pred(message)
			case model.JSONPredicate:
				// Evaluate JSON predicate
				match = s.evaluateJSONPredicate(pred, message)
			case map[string]any:
				// Convert map to JSONPredicate
				jsonPred := model.JSONPredicate{
					Type:  fmt.Sprintf("%v", pred["type"]),
					Field: fmt.Sprintf("%v", pred["field"]),
					Value: pred["value"],
				}
				match = s.evaluateJSONPredicate(jsonPred, message)
			default:
				s.logger.Warn("Unknown predicate type", "predicate", rule.Predicate)
			}

			if match {
				// push a copy to queue
				destMsg := *message
				if err := s.PublishMessage(domainName, destQueue, &destMsg); err != nil {
					return err
				}
			}
		}
	} else {
		s.logger.Info("No routes found for queue", "queue", queueName)
	}

	return nil
}

func (s *MessageServiceImpl) ConsumeMessageWithGroup(
	ctx context.Context,
	domainName, queueName, groupID string,
	options *inbound.ConsumeOptions,
) (*model.Message, error) {
	now := time.Now()
	if options == nil {
		options = &inbound.ConsumeOptions{}
	}

	channelQueue, err := s.queueService.GetChannelQueue(ctx, domainName, queueName)
	if err != nil {
		return nil, err
	}

	// Cast to ChannelQueue to access specific methods
	chQueue, ok := channelQueue.(*model.ChannelQueue)
	if !ok {
		return nil, errors.New("unexpected queue type")
	}

	position, err := s.consumerGroupRepo.GetPosition(ctx, domainName, queueName, groupID)
	if err != nil {
		position = 0
	}

	// Store consumer group to channel queue
	chQueue.AddConsumerGroup(groupID, position)

	// Store consumer to repository
	if options != nil && options.ConsumerID != "" {
		_ = s.consumerGroupRepo.RegisterConsumer(ctx, domainName, queueName, groupID, options.ConsumerID)
	}

	// Check group chan for messages
	message, err := chQueue.ConsumeMessage(groupID, 10*time.Millisecond)
	if err != nil {
		s.logger.Error("ConsumeMessageWithGroup chQueue.ConsumeMessage",
			"duration", time.Since(now).String(),
			"group", groupID,
			"ERROR", err)
	}

	if message == nil {
		maxCount := 5
		if options.MaxCount > 0 {
			maxCount = options.MaxCount
		}
		// If no messages send command
		channelQueue.RequestMessages(groupID, maxCount)

		timeout := 1 * time.Second
		if options != nil && options.Timeout > 0 {
			timeout = options.Timeout
		}

		// [CHECK] Waits for a message with full timeout duration = not working
		message, err = chQueue.ConsumeMessage(groupID, timeout)
		if err != nil {
			s.logger.Error("ConsumeMessageWithGroup chQueue.ConsumeMessage",
				"duration", time.Since(now).String(),
				"group", groupID,
				"timeout", timeout,
				"ERROR", err)
		}
	}

	// msg found -> auto ack update Pos
	if message != nil {
		if repo, ok := s.consumerGroupRepo.(interface {
			UpdateLastActivity(ctx context.Context, domainName, queueName, groupID string) error
		}); ok {
			if err = repo.UpdateLastActivity(ctx, domainName, queueName, groupID); err != nil {
				s.logger.Error("ConsumeMessageWithGroup updating last activity",
					"duration", time.Since(now).String(),
					"ERROR", err)
			}
		}

		index, err := s.messageRepo.GetIndexByMessageID(ctx, domainName, queueName, message.ID)
		if err != nil {
			s.logger.Error("ConsumeMessageWithGroup s.messageRepo.GetIndexByMessageID",
				"duration", time.Since(now).String(),
				"ERROR", err)
		} else {
			// Store next msg index as Pos
			newPosition := index + 1
			if err := s.consumerGroupRepo.StorePosition(ctx, domainName, queueName, groupID, newPosition); err != nil {
				s.logger.Error("ConsumeMessageWithGroup StorePosition",
					"duration", time.Since(now).String(),
					"ERROR", err)
				return nil, err
			}

			// IMPORTANT: Update Pos after store in repository
			chQueue.UpdateConsumerGroupPosition(groupID, newPosition)
		}

		// Elevate post treatment to asynchronous execution with new dedicated ctx
		bgCtx := context.Background()
		msgCopy := *message // Copy used to avoid race conditions

		go func(
			ctx context.Context,
			domainName, queueName, groupID, messageID string,
			startTime time.Time,
		) {
			// Acquitter automatiquement
			fullyAcked, err := s.messageRepo.AcknowledgeMessage(ctx, domainName, queueName, groupID, message.ID)
			if err != nil {
				s.logger.Error("ConsumeMessageWithGroup AcknowledgeMessage",
					"duration", time.Since(now).String(),
					"ERROR", err)
			}

			// delete if fully ack
			if fullyAcked {
				if err := s.messageRepo.DeleteMessage(ctx, domainName, queueName, message.ID); err != nil {
					// Ignore "message not found" error
					if err.Error() == "message not found" {
						s.logger.Error("Message already deleted",
							"message", message.ID)
					} else {
						s.logger.Error("Message not deleted",
							"message", message.ID,
							"ERROR", err)
					}
				}
			}

			// statistics
			if s.statsService != nil {
				s.statsService.TrackMessageConsumed(domainName, queueName)
			}

			// thread-safe counter increase
			s.cleanupMu.Lock()
			s.messageCountSinceLastCleanup++
			shouldCleanup := s.messageCountSinceLastCleanup >= 100
			if shouldCleanup {
				s.messageCountSinceLastCleanup = 0
			}
			s.cleanupMu.Unlock()

			// Clean indexs
			if shouldCleanup {
				// Find minimal pos cross group
				minPosition := int64(math.MaxInt64)
				groups, err := s.consumerGroupRepo.ListGroups(ctx, domainName, queueName)
				if err == nil && len(groups) > 0 {
					for _, gID := range groups {
						pos, err := s.consumerGroupRepo.GetPosition(ctx, domainName, queueName, gID)
						if err == nil && pos < minPosition && pos > 0 {
							minPosition = pos
						}
					}

					if minPosition < int64(math.MaxInt64) {
						safePosition := minPosition - 10 // Keep a secutiry margin
						if safePosition > 0 {
							s.messageRepo.CleanupMessageIndices(ctx, domainName, queueName, safePosition)
						}
					}
				}
			}
			s.logger.Debug("ConsumeMessageWithGroup Post Treatment Finished",
				"duration", time.Since(now).String())
		}(bgCtx, domainName, queueName, groupID, msgCopy.ID, now)

	}
	s.logger.Debug("ConsumeMessageWithGroup Finished",
		"duration", time.Since(now).String())

	return message, nil
}

func (s *MessageServiceImpl) GetMessagesAfterIndex(
	ctx context.Context,
	domainName, queueName string,
	startIndex int64,
	limit int,
) ([]*model.Message, error) {
	return s.messageRepo.GetMessagesAfterIndex(ctx, domainName, queueName, startIndex, limit)
}

func (s *MessageServiceImpl) SubscribeToQueue(
	domainName, queueName string,
	handler model.MessageHandler,
) (string, error) {
	domain, err := s.domainRepo.GetDomain(s.rootCtx, domainName)
	if err != nil {
		return "", ErrDomainNotFound
	}

	if _, exists := domain.Queues[queueName]; !exists {
		return "", ErrQueueNotFound
	}

	subscriptionID, err := s.subscriptionReg.RegisterSubscription(domainName, queueName, handler)
	if err != nil {
		return "", ErrSubscriptionFailed
	}

	return subscriptionID, nil
}

func (s *MessageServiceImpl) UnsubscribeFromQueue(
	domainName, queueName string,
	subscriptionID string,
) error {
	return s.subscriptionReg.UnregisterSubscription(subscriptionID)
}

func (s *MessageServiceImpl) evaluateJSONPredicate(predicate model.JSONPredicate, message *model.Message) bool {
	var payload map[string]interface{}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return false
	}

	fieldValue, exists := payload[predicate.Field]
	if !exists {
		return false
	}

	switch predicate.Type {
	case "eq": // Equals
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", predicate.Value)
	case "ne": // Not equals
		return fieldValue != predicate.Value
	case "gt": // Greater than
		switch v := fieldValue.(type) {
		case float64:
			if pv, ok := predicate.Value.(float64); ok {
				return v > pv
			}
		}
	case "lt": // Inferior
		switch v := fieldValue.(type) {
		case float64:
			if pv, ok := predicate.Value.(float64); ok {
				return v < pv
			}
		}
	case "contains": // for strings
		switch v := fieldValue.(type) {
		case string:
			if pv, ok := predicate.Value.(string); ok {
				return strings.Contains(v, pv)
			}
		}
	}

	return false
}

func (s *MessageServiceImpl) startCleanupTasks(ctx context.Context) {
	// Track how long queue's been ConsumerGroup-less
	type QueueInactivity struct {
		firstEmptyTime time.Time
		checked        bool
	}

	queueInactivity := make(map[string]map[string]*QueueInactivity) // domainName -> queueName -> inactivity
	var inactivityMu sync.Mutex

	// Clean ophan messages periodically
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				inactivityMu.Lock()

				domains, err := s.domainRepo.ListDomains(ctx)
				if err != nil {
					inactivityMu.Unlock()
					continue
				}

				for _, domain := range domains {
					if _, exists := queueInactivity[domain.Name]; !exists {
						queueInactivity[domain.Name] = make(map[string]*QueueInactivity)
					}

					for queueName := range domain.Queues {
						if _, exists := queueInactivity[domain.Name][queueName]; !exists {
							queueInactivity[domain.Name][queueName] = &QueueInactivity{}
						}

						inactivityInfo := queueInactivity[domain.Name][queueName]

						groupIDs, err := s.consumerGroupRepo.ListGroups(ctx, domain.Name, queueName)

						if err == nil && len(groupIDs) > 0 {
							// If consumer groups, reset tracking
							inactivityInfo.firstEmptyTime = time.Time{} // Zero time
							inactivityInfo.checked = false
							continue
						}

						// no consumer groups, check duration
						now := time.Now()

						if inactivityInfo.firstEmptyTime.IsZero() {
							inactivityInfo.firstEmptyTime = now
							s.logger.Debug("Queue " + domain.Name + "." + queueName + " sans consumer groups, début du tracking")
						} else if now.Sub(inactivityInfo.firstEmptyTime) > 24*time.Hour && !inactivityInfo.checked {
							// without any consumer groups for more than 24h, clean
							s.logger.Debug("Nettoyage de la queue " + domain.Name + "." + queueName + " (inactive depuis >24h")

							messages, _ := s.messageRepo.GetMessagesAfterIndex(ctx, domain.Name, queueName, 0, 1000)
							for _, msg := range messages {
								_ = s.messageRepo.DeleteMessage(ctx, domain.Name, queueName, msg.ID)
							}

							s.messageRepo.ClearQueueIndices(ctx, domain.Name, queueName)

							// To avoid cleaning every cycle
							inactivityInfo.checked = true
						}
					}
				}

				inactivityMu.Unlock()
			}
		}
	}()
}

func (s *MessageServiceImpl) Cleanup() {
	s.logger.Info("Cleaning up message service ressource...")
	// managed by QueueService
}
