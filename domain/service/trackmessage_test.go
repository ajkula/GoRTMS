package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrackMessagePublished(t *testing.T) {
	service := &StatsServiceImpl{
		publishCountSinceLastCollect: 0,
		consumeCountSinceLastCollect: 0,
	}

	t.Run("Single call increments counter", func(t *testing.T) {
		service.publishCountSinceLastCollect = 0

		service.TrackMessagePublished("domain1", "queue1")

		assert.Equal(t, 1, service.publishCountSinceLastCollect)
		assert.Equal(t, 0, service.consumeCountSinceLastCollect)
	})

	t.Run("Multiple calls increment correctly", func(t *testing.T) {
		service.publishCountSinceLastCollect = 0

		service.TrackMessagePublished("domain1", "queue1")
		service.TrackMessagePublished("domain1", "queue2")
		service.TrackMessagePublished("domain2", "queue1")

		assert.Equal(t, 3, service.publishCountSinceLastCollect)
	})

	t.Run("Parameters dont affect counter logic", func(t *testing.T) {
		service.publishCountSinceLastCollect = 5

		service.TrackMessagePublished("", "")
		service.TrackMessagePublished("very-long-domain-name", "very-long-queue-name")

		assert.Equal(t, 7, service.publishCountSinceLastCollect)
	})
}

func TestTrackMessageConsumed(t *testing.T) {
	service := &StatsServiceImpl{
		publishCountSinceLastCollect: 0,
		consumeCountSinceLastCollect: 0,
	}

	t.Run("Single call increments counter", func(t *testing.T) {
		service.consumeCountSinceLastCollect = 0

		service.TrackMessageConsumed("domain1", "queue1")

		assert.Equal(t, 1, service.consumeCountSinceLastCollect)
		assert.Equal(t, 0, service.publishCountSinceLastCollect)
	})

	t.Run("Multiple calls increment correctly", func(t *testing.T) {
		service.consumeCountSinceLastCollect = 0

		service.TrackMessageConsumed("domain1", "queue1")
		service.TrackMessageConsumed("domain1", "queue1")
		service.TrackMessageConsumed("domain2", "queue2")

		assert.Equal(t, 3, service.consumeCountSinceLastCollect)
	})
}

func TestTrackMessagesConcurrency(t *testing.T) {
	service := &StatsServiceImpl{
		publishCountSinceLastCollect: 0,
		consumeCountSinceLastCollect: 0,
	}

	const numGoroutines = 100
	const callsPerGoroutine = 10

	t.Run("Concurrent published tracking", func(t *testing.T) {
		service.publishCountSinceLastCollect = 0

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < callsPerGoroutine; j++ {
					service.TrackMessagePublished("domain1", "queue1")
				}
			}(i)
		}

		wg.Wait()

		expected := numGoroutines * callsPerGoroutine
		assert.Equal(t, expected, service.publishCountSinceLastCollect)
	})

	t.Run("Concurrent consumed tracking", func(t *testing.T) {
		service.consumeCountSinceLastCollect = 0

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < callsPerGoroutine; j++ {
					service.TrackMessageConsumed("domain1", "queue1")
				}
			}(i)
		}

		wg.Wait()

		expected := numGoroutines * callsPerGoroutine
		assert.Equal(t, expected, service.consumeCountSinceLastCollect)
	})

	t.Run("Mixed concurrent tracking", func(t *testing.T) {
		service.publishCountSinceLastCollect = 0
		service.consumeCountSinceLastCollect = 0

		var wg sync.WaitGroup
		wg.Add(numGoroutines * 2)

		// Goroutines for published
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < callsPerGoroutine; j++ {
					service.TrackMessagePublished("domain1", "queue1")
				}
			}()
		}

		// Goroutines for consumed
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < callsPerGoroutine; j++ {
					service.TrackMessageConsumed("domain1", "queue1")
				}
			}()
		}

		wg.Wait()

		expected := numGoroutines * callsPerGoroutine
		assert.Equal(t, expected, service.publishCountSinceLastCollect)
		assert.Equal(t, expected, service.consumeCountSinceLastCollect)
	})
}

func TestTrackMessagesIntegrationWithCollectMetrics(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	service := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)
	defer service.Cleanup()

	t.Run("Counters reset after collectMetrics", func(t *testing.T) {
		service.TrackMessagePublished("domain1", "queue1")
		service.TrackMessagePublished("domain1", "queue1")
		service.TrackMessageConsumed("domain1", "queue1")

		assert.Equal(t, 2, service.publishCountSinceLastCollect)
		assert.Equal(t, 1, service.consumeCountSinceLastCollect)

		service.collectMetrics()

		assert.Equal(t, 0, service.publishCountSinceLastCollect)
		assert.Equal(t, 0, service.consumeCountSinceLastCollect)
	})

	t.Run("Message rates reflect tracked messages", func(t *testing.T) {
		service.publishCountSinceLastCollect = 0
		service.consumeCountSinceLastCollect = 0

		service.TrackMessagePublished("domain1", "queue1")
		service.TrackMessagePublished("domain1", "queue1")
		service.TrackMessageConsumed("domain1", "queue1")

		initialRatesCount := len(service.metrics.messageRates)

		service.collectMetrics()

		assert.Equal(t, initialRatesCount+1, len(service.metrics.messageRates))

		latestRate := service.metrics.messageRates[len(service.metrics.messageRates)-1]
		assert.Equal(t, 2, latestRate.PublishedTotal)
		assert.Equal(t, 1, latestRate.ConsumedTotal)
		assert.True(t, latestRate.Rate > 0)
	})
}
