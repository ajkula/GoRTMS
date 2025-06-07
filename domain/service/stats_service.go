package service

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

const (
	maxPoints     int           = 60 * 60 * 24
	ratesInterval time.Duration = 1 * time.Second
	maxEvents     int           = 50
)

type StatsData struct {
	// global totals
	Domains  int `json:"domains"`
	Queues   int `json:"queues"`
	Messages int `json:"messages"`
	Routes   int `json:"routes"`
	// history bar chart
	MessageRates []MessageRate `json:"messageRates"`
	// dynamicaly calculated carts
	ActiveDomains []map[string]any `json:"activeDomains"`
	TopQueues     []map[string]any `json:"topQueues"`
	QueueAlerts   []map[string]any `json:"queueAlerts"`
	// trends
	DomainTrend  *Trend `json:"domainTrend"`
	QueueTrend   *Trend `json:"queueTrend"`
	MessageTrend *Trend `json:"messageTrend"`
	RouteTrend   *Trend `json:"routeTrend"`
	// events system
	RecentEvents []map[string]any `json:"recentEvents"`
}

type Trend struct {
	Direction string  `json:"direction"` // "up" / "down"
	Value     float64 `json:"value"`     // %
}

type MessageRate struct {
	Timestamp      int64   `json:"timestamp"`
	Rate           float64 `json:"rate"`
	Published      float64 `json:"published"`
	Consumed       float64 `json:"consumed"`
	PublishedTotal int     `json:"publishedTotal"`
	ConsumedTotal  int     `json:"consumedTotal"`
}

type QueueSnapshot struct {
	Domain          string    `json:"domain"`
	Queue           string    `json:"queue"`
	BufferSize      int       `json:"bufferSize"`
	BufferCapacity  int       `json:"bufferCapacity"`
	BufferUsage     float64   `json:"bufferUsage"`
	RepositoryCount int       `json:"repositoryCount"`
	LastUpdated     time.Time `json:"lastUpdated"`

	// Alert state
	AlertLevel string    `json:"alertLevel,omitempty"` // "", "warning", "critical"
	AlertSince time.Time `json:"alertSince,omitempty"`
	AlertID    string    `json:"alertId,omitempty"`
}

type MetricsStore struct {
	// History of message rates
	messageRates   []MessageRate
	queueSnapshots map[string]*QueueSnapshot // "domain:queue" -> snapshot

	// Previous state to calculate trends
	previousStats *StatsData

	// Timestamp of the last collection
	lastCollected time.Time

	// Recent system events
	systemEvents []model.SystemEvent

	// Root context
	rootCtx context.Context

	logger outbound.Logger

	// Mutex for concurrent access
	mu sync.RWMutex
}

type StatsServiceImpl struct {
	domainRepo                   outbound.DomainRepository
	messageRepo                  outbound.MessageRepository
	metrics                      *MetricsStore
	publishCountSinceLastCollect int
	consumeCountSinceLastCollect int
	countMu                      sync.Mutex

	// Metrics collection interval
	collectInterval time.Duration

	// Channel to stop automatic collection
	stopCollect chan struct{}
}

func NewStatsService(
	rootCtx context.Context,
	logger outbound.Logger,
	domainRepo outbound.DomainRepository,
	messageRepo outbound.MessageRepository,
) inbound.StatsService {
	metrics := &MetricsStore{
		rootCtx:        rootCtx,
		logger:         logger,
		messageRates:   make([]MessageRate, 0, maxPoints),
		queueSnapshots: make(map[string]*QueueSnapshot),
		lastCollected:  time.Now(),
		systemEvents:   make([]model.SystemEvent, 0),
	}

	service := &StatsServiceImpl{
		domainRepo:      domainRepo,
		messageRepo:     messageRepo,
		metrics:         metrics,
		collectInterval: ratesInterval,
		stopCollect:     make(chan struct{}),
	}

	go service.startMetricsCollection()

	return service
}

func (s *StatsServiceImpl) TrackMessagePublished(domainName, queueName string) {
	s.countMu.Lock()
	defer s.countMu.Unlock()
	s.publishCountSinceLastCollect++
}

func (s *StatsServiceImpl) TrackMessageConsumed(domainName, queueName string) {
	s.countMu.Lock()
	defer s.countMu.Unlock()
	s.consumeCountSinceLastCollect++
}

func (s *StatsServiceImpl) startMetricsCollection() {
	ticker := time.NewTicker(s.collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.collectMetrics()
		case <-s.stopCollect:
			return
		}
	}
}

