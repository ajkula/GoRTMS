package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

const (
	maxPoints int = 60
	maxEvents int = 50
)

type StatsData struct {
	Domains       int              `json:"domains"`
	Queues        int              `json:"queues"`
	Messages      int              `json:"messages"`
	Routes        int              `json:"routes"`
	MessageRates  []MessageRate    `json:"messageRates"`
	ActiveDomains []DomainStats    `json:"activeDomains"`
	QueueAlerts   []QueueAlert     `json:"queueAlerts"`
	DomainTrend   *Trend           `json:"domainTrend"`
	QueueTrend    *Trend           `json:"queueTrend"`
	MessageTrend  *Trend           `json:"messageTrend"`
	RouteTrend    *Trend           `json:"routeTrend"`
	TopQueues     []QueueStats     `json:"topQueues"`
	PublishCounts map[string]int   `json:"publishCounts"`
	ConsumeCounts map[string]int   `json:"consumeCounts"`
	RecentEvents  []map[string]any `json:"recentEvents"`
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

type DomainStats struct {
	Name         string  `json:"name"`
	QueueCount   int     `json:"queueCount"`
	MessageCount int     `json:"messageCount"`
	MessageRate  float64 `json:"messageRate"`
}

type QueueStats struct {
	Domain       string  `json:"domain"`
	Name         string  `json:"name"`
	MessageCount int     `json:"messageCount"`
	MaxSize      int     `json:"maxSize"`
	Usage        float64 `json:"usage"` // %
}

type QueueAlert struct {
	Domain     string  `json:"domain"`
	Queue      string  `json:"queue"`
	Usage      float64 `json:"usage"`    // %
	Severity   string  `json:"severity"` // "warning", "critical"
	DetectedAt int64   `json:"detectedAt"`
}

type MetricsStore struct {
	// History of message rates
	messageRates []MessageRate

	// Counters of published messages per queue
	publishCounters map[string]map[string]int // domainName -> queueName -> count

	// Counters of consumed messages per queue
	consumeCounters map[string]map[string]int // domainName -> queueName -> count

	// Active alerts on queues
	queueAlerts map[string]map[string]QueueAlert // domainName -> queueName -> alert

	// Previous state to calculate trends
	previousStats *StatsData

	// Timestamp of the last collection
	lastCollected time.Time

	// Recent system events
	systemEvents []model.SystemEvent

	// Root context
	rootCtx context.Context

	// Mutex for concurrent access
	mu sync.RWMutex
}

type StatsServiceImpl struct {
	domainRepo  outbound.DomainRepository
	messageRepo outbound.MessageRepository
	metrics     *MetricsStore

	// Metrics collection interval
	collectInterval time.Duration

	// Channel to stop automatic collection
	stopCollect chan struct{}
}

func NewStatsService(
	domainRepo outbound.DomainRepository,
	messageRepo outbound.MessageRepository,
	rootCtx context.Context,
) inbound.StatsService {
	metrics := &MetricsStore{
		messageRates:    make([]MessageRate, 0, maxPoints),
		publishCounters: make(map[string]map[string]int),
		consumeCounters: make(map[string]map[string]int),
		queueAlerts:     make(map[string]map[string]QueueAlert),
		lastCollected:   time.Now(),
		rootCtx:         rootCtx,
	}

	service := &StatsServiceImpl{
		domainRepo:      domainRepo,
		messageRepo:     messageRepo,
		metrics:         metrics,
		collectInterval: 1 * time.Minute,
		stopCollect:     make(chan struct{}),
	}

	go service.startMetricsCollection()

	return service
}

func (s *StatsServiceImpl) TrackMessagePublished(domainName, queueName string) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	// Initialize the map if necessary
	if _, exists := s.metrics.publishCounters[domainName]; !exists {
		s.metrics.publishCounters[domainName] = make(map[string]int)
	}

	// Increment the counter
	s.metrics.publishCounters[domainName][queueName]++
}

