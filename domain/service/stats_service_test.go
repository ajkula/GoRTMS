package service

import (
	"context"
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations
type mockLogger struct{}

func (m *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Warn(msg string, keysAndValues ...interface{})  {}

type mockDomainRepository struct {
	domains []*model.Domain
}

func (m *mockDomainRepository) StoreDomain(ctx context.Context, domain *model.Domain) error {
	m.domains = append(m.domains, domain)
	return nil
}

func (m *mockDomainRepository) GetDomain(ctx context.Context, name string) (*model.Domain, error) {
	for _, d := range m.domains {
		if d.Name == name {
			return d, nil
		}
	}
	return nil, nil
}

func (m *mockDomainRepository) ListDomains(ctx context.Context) ([]*model.Domain, error) {
	return m.domains, nil
}

func (m *mockDomainRepository) DeleteDomain(ctx context.Context, name string) error {
	for i, d := range m.domains {
		if d.Name == name {
			m.domains = append(m.domains[:i], m.domains[i+1:]...)
			return nil
		}
	}
	return nil
}

type mockMessageRepository struct {
	messages    map[string][]*model.Message // key: "domain:queue"
	ackMatrices map[string]*model.AckMatrix // key: "domain:queue"
}

func (m *mockMessageRepository) init() {
	if m.messages == nil {
		m.messages = make(map[string][]*model.Message)
	}
	if m.ackMatrices == nil {
		m.ackMatrices = make(map[string]*model.AckMatrix)
	}
}

func (m *mockMessageRepository) StoreMessage(ctx context.Context, domainName, queueName string, message *model.Message) error {
	m.init()
	key := domainName + ":" + queueName
	m.messages[key] = append(m.messages[key], message)
	return nil
}

func (m *mockMessageRepository) GetMessage(ctx context.Context, domainName, queueName, messageID string) (*model.Message, error) {
	m.init()
	key := domainName + ":" + queueName
	for _, msg := range m.messages[key] {
		if msg.ID == messageID {
			return msg, nil
		}
	}
	return nil, nil
}

func (m *mockMessageRepository) DeleteMessage(ctx context.Context, domainName, queueName, messageID string) error {
	m.init()
	key := domainName + ":" + queueName
	msgs := m.messages[key]
	for i, msg := range msgs {
		if msg.ID == messageID {
			m.messages[key] = append(msgs[:i], msgs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockMessageRepository) GetMessagesAfterIndex(ctx context.Context, domainName, queueName string, startIndex int64, limit int) ([]*model.Message, error) {
	m.init()
	key := domainName + ":" + queueName
	msgs := m.messages[key]

	if int(startIndex) >= len(msgs) {
		return []*model.Message{}, nil
	}

	end := int(startIndex) + limit
	if end > len(msgs) {
		end = len(msgs)
	}

	return msgs[startIndex:end], nil
}

func (m *mockMessageRepository) GetIndexByMessageID(ctx context.Context, domainName, queueName, messageID string) (int64, error) {
	m.init()
	key := domainName + ":" + queueName
	for i, msg := range m.messages[key] {
		if msg.ID == messageID {
			return int64(i), nil
		}
	}
	return -1, nil
}

func (m *mockMessageRepository) GetOrCreateAckMatrix(domainName, queueName string) *model.AckMatrix {
	m.init()
	key := domainName + ":" + queueName
	if matrix, exists := m.ackMatrices[key]; exists {
		return matrix
	}
	matrix := model.NewAckMatrix()
	m.ackMatrices[key] = matrix
	return matrix
}

func (m *mockMessageRepository) AcknowledgeMessage(ctx context.Context, domainName, queueName, groupID, messageID string) (bool, error) {
	matrix := m.GetOrCreateAckMatrix(domainName, queueName)
	return matrix.Acknowledge(groupID, messageID), nil
}

func (m *mockMessageRepository) ClearQueueIndices(ctx context.Context, domainName, queueName string) {
	// Mock implementation - nothing to do
}

func (m *mockMessageRepository) CleanupMessageIndices(ctx context.Context, domainName, queueName string, minPosition int64) {
	// Mock implementation - nothing to do
}

func (m *mockMessageRepository) GetQueueMessageCount(domainName, queueName string) int {
	m.init()
	key := domainName + ":" + queueName
	return len(m.messages[key])
}

func TestDetermineGranularity(t *testing.T) {
	tests := []struct {
		name        string
		period      string
		granularity string
		expected    int
	}{
		// Auto granularity tests
		{"Auto 1h", "1h", "auto", 60},
		{"Auto 6h", "6h", "auto", 300},
		{"Auto 12h", "12h", "auto", 900},
		{"Auto 24h", "24h", "auto", 1800},
		{"Auto unknown period", "48h", "auto", 60},

		// Explicit granularity tests
		{"Explicit 10s", "1h", "10s", 10},
		{"Explicit 1m", "1h", "1m", 60},
		{"Explicit 5m", "6h", "5m", 300},
		{"Explicit 15m", "12h", "15m", 900},
		{"Explicit 30m", "24h", "30m", 1800},
		{"Explicit 1h", "24h", "1h", 3600},
		{"Unknown granularity", "1h", "2m", 60},
	}

	s := &StatsServiceImpl{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.determineGranularity(tt.period, tt.granularity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAggregateMessageRates(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	testRates := []MessageRate{
		// Group 1: -90 min
		{Timestamp: baseTime.Add(-90 * time.Minute).Unix(), PublishedTotal: 10, ConsumedTotal: 5},
		// Group 2: -60 min
		{Timestamp: baseTime.Add(-60 * time.Minute).Unix(), PublishedTotal: 20, ConsumedTotal: 10},
		// Group 3: -30 min
		{Timestamp: baseTime.Add(-30 * time.Minute).Unix(), PublishedTotal: 30, ConsumedTotal: 15},
		// Group 4: -15 min
		{Timestamp: baseTime.Add(-15 * time.Minute).Unix(), PublishedTotal: 40, ConsumedTotal: 20},
		// Group 5: -5 min
		{Timestamp: baseTime.Add(-5 * time.Minute).Unix(), PublishedTotal: 50, ConsumedTotal: 25},
	}

	s := &StatsServiceImpl{
		metrics: &MetricsStore{
			messageRates: testRates,
			logger:       &mockLogger{},
		},
	}

	tests := []struct {
		name               string
		startTime          time.Time
		granularitySeconds int
		expectedBuckets    int
		description        string
	}{
		{
			name:               "15 minute granularity for last 2 hours",
			startTime:          baseTime.Add(-2 * time.Hour),
			granularitySeconds: 900, // 15 minutes
			expectedBuckets:    4,
			description:        "Points at -90, -60, -30, -15, -5 min → 5 buckets",
		},
		{
			name:               "30 minute granularity for last 2 hours",
			startTime:          baseTime.Add(-2 * time.Hour),
			granularitySeconds: 1800, // 30 minutes
			expectedBuckets:    3,    // -90 alone, -60 alone, -30 alone, -15/-5 together
			description:        "-90→-90, -60→-60, -30→-30, -15/-5→-0",
		},
		{
			name:               "60 minute granularity for last 2 hours",
			startTime:          baseTime.Add(-2 * time.Hour),
			granularitySeconds: 3600, // 60 minutes
			expectedBuckets:    2,    // -90/-60 together, -30/-15/-5 together
			description:        "-90/-60→-60, -30/-15/-5→-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.aggregateMessageRates(tt.startTime, tt.granularitySeconds)

			t.Logf("Test: %s", tt.description)
			t.Logf("Expected %d buckets, got %d", tt.expectedBuckets, len(result))

			assert.Equal(t, tt.expectedBuckets, len(result), "Bucket count mismatch")

			// Verify aggregation is correct
			totalPublished := 0
			totalConsumed := 0
			for _, rate := range result {
				totalPublished += rate.PublishedTotal
				totalConsumed += rate.ConsumedTotal
			}
			assert.Equal(t, 150, totalPublished, "Total published should be sum of all inputs")
			assert.Equal(t, 75, totalConsumed, "Total consumed should be sum of all inputs")
		})
	}
}

func TestGetStatsWithAggregation(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{
		domains: []*model.Domain{
			{Name: "test-domain", Queues: map[string]*model.Queue{
				"queue1": {Name: "queue1", Config: model.QueueConfig{MaxSize: 1000}},
			}},
		},
	}
	messageRepo := &mockMessageRepository{}

	metrics := &MetricsStore{
		rootCtx:        ctx,
		logger:         logger,
		messageRates:   make([]MessageRate, 0),
		queueSnapshots: make(map[string]*QueueSnapshot),
		lastCollected:  time.Now(),
		systemEvents:   make([]model.SystemEvent, 0),
	}

	s := &StatsServiceImpl{
		domainRepo:      domainRepo,
		messageRepo:     messageRepo,
		metrics:         metrics,
		collectInterval: time.Second,
		stopCollect:     make(chan struct{}),
	}

	// Use current time for test data
	now := time.Now()
	for i := 0; i < 60; i++ {
		s.metrics.messageRates = append(s.metrics.messageRates, MessageRate{
			Timestamp:      now.Add(-time.Duration(59-i) * time.Minute).Unix(),
			PublishedTotal: 1,
			ConsumedTotal:  1,
		})
	}

	tests := []struct {
		name        string
		period      string
		granularity string
		expected    int
	}{
		{
			name:        "Default 1h auto",
			period:      "1h",
			granularity: "auto",
			expected:    60,
		},
		{
			name:        "6h with 5m",
			period:      "6h",
			granularity: "5m",
			expected:    12,
		},
		{
			name:        "24h with auto",
			period:      "24h",
			granularity: "auto",
			expected:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.GetStatsWithAggregation(ctx, tt.period, tt.granularity)
			require.NoError(t, err)

			stats, ok := result.(*StatsData)
			require.True(t, ok)

			assert.NotEmpty(t, stats.MessageRates)

			totalPublished := 0
			for _, rate := range stats.MessageRates {
				totalPublished += rate.PublishedTotal
			}

			assert.Equal(t, 60, totalPublished)
			assert.Equal(t, 1, stats.Domains)
			assert.NotNil(t, stats.ActiveDomains)
		})
	}
}

func TestGetStatsWithAggregation_EdgeCases(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	domainRepo := &mockDomainRepository{}
	messageRepo := &mockMessageRepository{}

	s := NewStatsService(ctx, logger, domainRepo, messageRepo).(*StatsServiceImpl)

	t.Run("No message rates", func(t *testing.T) {
		result, err := s.GetStatsWithAggregation(ctx, "1h", "auto")
		require.NoError(t, err)

		stats, ok := result.(*StatsData)
		require.True(t, ok)
		assert.Empty(t, stats.MessageRates)
	})

	t.Run("Invalid parameters", func(t *testing.T) {
		// Should use defaults
		result, err := s.GetStatsWithAggregation(ctx, "invalid", "invalid")
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("Concurrent access", func(t *testing.T) {
		// Add some data
		now := time.Now()
		for i := 0; i < 10; i++ {
			s.metrics.messageRates = append(s.metrics.messageRates, MessageRate{
				Timestamp:      now.Add(-time.Duration(i) * time.Second).Unix(),
				PublishedTotal: i,
				ConsumedTotal:  i,
			})
		}

		// Concurrent reads should not panic
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				_, err := s.GetStatsWithAggregation(ctx, "1h", "auto")
				assert.NoError(t, err)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestCalculateTrend(t *testing.T) {
	tests := []struct {
		name     string
		previous int
		current  int
		expected *Trend
	}{
		{
			name:     "No previous data",
			previous: 0,
			current:  10,
			expected: nil,
		},

		{
			name:     "No change",
			previous: 10,
			current:  10,
			expected: &Trend{Direction: "up", Value: 0},
		},

		{
			name:     "20% increase",
			previous: 10,
			current:  12,
			expected: &Trend{Direction: "up", Value: 20},
		},
		{
			name:     "100% increase (double)",
			previous: 5,
			current:  10,
			expected: &Trend{Direction: "up", Value: 100},
		},
		{
			name:     "Large increase",
			previous: 1,
			current:  10,
			expected: &Trend{Direction: "up", Value: 900},
		},

		{
			name:     "50% decrease",
			previous: 10,
			current:  5,
			expected: &Trend{Direction: "down", Value: 50},
		},
		{
			name:     "Complete decrease to zero",
			previous: 10,
			current:  0,
			expected: &Trend{Direction: "down", Value: 100},
		},
		{
			name:     "Small decrease",
			previous: 100,
			current:  99,
			expected: &Trend{Direction: "down", Value: 1},
		},

		{
			name:     "Negative previous to positive",
			previous: -10,
			current:  5,
			expected: &Trend{Direction: "up", Value: 150},
		},
		{
			name:     "Both negative - improvement",
			previous: -10,
			current:  -5,
			expected: &Trend{Direction: "up", Value: 50},
		},
		{
			name:     "Both negative - degradation",
			previous: -5,
			current:  -10,
			expected: &Trend{Direction: "down", Value: 100},
		},

		{
			name:     "Large numbers",
			previous: 1000000,
			current:  1100000,
			expected: &Trend{Direction: "up", Value: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTrend(tt.previous, tt.current)

			if tt.expected == nil {
				assert.Nil(t, result, "Expected nil trend")
			} else {
				assert.NotNil(t, result, "Expected non-nil trend")
				assert.Equal(t, tt.expected.Direction, result.Direction, "Direction should match")
				assert.InDelta(t, tt.expected.Value, result.Value, 0.01, "Value should match within tolerance")
			}
		})
	}
}

func TestCalculateDomainMessageRate(t *testing.T) {
	tests := []struct {
		name       string
		domainName string
		rates      []MessageRate
		expected   float64
	}{
		{
			name:       "Empty rates slice",
			domainName: "test-domain",
			rates:      []MessageRate{},
			expected:   0,
		},

		{
			name:       "Single rate",
			domainName: "test-domain",
			rates: []MessageRate{
				{Rate: 10.5},
			},
			expected: 10.5,
		},

		{
			name:       "Multiple rates - returns last",
			domainName: "test-domain",
			rates: []MessageRate{
				{Rate: 5.0},
				{Rate: 15.0},
				{Rate: 25.5},
			},
			expected: 25.5,
		},

		{
			name:       "Zero rate",
			domainName: "test-domain",
			rates: []MessageRate{
				{Rate: 0.0},
			},
			expected: 0.0,
		},

		{
			name:       "Decimal rates",
			domainName: "test-domain",
			rates: []MessageRate{
				{Rate: 1.234},
				{Rate: 5.678},
				{Rate: 9.999},
			},
			expected: 9.999,
		},

		{
			name:       "Different domain name (should not affect result)",
			domainName: "another-domain",
			rates: []MessageRate{
				{Rate: 42.0},
			},
			expected: 42.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateDomainMessageRate(tt.domainName, tt.rates)
			assert.Equal(t, tt.expected, result, "Message rate should match expected value")
		})
	}
}

func TestCalculateDomainMessageRate_SliceModification(t *testing.T) {
	rates := []MessageRate{
		{Rate: 10.0},
		{Rate: 20.0},
		{Rate: 30.0},
	}

	result := calculateDomainMessageRate("test", rates)
	assert.Equal(t, 30.0, result)

	rates[2].Rate = 99.0

	newResult := calculateDomainMessageRate("test", rates)
	assert.Equal(t, 99.0, newResult)

	assert.Equal(t, 30.0, result, "Previous result should remain unchanged")
}