func (s *StatsServiceImpl) collectMetrics() {
	s.metrics.mu.Lock()

	now := time.Now()
	elapsed := now.Sub(s.metrics.lastCollected).Seconds()

	publishRate := float64(s.publishCountSinceLastCollect) / elapsed
	consumeRate := float64(s.consumeCountSinceLastCollect) / elapsed

	s.metrics.messageRates = append(s.metrics.messageRates, MessageRate{
		Timestamp:      now.Unix(),
		Rate:           publishRate + consumeRate,
		Published:      publishRate,
		Consumed:       consumeRate,
		PublishedTotal: s.publishCountSinceLastCollect,
		ConsumedTotal:  s.consumeCountSinceLastCollect,
	})

	// Limit history size
	if len(s.metrics.messageRates) > maxPoints {
		s.metrics.messageRates = s.metrics.messageRates[len(s.metrics.messageRates)-maxPoints:]
	}

	s.publishCountSinceLastCollect = 0
	s.consumeCountSinceLastCollect = 0

	s.metrics.lastCollected = now

	s.metrics.mu.Unlock()

	s.updateQueueSnapshots()
}

func (s *StatsServiceImpl) RecordEvent(eventType, eventSeverity, resource string, data any) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	now := time.Now()

	switch eventType {
	case "domain_active":
		// For domain_active, replace the existing event instead of adding a new one
		for i, evt := range s.metrics.systemEvents {
			if evt.EventType == eventType && evt.Resource == resource {
				oldCount, newCount := 0, 0

				// Extract the old counter
				if oldData, ok := evt.Data.(map[string]any); ok {
					if count, ok := oldData["queueCount"]; ok {
						oldCount, _ = count.(int)
					}
				}

				// Extract the new counter
				if newData, ok := data.(map[string]any); ok {
					if count, ok := newData["queueCount"]; ok {
						newCount, _ = count.(int)
					}
				}

				// Update the event and refresh the timestamp
				// ONLY if the data has changed
				if oldCount != newCount {
					// Generate a new ID so the frontend detects it as a new event
					newID := fmt.Sprintf("event-%d-%d", now.UnixNano(), rand.Intn(10000))
					s.metrics.systemEvents[i].ID = newID
					s.metrics.systemEvents[i].Data = data
					s.metrics.systemEvents[i].Timestamp = now
					s.metrics.systemEvents[i].UnixTime = now.Unix()
				}
				return // nothing new GTFO
			}
		}
		// If we reach this point, it means no existing event was found

	case "queue_capacity":
		// Similar logic for queue_capacity - keep only the most recent event per resource
		for i, evt := range s.metrics.systemEvents {
			if evt.EventType == eventType && evt.Resource == resource {
				// Replace only if the new event is more critical
				oldSeverity := evt.Type
				oldData := evt.Data
				hasChanged := (eventSeverity != oldSeverity || data != oldData)

				if hasChanged {
					// Real change → update timestamp
					s.metrics.systemEvents[i].Data = data
					s.metrics.systemEvents[i].Type = eventSeverity
					s.metrics.systemEvents[i].Timestamp = now
					s.metrics.systemEvents[i].UnixTime = now.Unix()
				}
				// No change → keep existing timestamp
				return
			}
		}
	}

	// new evt => new ID
	id := fmt.Sprintf("event-%d-%d", now.UnixNano(), rand.Intn(10000))

	event := model.SystemEvent{
		ID:        id,
		Type:      eventSeverity,
		EventType: eventType,
		Resource:  resource,
		Data:      data,
		Timestamp: now,
		UnixTime:  now.Unix(),
	}

	s.metrics.systemEvents = append(s.metrics.systemEvents, event)

	// limit
	if len(s.metrics.systemEvents) > maxEvents {
		s.metrics.systemEvents = s.metrics.systemEvents[len(s.metrics.systemEvents)-50:]
	}
}

func (s *StatsServiceImpl) RecordDomainActive(name string, queueCount int) {
	s.RecordEvent("domain_active", "info", name, map[string]any{
		"queueCount": queueCount,
	})
}

func (s *StatsServiceImpl) RecordDomainCreated(name string) {
	s.RecordEvent("domain_created", "info", name, nil)
}

func (s *StatsServiceImpl) RecordDomainDeleted(name string) {
	s.RecordEvent("domain_deleted", "info", name, nil)
}

func (s *StatsServiceImpl) RecordQueueCreated(domain, queue string) {
	resource := fmt.Sprintf("%s.%s", domain, queue)
	s.RecordEvent("queue_created", "info", resource, nil)
}

