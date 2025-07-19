package service

import (
	"context"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordEvent(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("Basic event recording", func(t *testing.T) {
		service.RecordEvent("test_event", "info", "test-resource", map[string]string{"key": "value"})

		events := service.metrics.systemEvents
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "info", event.Type)
		assert.Equal(t, "test_event", event.EventType)
		assert.Equal(t, "test-resource", event.Resource)
		assert.NotEmpty(t, event.ID)
		assert.NotZero(t, event.Timestamp)
		assert.NotZero(t, event.UnixTime)
		assert.Equal(t, map[string]string{"key": "value"}, event.Data)
	})

	t.Run("Event with nil data", func(t *testing.T) {
		service.metrics.systemEvents = []model.SystemEvent{} // reset

		service.RecordEvent("simple_event", "warning", "resource1", nil)

		events := service.metrics.systemEvents
		require.Len(t, events, 1)
		assert.Nil(t, events[0].Data)
	})

	t.Run("Multiple different events", func(t *testing.T) {
		service.metrics.systemEvents = []model.SystemEvent{} // reset

		service.RecordEvent("event1", "info", "resource1", nil)
		service.RecordEvent("event2", "warning", "resource2", nil)
		service.RecordEvent("event3", "error", "resource3", nil)

		events := service.metrics.systemEvents
		require.Len(t, events, 3)

		assert.Equal(t, "event1", events[0].EventType)
		assert.Equal(t, "event2", events[1].EventType)
		assert.Equal(t, "event3", events[2].EventType)
	})
}

func TestRecordEvent_DomainActiveDeduplication(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("First domain_active event", func(t *testing.T) {
		service.RecordEvent("domain_active", "info", "domain1", map[string]any{"queueCount": 5})

		events := service.metrics.systemEvents
		require.Len(t, events, 1)
		assert.Equal(t, "domain_active", events[0].EventType)
		assert.Equal(t, "domain1", events[0].Resource)
	})

	t.Run("Duplicate domain_active with same data - no new event", func(t *testing.T) {
		initialCount := len(service.metrics.systemEvents)
		originalID := service.metrics.systemEvents[0].ID
		originalTime := service.metrics.systemEvents[0].UnixTime

		service.RecordEvent("domain_active", "info", "domain1", map[string]any{"queueCount": 5})

		events := service.metrics.systemEvents
		assert.Len(t, events, initialCount)               // no new event
		assert.Equal(t, originalID, events[0].ID)         // same ID
		assert.Equal(t, originalTime, events[0].UnixTime) // same timestamp
	})

	t.Run("domain_active with changed data - updates existing", func(t *testing.T) {
		initialCount := len(service.metrics.systemEvents)
		originalID := service.metrics.systemEvents[0].ID

		time.Sleep(1 * time.Millisecond) // ensure different timestamp
		service.RecordEvent("domain_active", "info", "domain1", map[string]any{"queueCount": 7})

		events := service.metrics.systemEvents
		assert.Len(t, events, initialCount)          // still same count
		assert.NotEqual(t, originalID, events[0].ID) // new ID generated

		data, ok := events[0].Data.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 7, data["queueCount"])
	})

	t.Run("Different domain - creates new event", func(t *testing.T) {
		initialCount := len(service.metrics.systemEvents)

		service.RecordEvent("domain_active", "info", "domain2", map[string]any{"queueCount": 3})

		events := service.metrics.systemEvents
		assert.Len(t, events, initialCount+1) // new event created
		assert.Equal(t, "domain2", events[len(events)-1].Resource)
	})
}

