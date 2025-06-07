package service

import (
	"context"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to setup MetricsStore with test data
func setupMetricsStore(logger outbound.Logger) *MetricsStore {
	return &MetricsStore{
		rootCtx:        context.Background(),
		logger:         logger,
		messageRates:   make([]MessageRate, 0),
		queueSnapshots: make(map[string]*QueueSnapshot),
		lastCollected:  time.Now(),
		systemEvents:   make([]model.SystemEvent, 0),
	}
}

// Helper to create test domain with queues
func createTestDomain(name string, queueConfigs map[string]int) *model.Domain {
	domain := &model.Domain{
		Name:   name,
		Queues: make(map[string]*model.Queue),
		Routes: make(map[string]map[string]*model.RoutingRule),
	}

	for queueName, maxSize := range queueConfigs {
		domain.Queues[queueName] = &model.Queue{
			Name: queueName,
			Config: model.QueueConfig{
				MaxSize: maxSize,
			},
		}
	}

	return domain
}

func TestGetStats_EmptyState(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{
		domains: []*model.Domain{}, // No domains
	}
	messageRepo := &mockMessageRepository{}

	service := &StatsServiceImpl{
		domainRepo:  domainRepo,
		messageRepo: messageRepo,
		metrics:     setupMetricsStore(logger),
	}

	result, err := service.GetStats(ctx)
	require.NoError(t, err)

	stats, ok := result.(*StatsData)
	require.True(t, ok)

	assert.Equal(t, 0, stats.Domains)
	assert.Equal(t, 0, stats.Queues)
	assert.Equal(t, 0, stats.Messages)
	assert.Equal(t, 0, stats.Routes)
	assert.Empty(t, stats.ActiveDomains)
	assert.Empty(t, stats.TopQueues)
	assert.Empty(t, stats.QueueAlerts)
	assert.Empty(t, stats.MessageRates)
	assert.Empty(t, stats.RecentEvents)
	assert.Nil(t, stats.DomainTrend)
	assert.Nil(t, stats.QueueTrend)
	assert.Nil(t, stats.MessageTrend)
	assert.Nil(t, stats.RouteTrend)
}

func TestGetStats_SingleDomainSingleQueue(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}

	domain := createTestDomain("test-domain", map[string]int{
		"queue1": 1000,
	})

	domainRepo := &mockDomainRepository{
		domains: []*model.Domain{domain},
	}
	messageRepo := &mockMessageRepository{}

	metrics := setupMetricsStore(logger)

	// Add queue snapshot
	metrics.queueSnapshots["test-domain:queue1"] = &QueueSnapshot{
		Domain:          "test-domain",
		Queue:           "queue1",
		BufferSize:      100,
		BufferCapacity:  1000,
		BufferUsage:     10.0,
		RepositoryCount: 100,
		LastUpdated:     time.Now(),
	}

	service := &StatsServiceImpl{
		domainRepo:  domainRepo,
		messageRepo: messageRepo,
		metrics:     metrics,
	}

	result, err := service.GetStats(ctx)
	require.NoError(t, err)

	stats, ok := result.(*StatsData)
	require.True(t, ok)

	assert.Equal(t, 1, stats.Domains)
	assert.Equal(t, 1, stats.Queues)
	assert.Equal(t, 100, stats.Messages)
	assert.Equal(t, 0, stats.Routes)

	require.Len(t, stats.ActiveDomains, 1)
	activeDomain := stats.ActiveDomains[0]
	assert.Equal(t, "test-domain", activeDomain["name"])
	assert.Equal(t, 100, activeDomain["messageCount"])
	assert.Equal(t, 1, activeDomain["queueCount"])

	require.Len(t, stats.TopQueues, 1)
	topQueue := stats.TopQueues[0]
	assert.Equal(t, "test-domain", topQueue["domain"])
	assert.Equal(t, "queue1", topQueue["name"])
	assert.Equal(t, 100, topQueue["messageCount"])
	assert.Equal(t, 1000, topQueue["maxSize"])
	assert.Equal(t, 10.0, topQueue["usage"])

	assert.Empty(t, stats.QueueAlerts) // No alerts for 10% usage
}

