package model

import "context"

// MessageProvider is an interface that will be implemented by MessageService
type MessageProvider interface {
	GetMessagesAfterIndex(ctx context.Context, domainName, queueName string, startIndex int64, limit int) ([]*Message, error)
}
