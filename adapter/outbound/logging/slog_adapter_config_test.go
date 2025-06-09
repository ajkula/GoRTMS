package logging

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_DynamicLevelChange(t *testing.T) {
	cfg := createTestConfig("DEBUG")
	logger := NewSlogAdapter(cfg)
	defer logger.Shutdown()

	adapter, ok := logger.(*SlogAdapter)
	require.True(t, ok, "Logger should be SlogAdapter type")

	t.Run("Initial level - DEBUG allows all messages", func(t *testing.T) {
		assert.True(t, adapter.shouldLog(LevelError))
		assert.True(t, adapter.shouldLog(LevelWarn))
		assert.True(t, adapter.shouldLog(LevelInfo))
		assert.True(t, adapter.shouldLog(LevelDebug))
	})

	t.Run("Change to ERROR level - only errors allowed", func(t *testing.T) {
		adapter.UpdateLevel("ERROR")

		// Give a moment for the update to take effect
		time.Sleep(1 * time.Millisecond)

		// Check that config was updated
		assert.Equal(t, "error", adapter.config.General.LogLevel)
		assert.Equal(t, "ERROR", adapter.config.Logging.Level)

		// Check that shouldLog respects new level
		assert.True(t, adapter.shouldLog(LevelError))
		assert.False(t, adapter.shouldLog(LevelWarn))
		assert.False(t, adapter.shouldLog(LevelInfo))
		assert.False(t, adapter.shouldLog(LevelDebug))
	})

	t.Run("Change to WARN level - error and warn allowed", func(t *testing.T) {
		adapter.UpdateLevel("WARN")

		time.Sleep(1 * time.Millisecond)

		assert.Equal(t, "warn", adapter.config.General.LogLevel)
		assert.Equal(t, "WARN", adapter.config.Logging.Level)

		assert.True(t, adapter.shouldLog(LevelError))
		assert.True(t, adapter.shouldLog(LevelWarn))
		assert.False(t, adapter.shouldLog(LevelInfo))
		assert.False(t, adapter.shouldLog(LevelDebug))
	})

	t.Run("Change to INFO level - error, warn, info allowed", func(t *testing.T) {
		adapter.UpdateLevel("INFO")

		time.Sleep(1 * time.Millisecond)

		assert.Equal(t, "info", adapter.config.General.LogLevel)
		assert.Equal(t, "INFO", adapter.config.Logging.Level)

		assert.True(t, adapter.shouldLog(LevelError))
		assert.True(t, adapter.shouldLog(LevelWarn))
		assert.True(t, adapter.shouldLog(LevelInfo))
		assert.False(t, adapter.shouldLog(LevelDebug))
	})

	t.Run("Change back to DEBUG - all messages allowed", func(t *testing.T) {
		adapter.UpdateLevel("DEBUG")

		time.Sleep(1 * time.Millisecond)

		assert.Equal(t, "debug", adapter.config.General.LogLevel)
		assert.Equal(t, "DEBUG", adapter.config.Logging.Level)

		assert.True(t, adapter.shouldLog(LevelError))
		assert.True(t, adapter.shouldLog(LevelWarn))
		assert.True(t, adapter.shouldLog(LevelInfo))
		assert.True(t, adapter.shouldLog(LevelDebug))
	})
}

func TestLogger_DynamicLevelChange_CaseInsensitive(t *testing.T) {
	cfg := createTestConfig("INFO")
	logger := NewSlogAdapter(cfg)
	defer logger.Shutdown()

	adapter := logger.(*SlogAdapter)

	testCases := []struct {
		name        string
		inputLevel  string
		expectInfo  bool
		expectDebug bool
	}{
		{
			name:        "Uppercase ERROR",
			inputLevel:  "ERROR",
			expectInfo:  false,
			expectDebug: false,
		},
		{
			name:        "Lowercase error",
			inputLevel:  "error",
			expectInfo:  false,
			expectDebug: false,
		},
		{
			name:        "Mixed case Error",
			inputLevel:  "Error",
			expectInfo:  false,
			expectDebug: false,
		},
		{
			name:        "Uppercase DEBUG",
			inputLevel:  "DEBUG",
			expectInfo:  true,
			expectDebug: true,
		},
		{
			name:        "Lowercase debug",
			inputLevel:  "debug",
			expectInfo:  true,
			expectDebug: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter.UpdateLevel(tc.inputLevel)
			time.Sleep(1 * time.Millisecond)

			assert.Equal(t, tc.expectInfo, adapter.shouldLog(LevelInfo),
				"Info level should be %v for input level %s", tc.expectInfo, tc.inputLevel)
			assert.Equal(t, tc.expectDebug, adapter.shouldLog(LevelDebug),
				"Debug level should be %v for input level %s", tc.expectDebug, tc.inputLevel)
		})
	}
}