func (s *StatsServiceImpl) TrackMessageConsumed(domainName, queueName string) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	// Initialize the map if necessary
	if _, exists := s.metrics.consumeCounters[domainName]; !exists {
		s.metrics.consumeCounters[domainName] = make(map[string]int)
	}

	// Increment the counter
	s.metrics.consumeCounters[domainName][queueName]++
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

	// Agregate counters
	totalPublished := 0
	totalConsumed := 0

	for _, domainCounters := range s.metrics.publishCounters {
		for _, count := range domainCounters {
			totalPublished += count
		}
	}

	for _, domainCounters := range s.metrics.consumeCounters {
		for _, count := range domainCounters {
			totalConsumed += count
		}
	}

	// Calculate per-second rates
	publishRate := float64(totalPublished) / elapsed
	consumeRate := float64(totalConsumed) / elapsed
	totalRate := publishRate + consumeRate

	// Append to the rates array
	s.metrics.messageRates = append(s.metrics.messageRates, MessageRate{
		Timestamp:      now.Unix(),
		Rate:           totalRate,
		Published:      publishRate,
		Consumed:       consumeRate,
		PublishedTotal: totalPublished,
		ConsumedTotal:  totalConsumed,
	})

	// Limit the size of the history to = maxPoints
	if len(s.metrics.messageRates) > maxPoints {
		s.metrics.messageRates = s.metrics.messageRates[len(s.metrics.messageRates)-60:]
	}

	// reset counters
	s.metrics.publishCounters = make(map[string]map[string]int)
	s.metrics.consumeCounters = make(map[string]map[string]int)

	// update timestamp
	s.metrics.lastCollected = now

	// IMPORTANT!
	s.metrics.mu.Unlock()

	// Check for full queue alerts in a separate goroutine
	go s.checkQueueAlerts()
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
				// or if the old one is too old (> 5 minutes)
				oldSeverity := evt.Type
				timeDiff := now.Unix() - evt.UnixTime

				if eventSeverity == "warning" || oldSeverity != "warning" || timeDiff > 300 {
					s.metrics.systemEvents[i].Data = data
					s.metrics.systemEvents[i].Type = eventSeverity
					s.metrics.systemEvents[i].Timestamp = now
					s.metrics.systemEvents[i].UnixTime = now.Unix()
				}
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

func (s *StatsServiceImpl) checkQueueAlerts() {
	domains, err := s.domainRepo.ListDomains(s.metrics.rootCtx)
	if err != nil {
		log.Printf("Error fetching domains for alerts: %v", err)
		return
	}

	// Prepare a list of alerts to be recorded later
	var alertsToRecord []struct {
		domain string
		queue  string
		usage  float64
	}

	s.metrics.mu.Lock()

	// Reset alerts after 1 hour
	expireBefore := time.Now().Add(-1 * time.Hour).Unix()

	// Check each domain
	for _, domain := range domains {
		// Initialize the map if necessary
		if _, exists := s.metrics.queueAlerts[domain.Name]; !exists {
			s.metrics.queueAlerts[domain.Name] = make(map[string]QueueAlert)
		}

		// Iterate over the queues
		for queueName, queue := range domain.Queues {
			// Check if maxSize is defined and > 0
			if queue.Config.MaxSize > 0 {
				usage := float64(queue.MessageCount) / float64(queue.Config.MaxSize) * 100

				// Check if usage exceeds thresholds
				if usage >= 90 {
					// Critical alert
					s.metrics.queueAlerts[domain.Name][queueName] = QueueAlert{
						Domain:     domain.Name,
						Queue:      queueName,
						Usage:      usage,
						Severity:   "critical",
						DetectedAt: time.Now().Unix(),
					}

					// Add to the list of alerts to be recorded later
					alertsToRecord = append(alertsToRecord, struct {
						domain string
						queue  string
						usage  float64
					}{domain.Name, queueName, usage})

				} else if usage >= 75 {
					// Warning alert
					s.metrics.queueAlerts[domain.Name][queueName] = QueueAlert{
						Domain:     domain.Name,
						Queue:      queueName,
						Usage:      usage,
						Severity:   "warning",
						DetectedAt: time.Now().Unix(),
					}

					// Add to the list of alerts to be recorded later
					alertsToRecord = append(alertsToRecord, struct {
						domain string
						queue  string
						usage  float64
					}{domain.Name, queueName, usage})

				} else {
					// Remove the existing alert if usage has returned to normal
					delete(s.metrics.queueAlerts[domain.Name], queueName)
				}
			}
		}

		// Remove expired alerts
		for queueName, alert := range s.metrics.queueAlerts[domain.Name] {
			if alert.DetectedAt < expireBefore {
				delete(s.metrics.queueAlerts[domain.Name], queueName)
			}
		}

		// Delete the map if it's empty
		if len(s.metrics.queueAlerts[domain.Name]) == 0 {
			delete(s.metrics.queueAlerts, domain.Name)
		}
	}

	// Unlock the mutex before recording the events
	s.metrics.mu.Unlock()

	// Now record the events without holding the mutex
	for _, alert := range alertsToRecord {
		s.RecordQueueCapacity(alert.domain, alert.queue, alert.usage)
	}
}