func (s *StatsServiceImpl) RecordQueueDeleted(domain, queue string) {
	resource := fmt.Sprintf("%s.%s", domain, queue)
	s.RecordEvent("queue_deleted", "info", resource, nil)
}

func (s *StatsServiceImpl) RecordRoutingRuleCreated(domain, source, dest string) {
	s.RecordEvent("routing_rule_created", "info", domain, map[string]string{
		"source":      source,
		"destination": dest,
	})
}

func (s *StatsServiceImpl) RecordQueueCapacity(domain, queue string, usage float64) {
	resource := fmt.Sprintf("%s.%s", domain, queue)
	severity := "warning"
	if usage >= 90 {
		severity = "error"
	}
	s.RecordEvent("queue_capacity", severity, resource, usage)
}

func (s *StatsServiceImpl) RecordConnectionLost(domain, queue, consumerId string) {
	resource := fmt.Sprintf("%s.%s", domain, queue)
	s.RecordEvent("connection_lost", "error", resource, map[string]string{
		"consumerId": consumerId,
	})
}

func (s *StatsServiceImpl) updateQueueSnapshots() {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	ctx, cancel := context.WithTimeout(s.metrics.rootCtx, 5*time.Second)
	defer cancel()

	domains, err := s.domainRepo.ListDomains(ctx)
	if err != nil {
		s.metrics.logger.Error("Error fetching domains for snapshots", "ERROR", err)
		return
	}

	now := time.Now()

	// mark all snapshots as "viewed"
	seen := make(map[string]bool)

	// TODO: use queueService to access ChannelQueues (if required)
	for _, domain := range domains {
		for queueName, queue := range domain.Queues {
			key := fmt.Sprintf("%s:%s", domain.Name, queueName)
			seen[key] = true

			// buffer config
			bufferCapacity := queue.Config.MaxSize
			if bufferCapacity <= 0 {
				bufferCapacity = 1000
			}

			// Stats
			repoCount := s.messageRepo.GetQueueMessageCount(domain.Name, queueName)
			bufferSize := repoCount // TODO: remplace with GetBufferStats()
			usage := float64(bufferSize) / float64(bufferCapacity) * 100

			// get/create snapshot
			snapshot, exists := s.metrics.queueSnapshots[key]
			if !exists {
				snapshot = &QueueSnapshot{
					Domain: domain.Name,
					Queue:  queueName,
				}
				s.metrics.queueSnapshots[key] = snapshot
			}

			snapshot.BufferSize = bufferSize
			snapshot.BufferCapacity = bufferCapacity
			snapshot.BufferUsage = usage
			snapshot.RepositoryCount = repoCount
			snapshot.LastUpdated = now

			// Alerts management
			previousLevel := snapshot.AlertLevel
			newLevel := ""

			if usage >= 90 {
				newLevel = "critical"
			} else if usage >= 75 {
				newLevel = "warning"
			}

			// if state change
			if newLevel != previousLevel {
				if newLevel != "" {
					// new alert
					snapshot.AlertLevel = newLevel
					snapshot.AlertSince = now
					snapshot.AlertID = fmt.Sprintf("alert-%s-%d", key, now.UnixNano())

					s.RecordQueueCapacity(domain.Name, queueName, usage)
				} else {
					// Alert resolved
					snapshot.AlertLevel = ""
					snapshot.AlertSince = time.Time{}
					snapshot.AlertID = ""
				}
			}
		}
	}

	// clean obsolete
	for key := range s.metrics.queueSnapshots {
		if !seen[key] {
			delete(s.metrics.queueSnapshots, key)
		}
	}
}

func (s *StatsServiceImpl) GetStatsWithAggregation(ctx context.Context, period, granularity string) (any, error) {
	fullStats, err := s.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// Type assertion
	stats, ok := fullStats.(*StatsData)
	if !ok {
		return nil, fmt.Errorf("unexpected stats type")
	}

	// Create a COPY
	clientStats := &StatsData{
		Domains:       stats.Domains,
		Queues:        stats.Queues,
		Messages:      stats.Messages,
		Routes:        stats.Routes,
		ActiveDomains: stats.ActiveDomains,
		TopQueues:     stats.TopQueues,
		QueueAlerts:   stats.QueueAlerts,
		DomainTrend:   stats.DomainTrend,
		QueueTrend:    stats.QueueTrend,
		MessageTrend:  stats.MessageTrend,
		RouteTrend:    stats.RouteTrend,
		RecentEvents:  stats.RecentEvents,
	}

	aggregatedRates := s.getAggregatedMessageRates(period, granularity)

	clientStats.MessageRates = aggregatedRates

	return clientStats, nil
}

