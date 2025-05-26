package inbound

import (
	"context"
)

// ResourceStats holds resource usage statistics
type ResourceStats struct {
	Timestamp   int64  `json:"timestamp"`
	MemoryUsage int64  `json:"memoryUsage"` // in bytes
	Goroutines  int    `json:"goroutines"`
	GCCycles    uint32 `json:"gcCycles"`
	GCPauseNs   int64  `json:"gcPauseNs"`
	HeapObjects uint64 `json:"heapObjects"`
	// Per domain and queue
	DomainStats map[string]DomainResourceInfo `json:"domainStats"`
}

// DomainResourceInfo holds stats per domain
type DomainResourceInfo struct {
	QueueCount      int                          `json:"queueCount"`
	MessageCount    int                          `json:"messageCount"`
	QueueStats      map[string]QueueResourceInfo `json:"queueStats"`
	EstimatedMemory int64                        `json:"estimatedMemory"` // rough estimate
}

// QueueResourceInfo holds stats per queue
type QueueResourceInfo struct {
	MessageCount    int   `json:"messageCount"`
	BufferSize      int   `json:"bufferSize"`
	EstimatedMemory int64 `json:"estimatedMemory"` // rough estimate
}

// ResourceMonitorService defines the interface for resource monitoring
type ResourceMonitorService interface {
	// GetCurrentStats retrieves current resource usage statistics
	GetCurrentStats(ctx context.Context) (*ResourceStats, error)

	// GetStatsHistory retrieves the resource usage history
	GetStatsHistory(ctx context.Context, limit int) ([]*ResourceStats, error)

	// Cleanup frees resources used by the service
	Cleanup()
}