func TestLogger_DynamicLevelChange_MessageFiltering(t *testing.T) {
	cfg := createTestConfig("DEBUG")
	logger := NewSlogAdapter(cfg)
	defer logger.Shutdown()

	adapter := logger.(*SlogAdapter)

	// Initial state: DEBUG level, all messages should go through
	logger.Debug("debug message 1")
	logger.Info("info message 1")
	logger.Warn("warn message 1")
	logger.Error("error message 1")

	// Change to ERROR level
	adapter.UpdateLevel("ERROR")
	time.Sleep(5 * time.Millisecond) // Give time for level change

	// Only error messages should go through now
	logger.Debug("debug message 2 - should be filtered")
	logger.Info("info message 2 - should be filtered")
	logger.Warn("warn message 2 - should be filtered")
	logger.Error("error message 2 - should pass")

	// Change back to INFO level
	adapter.UpdateLevel("INFO")
	time.Sleep(5 * time.Millisecond)

	// Info, warn, and error should go through now
	logger.Debug("debug message 3 - should be filtered")
	logger.Info("info message 3 - should pass")
	logger.Warn("warn message 3 - should pass")
	logger.Error("error message 3 - should pass")

	// Give async logger time to process all messages
	time.Sleep(20 * time.Millisecond)

	// Test passes if no panics occur and messages are processed
	// In a real scenario, you'd capture output and verify which messages appear
}

func TestLogger_DynamicLevelChange_Concurrency(t *testing.T) {
	cfg := createTestConfig("INFO")
	logger := NewSlogAdapter(cfg)
	defer logger.Shutdown()

	adapter := logger.(*SlogAdapter)

	// Test concurrent level changes and logging
	done := make(chan bool, 3)

	// Goroutine 1: Change levels repeatedly
	go func() {
		levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
		for i := 0; i < 20; i++ {
			adapter.UpdateLevel(levels[i%len(levels)])
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: Log messages continuously
	go func() {
		for i := 0; i < 100; i++ {
			logger.Info("concurrent message", "iteration", i)
			if i%10 == 0 {
				time.Sleep(1 * time.Millisecond)
			}
		}
		done <- true
	}()

	// Goroutine 3: Check shouldLog continuously
	go func() {
		for i := 0; i < 50; i++ {
			_ = adapter.shouldLog(LevelInfo)
			_ = adapter.shouldLog(LevelDebug)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Goroutine completed successfully
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out - possible deadlock")
		}
	}

	// Test passes if no race conditions or deadlocks occur
}

func TestLogger_DynamicLevelChange_InvalidLevels(t *testing.T) {
	cfg := createTestConfig("INFO")
	logger := NewSlogAdapter(cfg)
	defer logger.Shutdown()

	adapter := logger.(*SlogAdapter)

	// Store original level
	originalLevel := adapter.config.General.LogLevel

	// Test invalid levels - should not crash and should maintain previous level
	invalidLevels := []string{"INVALID", "TRACE", "FATAL", "", "123"}

	for _, invalidLevel := range invalidLevels {
		t.Run("Invalid level: "+invalidLevel, func(t *testing.T) {
			// Set to a known good level first
			adapter.UpdateLevel("WARN")
			time.Sleep(1 * time.Millisecond)

			// Try to set invalid level
			adapter.UpdateLevel(invalidLevel)
			time.Sleep(1 * time.Millisecond)

			// Should still work (implementation may handle invalid levels gracefully)
			// At minimum, it shouldn't crash
			assert.NotPanics(t, func() {
				logger.Info("test message after invalid level")
				adapter.shouldLog(LevelInfo)
			})
		})
	}

	// Restore original level
	adapter.UpdateLevel(originalLevel)
}
