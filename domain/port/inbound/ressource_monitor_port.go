package inbound

import (
	"context"
)

// ResourceStats contient les statistiques d'utilisation des ressources
type ResourceStats struct {
	Timestamp   int64  `json:"timestamp"`
	MemoryUsage int64  `json:"memoryUsage"` // en octets
	Goroutines  int    `json:"goroutines"`
	GCCycles    uint32 `json:"gcCycles"`
	GCPauseNs   int64  `json:"gcPauseNs"`
	HeapObjects uint64 `json:"heapObjects"`
	// Par domaine et queue
	DomainStats map[string]DomainResourceInfo `json:"domainStats"`
}

// DomainResourceInfo contient les statistiques par domaine
type DomainResourceInfo struct {
	QueueCount      int                          `json:"queueCount"`
	MessageCount    int                          `json:"messageCount"`
	QueueStats      map[string]QueueResourceInfo `json:"queueStats"`
	EstimatedMemory int64                        `json:"estimatedMemory"` // estimation grossière
}

// QueueResourceInfo contient les statistiques par queue
type QueueResourceInfo struct {
	MessageCount    int   `json:"messageCount"`
	BufferSize      int   `json:"bufferSize"`
	EstimatedMemory int64 `json:"estimatedMemory"` // estimation grossière
}

// ResourceMonitorService définit l'interface pour le service de monitoring des ressources
type ResourceMonitorService interface {
	// GetCurrentStats récupère les statistiques actuelles d'utilisation des ressources
	GetCurrentStats(ctx context.Context) (*ResourceStats, error)

	// GetStatsHistory récupère l'historique des statistiques d'utilisation des ressources
	GetStatsHistory(ctx context.Context, limit int) ([]*ResourceStats, error)

	// Cleanup libère les ressources utilisées par le service
	Cleanup()
}