func TestRecordEvent_QueueCapacityDeduplication(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("First queue_capacity event", func(t *testing.T) {
		service.RecordEvent("queue_capacity", "warning", "domain1.queue1", 80.0)

		events := service.metrics.systemEvents
		require.Len(t, events, 1)
		assert.Equal(t, "queue_capacity", events[0].EventType)
		assert.Equal(t, "warning", events[0].Type)
	})

	t.Run("Same severity - updates existing", func(t *testing.T) {
		initialCount := len(service.metrics.systemEvents)

		service.RecordEvent("queue_capacity", "warning", "domain1.queue1", 85.0)

		events := service.metrics.systemEvents
		assert.Len(t, events, initialCount) // same count
		assert.Equal(t, 85.0, events[0].Data)
	})

	t.Run("Higher severity - updates existing", func(t *testing.T) {
		initialCount := len(service.metrics.systemEvents)

		service.RecordEvent("queue_capacity", "error", "domain1.queue1", 95.0)

		events := service.metrics.systemEvents
		assert.Len(t, events, initialCount) // same count
		assert.Equal(t, "error", events[0].Type)
		assert.Equal(t, 95.0, events[0].Data)
	})

	t.Run("Different queue - creates new event", func(t *testing.T) {
		initialCount := len(service.metrics.systemEvents)

		service.RecordEvent("queue_capacity", "warning", "domain1.queue2", 70.0)

		events := service.metrics.systemEvents
		assert.Len(t, events, initialCount+1)
		assert.Equal(t, "domain1.queue2", events[len(events)-1].Resource)
	})
}

func TestRecordEvent_MaxEventsLimit(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	// Add more than maxEvents (50) to test truncation
	for i := 0; i < 55; i++ {
		service.RecordEvent("test_event", "info", "resource", i)
	}

	events := service.metrics.systemEvents
	assert.LessOrEqual(t, len(events), maxEvents)
	assert.Equal(t, 50, len(events)) // should be exactly 50
}

func TestRecordDomainActive(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("Records domain active event", func(t *testing.T) {
		service.RecordDomainActive("test-domain", 5)

		events := service.metrics.systemEvents
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "domain_active", event.EventType)
		assert.Equal(t, "info", event.Type)
		assert.Equal(t, "test-domain", event.Resource)

		data, ok := event.Data.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 5, data["queueCount"])
	})
}

func TestRecordDomainCreated(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("Records domain created event", func(t *testing.T) {
		service.RecordDomainCreated("new-domain")

		events := service.metrics.systemEvents
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "domain_created", event.EventType)
		assert.Equal(t, "info", event.Type)
		assert.Equal(t, "new-domain", event.Resource)
		assert.Nil(t, event.Data)
	})
}

func TestRecordQueueCreated(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("Records queue created event with correct resource format", func(t *testing.T) {
		service.RecordQueueCreated("domain1", "queue1")

		events := service.metrics.systemEvents
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "queue_created", event.EventType)
		assert.Equal(t, "info", event.Type)
		assert.Equal(t, "domain1.queue1", event.Resource)
		assert.Nil(t, event.Data)
	})
}

func TestRecordQueueCapacity(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("Warning level for usage < 90", func(t *testing.T) {
		service.RecordQueueCapacity("domain1", "queue1", 85.5)
		service.FlushEvents()

		events := service.metrics.systemEvents
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "queue_capacity", event.EventType)
		assert.Equal(t, "warning", event.Type)
		assert.Equal(t, "domain1.queue1", event.Resource)
		assert.Equal(t, 85.5, event.Data)
	})

	t.Run("Error level for usage >= 90", func(t *testing.T) {
		service.metrics.systemEvents = []model.SystemEvent{} // reset

		service.RecordQueueCapacity("domain1", "queue1", 95.0)
		service.FlushEvents()

		events := service.metrics.systemEvents
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "error", event.Type)
		assert.Equal(t, 95.0, event.Data)
	})
}

func TestRecordConnectionLost(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("Records connection lost event", func(t *testing.T) {
		service.RecordConnectionLost("domain1", "queue1", "consumer-123")

		events := service.metrics.systemEvents
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "connection_lost", event.EventType)
		assert.Equal(t, "error", event.Type)
		assert.Equal(t, "domain1.queue1", event.Resource)

		data, ok := event.Data.(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "consumer-123", data["consumerId"])
	})
}

func TestRecordRoutingRuleCreated(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("Records routing rule created event", func(t *testing.T) {
		service.RecordRoutingRuleCreated("domain1", "source-queue", "dest-queue")

		events := service.metrics.systemEvents
		require.Len(t, events, 1)

		event := events[0]
		assert.Equal(t, "routing_rule_created", event.EventType)
		assert.Equal(t, "info", event.Type)
		assert.Equal(t, "domain1", event.Resource)

		data, ok := event.Data.(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "source-queue", data["source"])
		assert.Equal(t, "dest-queue", data["destination"])
	})
}
