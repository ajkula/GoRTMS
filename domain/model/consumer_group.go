package model

import (
	"time"
)

// ConsumerGroup représente ungroupe de consumers d'une queue
type ConsumerGroup struct {
	DomainName   string
	QueueName    string
	GroupID      string
	LastOffset   string    //ID du dernier msg consomé
	LastConsumed time.Time // Dernière consommation
	ConsumerIDs  []string  // Liste des consumers
}
