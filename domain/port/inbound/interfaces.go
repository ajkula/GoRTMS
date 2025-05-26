package inbound

import (
	"context"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
)

// ConsumeOptions defines options for consuming messages
type ConsumeOptions struct {
	StartFromID string
	ConsumerID  string
	Timeout     time.Duration
	MaxCount    int
}

// MessageService defines operations for messages
type MessageService interface {
	// PublishMessage publishes a message to a queue
	PublishMessage(domainName, queueName string, message *model.Message) error

	// SubscribeToQueue subscribes to a queue
	SubscribeToQueue(domainName, queueName string, handler model.MessageHandler) (string, error)

	// UnsubscribeFromQueue unsubscribes from a queue
	UnsubscribeFromQueue(domainName, queueName string, subscriptionID string) error

	// ConsumeMessageWithGroup consumes a message with offset management
	ConsumeMessageWithGroup(ctx context.Context,
		domainName, queueName, groupID string, options *ConsumeOptions,
	) (*model.Message, error)

	// GetMessagesAfterIndex returns messages from a given index
	GetMessagesAfterIndex(ctx context.Context, domainName, queueName string, startIndex int64, limit int) ([]*model.Message, error)
}

// DomainService defines operations for domains
type DomainService interface {
	// CreateDomain creates a new domain
	CreateDomain(ctx context.Context, config *model.DomainConfig) error

	// GetDomain retrieves an existing domain
	GetDomain(ctx context.Context, name string) (*model.Domain, error)

	// DeleteDomain deletes a domain
	DeleteDomain(ctx context.Context, name string) error

	// ListDomains lists all domains
	ListDomains(ctx context.Context) ([]*model.Domain, error)
}

// QueueService defines operations for queues
type QueueService interface {
	// CreateQueue creates a new queue
	CreateQueue(ctx context.Context, domainName, queueName string, config *model.QueueConfig) error

	// GetQueue retrieves an existing queue
	GetQueue(ctx context.Context, domainName, queueName string) (*model.Queue, error)

	// DeleteQueue deletes a queue
	DeleteQueue(ctx context.Context, domainName, queueName string) error

	// ListQueues lists all queues in a domain
	ListQueues(ctx context.Context, domainName string) ([]*model.Queue, error)

	// GetChannelQueue retrieves or creates a ChannelQueue for an existing queue
	GetChannelQueue(ctx context.Context, domainName, queueName string) (model.QueueHandler, error)

	// StopDomainQueues stops all queues for a domain
	StopDomainQueues(ctx context.Context, domainName string) error

	// Cleanup releases resources used by the service
	Cleanup()
}

// RoutingService defines operations for routing rules
type RoutingService interface {
	// AddRoutingRule adds a routing rule
	AddRoutingRule(ctx context.Context, domainName string, rule *model.RoutingRule) error

	// RemoveRoutingRule removes a routing rule
	RemoveRoutingRule(ctx context.Context, domainName string, sourceQueue, destQueue string) error

	// ListRoutingRules lists all routing rules for a domain
	ListRoutingRules(ctx context.Context, domainName string) ([]*model.RoutingRule, error)
}
