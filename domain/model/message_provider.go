package model

import "context"

// MessageProvider est une interface que MessageService implémentera
type MessageProvider interface {
	GetMessagesAfterID(ctx context.Context, domainName, queueName, startMessageID string, limit int) ([]*Message, error)
}