func TestGetStats_WithQueueAlerts(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}

	domain := createTestDomain("test-domain", map[string]int{
		"critical-queue": 1000,
		"warning-queue":  500,
	})

	domainRepo := &mockDomainRepository{
		domains: []*model.Domain{domain},
	}
	messageRepo := &mockMessageRepository{}

	metrics := setupMetricsStore(logger)

	// Add queue snapshots with alerts
	now := time.Now()
	metrics.queueSnapshots["test-domain:critical-queue"] = &QueueSnapshot{
		Domain:          "test-domain",
		Queue:           "critical-queue",
		BufferSize:      950,
		BufferCapacity:  1000,
		BufferUsage:     95.0,
		RepositoryCount: 950,
		LastUpdated:     now,
		AlertLevel:      "critical",
		AlertSince:      now.Add(-5 * time.Minute),
		AlertID:         "alert-critical-123",
	}

	metrics.queueSnapshots["test-domain:warning-queue"] = &QueueSnapshot{
		Domain:          "test-domain",
		Queue:           "warning-queue",
		BufferSize:      400,
		BufferCapacity:  500,
		BufferUsage:     80.0,
		RepositoryCount: 400,
		LastUpdated:     now,
		AlertLevel:      "warning",
		AlertSince:      now.Add(-2 * time.Minute),
		AlertID:         "alert-warning-456",
	}

	service := &StatsServiceImpl{
		domainRepo:  domainRepo,
		messageRepo: messageRepo,
		metrics:     metrics,
	}

	result, err := service.GetStats(ctx)
	require.NoError(t, err)

	stats, ok := result.(*StatsData)
	require.True(t, ok)

	require.Len(t, stats.QueueAlerts, 2)

	// Find alerts by severity
	var criticalAlert, warningAlert map[string]any
	for _, alert := range stats.QueueAlerts {
		switch alert["severity"] {
		case "critical":
			criticalAlert = alert
		case "warning":
			warningAlert = alert
		}
	}

	require.NotNil(t, criticalAlert)
	assert.Equal(t, "test-domain", criticalAlert["domain"])
	assert.Equal(t, "critical-queue", criticalAlert["queue"])
	assert.Equal(t, 95.0, criticalAlert["usage"])

	require.NotNil(t, warningAlert)
	assert.Equal(t, "test-domain", warningAlert["domain"])
	assert.Equal(t, "warning-queue", warningAlert["queue"])
	assert.Equal(t, 80.0, warningAlert["usage"])
}

func TestGetStats_WithMessageRates(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{domains: []*model.Domain{}}
	messageRepo := &mockMessageRepository{}

	metrics := setupMetricsStore(logger)

	// Add message rates
	now := time.Now()
	for i := 0; i < 5; i++ {
		metrics.messageRates = append(metrics.messageRates, MessageRate{
			Timestamp:      now.Add(-time.Duration(4-i) * time.Minute).Unix(),
			Rate:           float64(10 + i),
			Published:      float64(5 + i),
			Consumed:       float64(5),
			PublishedTotal: 5 + i,
			ConsumedTotal:  5,
		})
	}

	service := &StatsServiceImpl{
		domainRepo:  domainRepo,
		messageRepo: messageRepo,
		metrics:     metrics,
	}

	result, err := service.GetStats(ctx)
	require.NoError(t, err)

	stats, ok := result.(*StatsData)
	require.True(t, ok)

	require.Len(t, stats.MessageRates, 5)

	// Verify rates are copied correctly
	for i, rate := range stats.MessageRates {
		assert.Equal(t, float64(10+i), rate.Rate)
		assert.Equal(t, 5+i, rate.PublishedTotal)
		assert.Equal(t, 5, rate.ConsumedTotal)
	}
}

func TestGetStats_WithSystemEvents(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{domains: []*model.Domain{}}
	messageRepo := &mockMessageRepository{}

	metrics := setupMetricsStore(logger)

	// Add system events
	now := time.Now()
	events := []model.SystemEvent{
		{
			ID:        "event-1",
			Type:      "info",
			EventType: "domain_created",
			Resource:  "domain1",
			Data:      nil,
			Timestamp: now.Add(-5 * time.Minute),
			UnixTime:  now.Add(-5 * time.Minute).Unix(),
		},
		{
			ID:        "event-2",
			Type:      "warning",
			EventType: "queue_capacity",
			Resource:  "domain1.queue1",
			Data:      85.5,
			Timestamp: now.Add(-2 * time.Minute),
			UnixTime:  now.Add(-2 * time.Minute).Unix(),
		},
		{
			ID:        "event-3",
			Type:      "error",
			EventType: "connection_lost",
			Resource:  "domain1.queue1",
			Data:      map[string]string{"consumerId": "consumer-123"},
			Timestamp: now.Add(-1 * time.Minute),
			UnixTime:  now.Add(-1 * time.Minute).Unix(),
		},
	}

	metrics.systemEvents = events

	service := &StatsServiceImpl{
		domainRepo:  domainRepo,
		messageRepo: messageRepo,
		metrics:     metrics,
	}

	result, err := service.GetStats(ctx)
	require.NoError(t, err)

	stats, ok := result.(*StatsData)
	require.True(t, ok)

	require.Len(t, stats.RecentEvents, 3)

	// Events should be reversed (recent first)
	assert.Equal(t, "event-3", stats.RecentEvents[0]["id"])
	assert.Equal(t, "error", stats.RecentEvents[0]["type"])
	assert.Equal(t, "connection_lost", stats.RecentEvents[0]["eventType"])

	assert.Equal(t, "event-2", stats.RecentEvents[1]["id"])
	assert.Equal(t, "warning", stats.RecentEvents[1]["type"])

	assert.Equal(t, "event-1", stats.RecentEvents[2]["id"])
	assert.Equal(t, "info", stats.RecentEvents[2]["type"])
}

