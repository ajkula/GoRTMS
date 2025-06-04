package outbound

import (
	"context"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
)

// defines storage operations for messages
type MessageRepository interface {
	// StoreMessage saves a message
	StoreMessage(ctx context.Context, domainName, queueName string, message *model.Message) error

	// GetMessage fetches a message by its ID
	GetMessage(ctx context.Context, domainName, queueName, messageID string) (*model.Message, error)

	// DeleteMessage removes a message
	DeleteMessage(ctx context.Context, domainName, queueName, messageID string) error

	// GetMessagesAfterIndex fetches messages starting from a given index
	// If startIndex = 0, same as the old GetMessages method
	GetMessagesAfterIndex(
		ctx context.Context,
		domainName, queueName string, startIndex int64,
		limit int,
	) ([]*model.Message, error)

	// Get the index of a message by its ID
	GetIndexByMessageID(ctx context.Context, domainName, queueName, messageID string) (int64, error)

	// Get or create the acknowledgment matrix for a queue
	GetOrCreateAckMatrix(domainName, queueName string) *model.AckMatrix

	// AcknowledgeMessage marks a message as acknowledged by a group
	// Returns true if acknowledged by all groups
	AcknowledgeMessage(
		ctx context.Context,
		domainName, queueName, groupID, messageID string,
	) (bool, error)

	// Clear all index references for a specific queue
	ClearQueueIndices(
		ctx context.Context,
		domainName, queueName string,
	)

	// Cleanup old message index references
	CleanupMessageIndices(
		ctx context.Context,
		domainName, queueName string,
		minPosition int64,
	)

	// Get the number of messages in a queue
	GetQueueMessageCount(domainName, queueName string) int
}

// defines storage operations for domains
type DomainRepository interface {
	// StoreDomain saves a domain
	StoreDomain(ctx context.Context, domain *model.Domain) error

	// GetDomain fetches a domain by name
	GetDomain(ctx context.Context, name string) (*model.Domain, error)

	// DeleteDomain removes a domain
	DeleteDomain(ctx context.Context, name string) error

	// ListDomains lists all domains
	ListDomains(ctx context.Context) ([]*model.Domain, error)
}

// defines storage operations for queues
type QueueRepository interface {
	// StoreQueue saves a queue
	StoreQueue(ctx context.Context, domainName string, queue *model.Queue) error

	// GetQueue fetches a queue by name
	GetQueue(ctx context.Context, domainName, queueName string) (*model.Queue, error)

	// DeleteQueue removes a queue
	DeleteQueue(ctx context.Context, domainName, queueName string) error

	// ListQueues lists all queues in a domain
	ListQueues(ctx context.Context, domainName string) ([]*model.Queue, error)
}

// defines operations to manage subscriptions
type SubscriptionRegistry interface {
	// RegisterSubscription registers a new subscription
	RegisterSubscription(domainName, queueName string, handler model.MessageHandler) (string, error)

	// UnregisterSubscription removes a subscription
	UnregisterSubscription(subscriptionID string) error

	// NotifySubscribers sends a message to all subscribers
	NotifySubscribers(domainName, queueName string, message *model.Message) error
}

// defines operations for consumer groups
type ConsumerGroupRepository interface {
	// StorePosition saves a group offset
	StorePosition(ctx context.Context, domainName, queueNamme, groupID string, index int64) error

	// GetPosition retrieves a group's last offset
	GetPosition(ctx context.Context, domainName, queueName, groupID string) (int64, error)

	// RegisterConsumer adds a consumer to a group
	RegisterConsumer(ctx context.Context, domainName, queueName, groupID, consumerID string) error

	// RemoveConsumer removes a consumer from a group
	RemoveConsumer(ctx context.Context, domainName, queueName, groupID, consumerID string) error

	// ListGroups lists all groups for a queue
	ListGroups(ctx context.Context, domainName, queueName string) ([]string, error)

	// Delete a group
	DeleteGroup(ctx context.Context, domainName, queueName, groupID string) error

	// Cleanup inactive groups older than given duration
	CleanupStaleGroups(ctx context.Context, olderThan time.Duration) error

	// Set TTL for a group
	SetGroupTTL(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error

	// Update last activity timestamp for a group
	UpdateLastActivity(ctx context.Context, domainName, queueName, groupID string) error
}
