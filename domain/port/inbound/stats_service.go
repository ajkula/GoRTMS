package inbound

import (
	"context"
)

// StatsService définit les opérations pour les statistiques du système
type StatsService interface {
	// GetStats récupère les statistiques du système
	GetStats(ctx context.Context) (any, error)

	// TrackMessagePublished enregistre un message publié dans les métriques
	TrackMessagePublished(domainName, queueName string)

	// TrackMessageConsumed enregistre un message consommé dans les métriques
	TrackMessageConsumed(domainName, queueName string)

	// Méthodes spécialisées pour différents types d'événements
	RecordDomainCreated(name string)
	RecordDomainDeleted(name string)
	RecordQueueCreated(domain, queue string)
	RecordQueueDeleted(domain, queue string)
	RecordRoutingRuleCreated(domain, source, dest string)
	RecordDomainActive(name string, queueCount int)
}
