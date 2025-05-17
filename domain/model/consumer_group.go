package model

import (
	"time"
)

// ConsumerGroup représente ungroupe de consumers d'une queue
type ConsumerGroup struct {
	DomainName   string
	QueueName    string
	GroupID      string
	Position     int64
	ConsumerIDs  []string      // Liste des consumers
	TTL          time.Duration // Nouveau: Durée avant inactivité
	CreatedAt    time.Time     // Nouveau: Date de création
	LastActivity time.Time     // Nouveau: Dernière activité (pas juste consommation)
	MessageCount int           // Nouveau: Messages en attente d'acquittement
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
