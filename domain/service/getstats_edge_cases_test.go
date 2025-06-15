package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock repository that can simulate errors
type mockDomainRepositoryWithErrors struct {
	domains     []*model.Domain
	shouldError bool
	errorMsg    string
}

func (m *mockDomainRepositoryWithErrors) StoreDomain(ctx context.Context, domain *model.Domain) error {
	if m.shouldError {
		return errors.New(m.errorMsg)
	}
	m.domains = append(m.domains, domain)
	return nil
}

func (m *mockDomainRepositoryWithErrors) GetDomain(ctx context.Context, name string) (*model.Domain, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}
	for _, d := range m.domains {
		if d.Name == name {
			return d, nil
		}
	}
	return nil, nil
}

func (m *mockDomainRepositoryWithErrors) ListDomains(ctx context.Context) ([]*model.Domain, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMsg)
	}
	return m.domains, nil
}

func (m *mockDomainRepositoryWithErrors) DeleteDomain(ctx context.Context, name string) error {
	if m.shouldError {
		return errors.New(m.errorMsg)
	}
	for i, d := range m.domains {
		if d.Name == name {
			m.domains = append(m.domains[:i], m.domains[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockDomainRepositoryWithErrors) SystemDomains(ctx context.Context) ([]*model.Domain, error) {
	return []*model.Domain{}, nil
}

func TestGetStats_EdgeCases(t *testing.T) {
	t.Run("Repository error - ListDomains fails", func(t *testing.T) {
		ctx := context.Background()
		logger := &mockLogger{}

		domainRepo := &mockDomainRepositoryWithErrors{
			shouldError: true,
			errorMsg:    "database connection failed",
		}
		messageRepo := &mockMessageRepository{}

		service := &StatsServiceImpl{
			domainRepo:  domainRepo,
			messageRepo: messageRepo,
			metrics:     setupMetricsStore(logger),
		}

		result, err := service.GetStats(ctx)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "database connection failed")
	})

	t.Run("Context timeout", func(t *testing.T) {
		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Ensure context is already cancelled
		time.Sleep(1 * time.Millisecond)

		logger := &mockLogger{}
		domainRepo := &mockDomainRepository{
			domains: []*model.Domain{createTestDomain("test", map[string]int{"queue1": 1000})},
		}
		messageRepo := &mockMessageRepository{}

		service := &StatsServiceImpl{
			domainRepo:  domainRepo,
			messageRepo: messageRepo,
			metrics:     setupMetricsStore(logger),
		}

		result, err := service.GetStats(ctx)

		// Should handle context cancellation gracefully
		// Note: Actual behavior depends on how ListDomains implements context handling
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		} else {
			// If no error, result should still be valid (graceful degradation)
			assert.NotNil(t, result)
		}
	})

	t.Run("Nil domains from repository", func(t *testing.T) {
		ctx := context.Background()
		logger := &mockLogger{}

		domainRepo := &mockDomainRepository{
			domains: nil, // Repository returns nil slice
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

		// Should handle nil gracefully
		assert.Equal(t, 0, stats.Domains)
		assert.Equal(t, 0, stats.Queues)
		assert.Equal(t, 0, stats.Messages)
		assert.Empty(t, stats.ActiveDomains)
	})

	t.Run("Corrupted MetricsStore data", func(t *testing.T) {
		ctx := context.Background()
		logger := &mockLogger{}
		domainRepo := &mockDomainRepository{domains: []*model.Domain{}}
		messageRepo := &mockMessageRepository{}

		metrics := setupMetricsStore(logger)

		// Corrupt messageRates with invalid data
		metrics.messageRates = []MessageRate{
			{Timestamp: -1, Rate: -999, Published: -1, Consumed: -1},
			{}, // Empty rate
		}

		// Corrupt queueSnapshots
		metrics.queueSnapshots["invalid::key"] = &QueueSnapshot{
			Domain:      "",
			Queue:       "",
			BufferSize:  -1,
			BufferUsage: -50.0, // Invalid percentage
		}

		// Corrupt systemEvents
		metrics.systemEvents = []model.SystemEvent{
			{ID: "", Type: "", EventType: "", Resource: ""}, // Empty event
		}

		service := &StatsServiceImpl{
			domainRepo:  domainRepo,
			messageRepo: messageRepo,
			metrics:     metrics,
		}

		result, err := service.GetStats(ctx)
		require.NoError(t, err) // Should not crash

		stats, ok := result.(*StatsData)
		require.True(t, ok)

		// Should handle corrupted data gracefully
		assert.NotNil(t, stats.MessageRates) // May be empty or contain corrupt data
		assert.NotNil(t, stats.RecentEvents)
		assert.NotNil(t, stats.TopQueues)
	})

	t.Run("Extremely large dataset", func(t *testing.T) {
		ctx := context.Background()
		logger := &mockLogger{}

		// Create many domains with many queues
		domains := make([]*model.Domain, 0, 100)
		for i := 0; i < 100; i++ {
			queueConfigs := make(map[string]int)
			for j := 0; j < 10; j++ {
				queueConfigs[fmt.Sprintf("queue%d", j)] = 1000
			}
			domains = append(domains, createTestDomain(fmt.Sprintf("domain%d", i), queueConfigs))
		}

		domainRepo := &mockDomainRepository{domains: domains}
		messageRepo := &mockMessageRepository{}

		metrics := setupMetricsStore(logger)

		// Add many queue snapshots
		for i := 0; i < 100; i++ {
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("domain%d:queue%d", i, j)
				metrics.queueSnapshots[key] = &QueueSnapshot{
					Domain:          fmt.Sprintf("domain%d", i),
					Queue:           fmt.Sprintf("queue%d", j),
					BufferSize:      500,
					BufferCapacity:  1000,
					BufferUsage:     50.0,
					RepositoryCount: 500,
					LastUpdated:     time.Now(),
				}
			}
		}

		// Add many message rates
		for i := 0; i < 1000; i++ {
			metrics.messageRates = append(metrics.messageRates, MessageRate{
				Timestamp:      time.Now().Add(-time.Duration(i) * time.Second).Unix(),
				Rate:           float64(i),
				Published:      float64(i / 2),
				Consumed:       float64(i / 2),
				PublishedTotal: i / 2,
				ConsumedTotal:  i / 2,
			})
		}

		service := &StatsServiceImpl{
			domainRepo:  domainRepo,
			messageRepo: messageRepo,
			metrics:     metrics,
		}

		start := time.Now()
		result, err := service.GetStats(ctx)
		duration := time.Since(start)

		require.NoError(t, err)

		stats, ok := result.(*StatsData)
		require.True(t, ok)

		// Verify performance and data integrity
		assert.Equal(t, 100, stats.Domains)
		assert.Equal(t, 1000, stats.Queues) // 100 domains * 10 queues each
		assert.Len(t, stats.MessageRates, 1000)

		// TopQueues should be limited to 5
		assert.LessOrEqual(t, len(stats.TopQueues), 5)

		// Performance check - should complete reasonably fast
		assert.Less(t, duration, 5*time.Second, "GetStats should complete within 5 seconds even with large dataset")
	})

	t.Run("Concurrent access during GetStats", func(t *testing.T) {
		ctx := context.Background()
		logger := &mockLogger{}

		domain := createTestDomain("test-domain", map[string]int{"queue1": 1000})
		domainRepo := &mockDomainRepository{domains: []*model.Domain{domain}}
		messageRepo := &mockMessageRepository{}

		metrics := setupMetricsStore(logger)

		// Add initial data
		metrics.queueSnapshots["test-domain:queue1"] = &QueueSnapshot{
			Domain: "test-domain", Queue: "queue1",
			BufferSize: 500, BufferCapacity: 1000, BufferUsage: 50.0,
			RepositoryCount: 500, LastUpdated: time.Now(),
		}

		service := &StatsServiceImpl{
			domainRepo:  domainRepo,
			messageRepo: messageRepo,
			metrics:     metrics,
		}

		// Simulate concurrent modifications to metrics while GetStats is running
		go func() {
			for i := 0; i < 100; i++ {
				service.TrackMessagePublished("test-domain", "queue1")
				service.TrackMessageConsumed("test-domain", "queue1")
				service.RecordEvent("test_event", "info", "resource", i)
				time.Sleep(1 * time.Millisecond)
			}
		}()

		// Multiple concurrent GetStats calls
		results := make(chan any, 10)
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			go func() {
				result, err := service.GetStats(ctx)
				if err != nil {
					errors <- err
				} else {
					results <- result
				}
			}()
		}

		// Collect results
		for i := 0; i < 10; i++ {
			select {
			case result := <-results:
				// Should get valid result
				stats, ok := result.(*StatsData)
				assert.True(t, ok)
				assert.NotNil(t, stats)
			case err := <-errors:
				// Should not error due to concurrency
				assert.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("GetStats took too long - possible deadlock")
			}
		}
	})
}
