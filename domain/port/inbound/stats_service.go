package inbound

import (
	"context"
)

// StatsService defines operations for system statistics
type StatsService interface {
	// GetStats returns system statistics
	GetStats(ctx context.Context) (any, error)

	// TrackMessagePublished records a published message in metrics
	TrackMessagePublished(domainName, queueName string)

	// TrackMessageConsumed records a consumed message in metrics
	TrackMessageConsumed(domainName, queueName string)

	// GetStatsWithAggregation returns stats with time-based aggregation
	GetStatsWithAggregation(ctx context.Context, period, granularity string) (any, error)

	// Specialized methods for different event types
	RecordDomainCreated(name string)
	RecordDomainDeleted(name string)
	RecordQueueCreated(domain, queue string)
	RecordQueueDeleted(domain, queue string)
	RecordRoutingRuleCreated(domain, source, dest string)
	RecordDomainActive(name string, queueCount int)
}