// returns message rates aggregated by period and granularity
func (s *StatsServiceImpl) getAggregatedMessageRates(period, granularity string) []MessageRate {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	if len(s.metrics.messageRates) == 0 {
		return []MessageRate{}
	}

	// Determine time range
	now := time.Now()
	var startTime time.Time

	switch period {
	case "1h":
		startTime = now.Add(-1 * time.Hour)
	case "6h":
		startTime = now.Add(-6 * time.Hour)
	case "12h":
		startTime = now.Add(-12 * time.Hour)
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	default:
		startTime = now.Add(-1 * time.Hour) // Default to 1h
	}

	// Determine granularity in seconds
	granularitySeconds := s.determineGranularity(period, granularity)

	// Filter and aggregate
	return s.aggregateMessageRates(startTime, granularitySeconds)
}

// determineGranularity returns the granularity in seconds
func (s *StatsServiceImpl) determineGranularity(period, granularity string) int {
	// If auto, determine based on period
	if granularity == "auto" {
		switch period {
		case "1h":
			return 60 // 1 minute
		case "6h":
			return 300 // 5 minutes
		case "12h":
			return 900 // 15 minutes
		case "24h":
			return 1800 // 30 minutes
		default:
			return 60 // Default: 1 minute
		}
	}

	// Parse explicit granularity
	switch granularity {
	case "10s":
		return 10
	case "1m":
		return 60
	case "5m":
		return 300
	case "15m":
		return 900
	case "30m":
		return 1800
	case "1h":
		return 3600
	default:
		return 60 // Default: 1 minute
	}
}

// aggregateMessageRates performs the actual aggregation
func (s *StatsServiceImpl) aggregateMessageRates(startTime time.Time, granularitySeconds int) []MessageRate {
	aggregated := make([]MessageRate, 0)

	// Create time buckets
	buckets := make(map[int64]*MessageRate)

	for _, rate := range s.metrics.messageRates {
		// Skip if before start time
		if rate.Timestamp < startTime.Unix() {
			continue
		}

		// Determine bucket
		bucketTime := (rate.Timestamp / int64(granularitySeconds)) * int64(granularitySeconds)

		bucket, exists := buckets[bucketTime]
		if !exists {
			bucket = &MessageRate{
				Timestamp:      bucketTime,
				PublishedTotal: 0,
				ConsumedTotal:  0,
			}
			buckets[bucketTime] = bucket
		}

		// Aggregate counts
		bucket.PublishedTotal += rate.PublishedTotal
		bucket.ConsumedTotal += rate.ConsumedTotal
	}

	// Convert to slice and calculate rates
	for _, bucket := range buckets {
		// Calculate rate per second for the period
		bucket.Published = float64(bucket.PublishedTotal) / float64(granularitySeconds)
		bucket.Consumed = float64(bucket.ConsumedTotal) / float64(granularitySeconds)
		bucket.Rate = bucket.Published + bucket.Consumed

		aggregated = append(aggregated, *bucket)
	}

	// Sort by timestamp
	sort.Slice(aggregated, func(i, j int) bool {
		return aggregated[i].Timestamp < aggregated[j].Timestamp
	})

	s.metrics.logger.Debug("Aggregated message rates",
		"original_points", len(s.metrics.messageRates),
		"aggregated_points", len(aggregated),
		"period", startTime.Format("15:04:05"),
		"granularity_seconds", granularitySeconds)

	return aggregated
}

