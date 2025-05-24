package model

import (
	"time"
)

// Queue consumers group
type ConsumerGroup struct {
	DomainName   string
	QueueName    string
	GroupID      string
	Position     int64
	CreatedAt    time.Time
	ConsumerIDs  []string      // Consumers
	TTL          time.Duration // Time to live
	LastActivity time.Time     // Last activity (any)
	MessageCount int           // Messages waiting ack
}

func (cg *ConsumerGroup) UpdatePosition(newPosition int64) {
	if newPosition > cg.Position {
		cg.Position = newPosition
		cg.LastActivity = time.Now()
	}
}

func (cg *ConsumerGroup) GetPosition() int64 {
	return cg.Position
}

func (cg *ConsumerGroup) SetCreatedAt(t time.Time) {
	cg.CreatedAt = t
	cg.LastActivity = time.Now()
}
