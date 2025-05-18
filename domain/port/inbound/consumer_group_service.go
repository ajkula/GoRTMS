package inbound

import (
	"context"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
)

type ConsumerGroupService interface {
	ListConsumerGroups(ctx context.Context, domainName, queueName string) ([]*model.ConsumerGroup, error)
	ListAllGroups(ctx context.Context) ([]*model.ConsumerGroup, error)
	GetGroupDetails(ctx context.Context, domainName, queueName, groupID string) (*model.ConsumerGroup, error)
	CreateConsumerGroup(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error
	DeleteConsumerGroup(ctx context.Context, domainName, queueName, groupID string) error
	UpdateConsumerGroupTTL(ctx context.Context, domainName, queueName, groupID string, ttl time.Duration) error
	GetPendingMessages(ctx context.Context, domainName, queueName, groupID string) ([]*model.Message, error)
	// RegisterConsumer(...) error
	// RemoveConsumer(...) error
}