func TestGetStats_WithTrends(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}

	domain := createTestDomain("test-domain", map[string]int{"queue1": 1000})
	domainRepo := &mockDomainRepository{domains: []*model.Domain{domain}}
	messageRepo := &mockMessageRepository{}

	metrics := setupMetricsStore(logger)

	// Add queue snapshot
	metrics.queueSnapshots["test-domain:queue1"] = &QueueSnapshot{
		Domain:          "test-domain",
		Queue:           "queue1",
		BufferSize:      100,
		BufferCapacity:  1000,
		BufferUsage:     10.0,
		RepositoryCount: 100,
		LastUpdated:     time.Now(),
	}

	// Set previous stats for trend calculation
	metrics.previousStats = &StatsData{
		Domains:  2,  // Was 2, now 1 → down 50%
		Queues:   3,  // Was 3, now 1 → down 66.67%
		Messages: 50, // Was 50, now 100 → up 100%
		Routes:   1,  // Was 1, now 0 → down 100%
	}

	service := &StatsServiceImpl{
		domainRepo:  domainRepo,
		messageRepo: messageRepo,
		metrics:     metrics,
	}

	result, err := service.GetStats(ctx)
	require.NoError(t, err)

	stats, ok := result.(*StatsData)
	require.True(t, ok)

	// Verify trends are calculated
	require.NotNil(t, stats.DomainTrend)
	assert.Equal(t, "down", stats.DomainTrend.Direction)
	assert.InDelta(t, 50.0, stats.DomainTrend.Value, 0.1)

	require.NotNil(t, stats.QueueTrend)
	assert.Equal(t, "down", stats.QueueTrend.Direction)
	assert.InDelta(t, 66.67, stats.QueueTrend.Value, 0.1)

	require.NotNil(t, stats.MessageTrend)
	assert.Equal(t, "up", stats.MessageTrend.Direction)
	assert.InDelta(t, 100.0, stats.MessageTrend.Value, 0.1)

	require.NotNil(t, stats.RouteTrend)
	assert.Equal(t, "down", stats.RouteTrend.Direction)
	assert.InDelta(t, 100.0, stats.RouteTrend.Value, 0.1)
}

func TestGetStats_TopQueuesOrdering(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}

	domain := createTestDomain("test-domain", map[string]int{
		"queue1": 1000,
		"queue2": 500,
		"queue3": 800,
	})

	domainRepo := &mockDomainRepository{domains: []*model.Domain{domain}}
	messageRepo := &mockMessageRepository{}

	metrics := setupMetricsStore(logger)

	// Add queue snapshots with different usage levels
	metrics.queueSnapshots["test-domain:queue1"] = &QueueSnapshot{
		Domain: "test-domain", Queue: "queue1",
		BufferSize: 500, BufferCapacity: 1000, BufferUsage: 50.0,
		RepositoryCount: 500, LastUpdated: time.Now(),
	}
	metrics.queueSnapshots["test-domain:queue2"] = &QueueSnapshot{
		Domain: "test-domain", Queue: "queue2",
		BufferSize: 450, BufferCapacity: 500, BufferUsage: 90.0, // Highest usage
		RepositoryCount: 450, LastUpdated: time.Now(),
	}
	metrics.queueSnapshots["test-domain:queue3"] = &QueueSnapshot{
		Domain: "test-domain", Queue: "queue3",
		BufferSize: 560, BufferCapacity: 800, BufferUsage: 70.0,
		RepositoryCount: 560, LastUpdated: time.Now(),
	}

	service := &StatsServiceImpl{
		domainRepo:  domainRepo,
		messageRepo: messageRepo,
		metrics:     metrics,
	}

	result, err := service.GetStats(ctx)
	require.NoError(t, err)

	stats, ok := result.(*StatsData)
	require.True(t, ok)

	require.Len(t, stats.TopQueues, 3)

	// Should be ordered by usage: queue2 (90%) → queue3 (70%) → queue1 (50%)
	assert.Equal(t, "queue2", stats.TopQueues[0]["name"])
	assert.Equal(t, 90.0, stats.TopQueues[0]["usage"])

	assert.Equal(t, "queue3", stats.TopQueues[1]["name"])
	assert.Equal(t, 70.0, stats.TopQueues[1]["usage"])

	assert.Equal(t, "queue1", stats.TopQueues[2]["name"])
	assert.Equal(t, 50.0, stats.TopQueues[2]["usage"])
}