func (s *StatsServiceImpl) GetStats(ctx context.Context) (any, error) {
	s.metrics.logger.Info("Getting system statistics")

	domains, err := s.domainRepo.ListDomains(ctx)
	if err != nil {
		return nil, err
	}

	s.metrics.mu.RLock()
	previousStats := s.metrics.previousStats
	s.metrics.mu.RUnlock()

	stats := &StatsData{
		Domains:       len(domains),
		Queues:        0,
		Messages:      0,
		Routes:        0,
		ActiveDomains: make([]map[string]any, 0),
		TopQueues:     make([]map[string]any, 0),
		QueueAlerts:   make([]map[string]any, 0),
	}

	s.metrics.mu.RLock()
	stats.MessageRates = make([]MessageRate, len(s.metrics.messageRates))
	copy(stats.MessageRates, s.metrics.messageRates)

	domainAggregates := make(map[string]struct {
		MessageCount int
		QueueCount   int
	})

	queueDataList := make([]map[string]any, 0, len(s.metrics.queueSnapshots))

	for _, snapshot := range s.metrics.queueSnapshots {
		// by domain
		agg := domainAggregates[snapshot.Domain]
		agg.MessageCount += snapshot.RepositoryCount
		agg.QueueCount++
		domainAggregates[snapshot.Domain] = agg

		queueData := map[string]any{
			"domain":       snapshot.Domain,
			"name":         snapshot.Queue,
			"messageCount": snapshot.BufferSize,
			"maxSize":      snapshot.BufferCapacity,
			"usage":        snapshot.BufferUsage,
		}
		queueDataList = append(queueDataList, queueData)

		if snapshot.AlertLevel != "" {
			stats.QueueAlerts = append(stats.QueueAlerts, map[string]any{
				"domain":     snapshot.Domain,
				"queue":      snapshot.Queue,
				"usage":      snapshot.BufferUsage,
				"severity":   snapshot.AlertLevel,
				"detectedAt": snapshot.AlertSince.Unix(),
			})
		}
	}
	s.metrics.mu.RUnlock()

	// ActiveDomains for DomainPieChart
	for domainName, agg := range domainAggregates {
		stats.ActiveDomains = append(stats.ActiveDomains, map[string]any{
			"name":         domainName,
			"messageCount": agg.MessageCount,
			"queueCount":   agg.QueueCount,
			"messageRate":  calculateDomainMessageRate(domainName, stats.MessageRates),
		})
		stats.Queues += agg.QueueCount
		stats.Messages += agg.MessageCount
	}

	for _, domain := range domains {
		for _, routes := range domain.Routes {
			stats.Routes += len(routes)
		}
	}

	// TopQueues sort/limit
	sort.Slice(queueDataList, func(i, j int) bool {
		return queueDataList[i]["usage"].(float64) > queueDataList[j]["usage"].(float64)
	})

	if len(queueDataList) > 5 {
		stats.TopQueues = queueDataList[:5]
	} else {
		stats.TopQueues = queueDataList
	}

	if previousStats != nil {
		stats.DomainTrend = calculateTrend(previousStats.Domains, stats.Domains)
		stats.QueueTrend = calculateTrend(previousStats.Queues, stats.Queues)
		stats.MessageTrend = calculateTrend(previousStats.Messages, stats.Messages)
		stats.RouteTrend = calculateTrend(previousStats.Routes, stats.Routes)
	}

	s.metrics.mu.Lock()
	s.metrics.previousStats = stats
	s.metrics.mu.Unlock()

	s.metrics.mu.RLock()
	events := make([]map[string]any, 0, len(s.metrics.systemEvents))
	for _, event := range s.metrics.systemEvents {
		eventMap := map[string]any{
			"id":        event.ID,
			"type":      event.Type,
			"eventType": event.EventType,
			"resource":  event.Resource,
			"timestamp": event.UnixTime,
		}

		if event.Data != nil {
			eventMap["data"] = event.Data
		}

		events = append(events, eventMap)
	}
	s.metrics.mu.RUnlock()

	// recent first
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	stats.RecentEvents = events

	return stats, nil
}

func calculateDomainMessageRate(domainName string, rates []MessageRate) float64 {
	if len(rates) == 0 {
		return 0
	}

	// For now, simply return the latest global rate
	// we could provide rates per domain
	return rates[len(rates)-1].Rate
}

// calculateTrend computes the trend between two values
func calculateTrend(previous, current int) *Trend {
	if previous == 0 {
		return nil
	}

	direction := "up"
	if current < previous {
		direction = "down"
	}

	change := float64(current-previous) / float64(abs(previous)) * 100
	if change < 0 {
		change = -change
	}

	return &Trend{
		Direction: direction,
		Value:     change,
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (s *StatsServiceImpl) Cleanup() {
	s.metrics.logger.Info("Stats service cleanup starting")

	// Signal the stop of metrics collection
	// and wait for it to finish
	close(s.stopCollect)

	// Use a timeout to avoid blocking indefinitely
	cleanupDone := make(chan struct{})
	go func() {
		// Clean up resources safely
		s.metrics.mu.Lock()
		s.metrics.messageRates = nil
		s.metrics.systemEvents = nil
		s.metrics.previousStats = nil
		s.metrics.queueSnapshots = nil
		s.metrics.mu.Unlock()

		close(cleanupDone)
	}()

	// wait with timeout
	select {
	case <-cleanupDone:
		s.metrics.logger.Info("Stats service resources cleaned up")
	case <-time.After(5 * time.Second):
		s.metrics.logger.Info("Stats service cleanup timed out, forcing shutdown")
	}

	s.metrics.logger.Info("Stats service cleanup complete")
}
