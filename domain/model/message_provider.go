package model

import "context"

// MessageProvider est une interface que MessageService implémentera
type MessageProvider interface {
	GetMessagesAfterIndex(ctx context.Context, domainName, queueName string, startIndex int64, limit int) ([]*Message, error)
}
