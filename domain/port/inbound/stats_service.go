package inbound

import (
	"context"
)

// StatsService définit les opérations pour les statistiques du système
type StatsService interface {
	// GetStats récupère les statistiques du système
	GetStats(ctx context.Context) (interface{}, error)

	// TrackMessagePublished enregistre un message publié dans les métriques
	TrackMessagePublished(domainName, queueName string)

	// TrackMessageConsumed enregistre un message consommé dans les métriques
	TrackMessageConsumed(domainName, queueName string)
}
