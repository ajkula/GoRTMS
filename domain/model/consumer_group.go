package model

import (
	"slices"
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
	MessageCount int           // Messages waiting for acknowledgment
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

// Consumer management
func (cg *ConsumerGroup) AddConsumer(consumerID string) {
	if !slices.Contains(cg.ConsumerIDs, consumerID) {
		cg.ConsumerIDs = append(cg.ConsumerIDs, consumerID)
		cg.LastActivity = time.Now()
	}
}

func (cg *ConsumerGroup) RemoveConsumer(consumerID string) bool {
	for i, id := range cg.ConsumerIDs {
		if id == consumerID {
			cg.ConsumerIDs = append(cg.ConsumerIDs[:i], cg.ConsumerIDs[i+1:]...)
			cg.LastActivity = time.Now()
			return len(cg.ConsumerIDs) == 0 // returns true if last consumer
		}
	}
	return false
}

// TTL management
func (cg *ConsumerGroup) SetTTL(ttl time.Duration) {
	cg.TTL = ttl
	cg.LastActivity = time.Now()
}

func (cg *ConsumerGroup) IsExpired(maxAge time.Duration) bool {
	return time.Since(cg.LastActivity) > maxAge
}

func (cg *ConsumerGroup) UpdateActivity() {
	cg.LastActivity = time.Now()
}