func (s *StatsServiceImpl) GetStats(ctx context.Context) (any, error) {
	log.Println("Getting system statistics")

	domains, err := s.domainRepo.ListDomains(ctx)
	if err != nil {
		return nil, err
	}

	// Retrieve the previous state to calculate trends
	var previousStats *StatsData
	s.metrics.mu.RLock()
	previousStats = s.metrics.previousStats
	s.metrics.mu.RUnlock()

	stats := &StatsData{
		Domains:       len(domains),
		Queues:        0,
		Messages:      0,
		Routes:        0,
		ActiveDomains: make([]DomainStats, 0, len(domains)),
		TopQueues:     make([]QueueStats, 0, 5),
		PublishCounts: make(map[string]int),
		ConsumeCounts: make(map[string]int),
	}

	// Retrieve message rates
	s.metrics.mu.RLock()
	stats.MessageRates = make([]MessageRate, len(s.metrics.messageRates))
	copy(stats.MessageRates, s.metrics.messageRates)

	// Retrieve alerts
	queueAlerts := []QueueAlert{}
	for _, domainAlerts := range s.metrics.queueAlerts {
		for _, alert := range domainAlerts {
			queueAlerts = append(queueAlerts, alert)
		}
	}
	stats.QueueAlerts = queueAlerts
	s.metrics.mu.RUnlock()

	// Compute statistics per domain
	allQueues := make([]QueueStats, 0)

	for _, domain := range domains {
		queueCount := len(domain.Queues)
		messageCount := 0
		routeCount := 0

		// Count the number of messages and rules
		for queueName, queue := range domain.Queues {
			// Count the number of messages in the repository
			queueMessageCount := s.messageRepo.GetQueueMessageCount(domain.Name, queueName)

			messageCount += queueMessageCount

			// Add to the list of all queues to sort the top queues
			maxSize := queue.Config.MaxSize
			if maxSize <= 0 {
				maxSize = 1000
			}

			usage := float64(queueMessageCount) / float64(maxSize) * 100
			allQueues = append(allQueues, QueueStats{
				Domain:       domain.Name,
				Name:         queueName,
				MessageCount: queueMessageCount,
				MaxSize:      queue.Config.MaxSize,
				Usage:        usage,
			})

			// Count the routing rules
			if routes, exists := domain.Routes[queueName]; exists {
				routeCount += len(routes)
			}
		}

		// Add to the totals
		stats.Queues += queueCount
		stats.Messages += messageCount
		stats.Routes += routeCount

		// Add to the domain stats
		if queueCount > 0 {
			stats.ActiveDomains = append(stats.ActiveDomains, DomainStats{
				Name:         domain.Name,
				QueueCount:   queueCount,
				MessageCount: messageCount,
				MessageRate:  calculateDomainMessageRate(domain.Name, stats.MessageRates),
			})
		}
	}

	// Sort and limit the top queues
	sortQueuesByUsage(allQueues)
	if len(allQueues) > 5 {
		stats.TopQueues = allQueues[:5]
	} else {
		stats.TopQueues = allQueues
	}

	// Compute trends if previous stats are available
	if previousStats != nil {
		stats.DomainTrend = calculateTrend(previousStats.Domains, stats.Domains)
		stats.QueueTrend = calculateTrend(previousStats.Queues, stats.Queues)
		stats.MessageTrend = calculateTrend(previousStats.Messages, stats.Messages)
		stats.RouteTrend = calculateTrend(previousStats.Routes, stats.Routes)
	}

	// Store the current statistics for next time
	s.metrics.mu.Lock()
	s.metrics.previousStats = stats
	s.metrics.mu.Unlock()

	// Retrieve and format recent system events
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

	// Reverse the order to have the most recent ones first
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
	// In a full implementation, we could provide rates per domain
	return rates[len(rates)-1].Rate
}

// calculateTrend computes the trend between two values
func calculateTrend(previous, current int) *Trend {
	if previous == 0 {
		return nil
	}

	change := float64(current-previous) / float64(previous) * 100
	direction := "up"
	if change < 0 {
		direction = "down"
		change = -change
	}

	return &Trend{
		Direction: direction,
		Value:     change,
	}
}

func sortQueuesByUsage(queues []QueueStats) {
	// Simple sort by decreasing usage
	for i := 0; i < len(queues); i++ {
		for j := i + 1; j < len(queues); j++ {
			if queues[j].Usage > queues[i].Usage {
				queues[i], queues[j] = queues[j], queues[i]
			}
		}
	}
}

func (s *StatsServiceImpl) Cleanup() {
	log.Println("Stats service cleanup starting")

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
		s.metrics.publishCounters = nil
		s.metrics.consumeCounters = nil
		s.metrics.queueAlerts = nil
		s.metrics.mu.Unlock()

		close(cleanupDone)
	}()

	// wait with timeout
	select {
	case <-cleanupDone:
		log.Println("Stats service resources cleaned up")
	case <-time.After(5 * time.Second):
		log.Println("Stats service cleanup timed out, forcing shutdown")
	}

	log.Println("Stats service cleanup complete")
}
