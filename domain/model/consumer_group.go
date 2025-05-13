package model

import (
	"time"
)

// ConsumerGroup représente ungroupe de consumers d'une queue
type ConsumerGroup struct {
	DomainName   string
	QueueName    string
	GroupID      string
	LastOffset   string        //ID du dernier msg consomé
	ConsumerIDs  []string      // Liste des consumers
	TTL          time.Duration // Nouveau: Durée avant inactivité
	CreatedAt    time.Time     // Nouveau: Date de création
	LastActivity time.Time     // Nouveau: Dernière activité (pas juste consommation)
	MessageCount int           // Nouveau: Messages en attente d'acquittement
}
