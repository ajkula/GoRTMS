package inbound

import (
	"context"
)

// StatsService définit l'interface pour le service de statistiques
type StatsService interface {
	// GetStats récupère les statistiques du système
	GetStats(ctx context.Context) (interface{}, error)
}
