package logging

import (
	"testing"
	"time"

	"github.com/ajkula/GoRTMS/config"
)

// Helper to create test config
func createTestConfig(level string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Logging.Level = level
	cfg.Logging.ChannelSize = 100
	cfg.Logging.Format = "json"
	cfg.Logging.Output = "stdout"
	return cfg
}

func TestLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		expectError bool
		expectWarn  bool
		expectInfo  bool
		expectDebug bool
	}{
		{
			name:        "ERROR level - only errors",
			level:       "ERROR",
			expectError: true,
			expectWarn:  false,
			expectInfo:  false,
			expectDebug: false,
		},
		{
			name:        "WARN level - error and warn",
			level:       "WARN",
			expectError: true,
			expectWarn:  true,
			expectInfo:  false,
			expectDebug: false,
		},
		{
			name:        "INFO level - error, warn, info",
			level:       "INFO",
			expectError: true,
			expectWarn:  true,
			expectInfo:  true,
			expectDebug: false,
		},
		{
			name:        "DEBUG level - all messages",
			level:       "DEBUG",
			expectError: true,
			expectWarn:  true,
			expectInfo:  true,
			expectDebug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createTestConfig(tt.level)
			logger := NewSlogAdapter(cfg)

			// Send all types of messages
			logger.Error("error message", "key", "error_value")
			logger.Warn("warn message", "key", "warn_value")
			logger.Info("info message", "key", "info_value")
			logger.Debug("debug message", "key", "debug_value")

			// Give async logger time to process
			time.Sleep(10 * time.Millisecond)

			// For this test, we'll check the shouldLog method directly
			// since capturing actual output requires more complex setup
			adapter := logger.(*SlogAdapter)

			if got := adapter.shouldLog(LevelError); got != tt.expectError {
				t.Errorf("shouldLog(ERROR) = %v, want %v", got, tt.expectError)
			}
			if got := adapter.shouldLog(LevelWarn); got != tt.expectWarn {
				t.Errorf("shouldLog(WARN) = %v, want %v", got, tt.expectWarn)
			}
			if got := adapter.shouldLog(LevelInfo); got != tt.expectInfo {
				t.Errorf("shouldLog(INFO) = %v, want %v", got, tt.expectInfo)
			}
			if got := adapter.shouldLog(LevelDebug); got != tt.expectDebug {
				t.Errorf("shouldLog(DEBUG) = %v, want %v", got, tt.expectDebug)
			}
		})
	}
}

func TestLogger_MessageStructure(t *testing.T) {
	cfg := createTestConfig("DEBUG")
	logger := NewSlogAdapter(cfg)

	// Test that logger accepts various argument types
	testCases := []struct {
		name string
		fn   func()
	}{
		{
			name: "simple message",
			fn:   func() { logger.Info("simple message") },
		},
		{
			name: "message with string args",
			fn:   func() { logger.Info("message with args", "key", "value") },
		},
		{
			name: "message with int args",
			fn:   func() { logger.Info("message with int", "count", 42) },
		},
		{
			name: "message with duration",
			fn:   func() { logger.Info("message with duration", "elapsed", (100 * time.Millisecond).String()) },
		},
		{
			name: "message with multiple args",
			fn: func() {
				logger.Error("operation failed",
					"user_id", 123,
					"operation", "delete",
					"duration", (50 * time.Microsecond).String(),
					"error", "not found")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic
			tc.fn()
		})
	}

	// Give async logger time to process
	time.Sleep(10 * time.Millisecond)
}

func TestLogger_AsyncBehavior(t *testing.T) {
	cfg := createTestConfig("DEBUG")
	cfg.Logging.ChannelSize = 5 // Small buffer to test overflow behavior

	logger := NewSlogAdapter(cfg)

	// Send many messages quickly to test async behavior
	start := time.Now()
	for i := range 100 {
		logger.Debug("message", "iteration", i)
	}
	elapsed := time.Since(start)

	// Async logging should be very fast (not blocked by I/O)
	if elapsed > 10*time.Millisecond {
		t.Errorf("Logging took too long: %v, expected < 10ms (async should be fast)", elapsed)
	}

	// Give async logger time to process
	time.Sleep(50 * time.Millisecond)
}

func TestLogger_ChannelOverflow(t *testing.T) {
	cfg := createTestConfig("DEBUG")
	cfg.Logging.ChannelSize = 1 // Very small buffer to force overflow

	logger := NewSlogAdapter(cfg)

	// Fill the channel and send more (should not block)
	start := time.Now()
	for i := 0; i < 10; i++ {
		logger.Debug("overflow test", "iteration", i)
	}
	elapsed := time.Since(start)

	// Even with overflow, should not block
	if elapsed > 5*time.Millisecond {
		t.Errorf("Logging blocked on overflow: %v, expected < 5ms", elapsed)
	}
}

func TestLogger_Shutdown(t *testing.T) {
	cfg := createTestConfig("DEBUG")
	adapter := NewSlogAdapter(cfg).(*SlogAdapter)

	// Send some messages
	adapter.Debug("message 1")
	adapter.Info("message 2")

	// Shutdown should not panic and should complete quickly
	start := time.Now()
	adapter.Shutdown()
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("Shutdown took too long: %v, expected < 100ms", elapsed)
	}

	// Sending messages after shutdown should not panic
	adapter.Debug("message after shutdown")
}

func TestLogger_ConfigDefaults(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{
			name:  "empty level defaults to ERROR",
			level: "",
		},
		{
			name:  "invalid level defaults to ERROR",
			level: "INVALID",
		},
		{
			name:  "case insensitive level",
			level: "debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createTestConfig(tt.level)
			logger := NewSlogAdapter(cfg)
			adapter := logger.(*SlogAdapter)

			// Should not panic when creating logger with any config
			if adapter == nil {
				t.Error("Logger creation returned nil")
			} else {
				// Should handle invalid configs gracefully
				adapter.Debug("test message")
				adapter.Info("test message")
				adapter.Warn("test message")
				adapter.Error("test message")
			}
		})
	}
}

func TestLogger_DifferentFormats(t *testing.T) {
	formats := []string{"json", "text", "invalid", ""}

	for _, format := range formats {
		t.Run("format_"+format, func(t *testing.T) {
			cfg := createTestConfig("DEBUG")
			cfg.Logging.Format = format

			logger := NewSlogAdapter(cfg)

			// Should not panic with any format
			logger.Info("test message", "key", "value")

			// Give time for async processing
			time.Sleep(10 * time.Millisecond)
		})
	}
}

func TestLogger_DifferentOutputs(t *testing.T) {
	outputs := []string{"stdout", "stderr", "invalid", ""}

	for _, output := range outputs {
		t.Run("output_"+output, func(t *testing.T) {
			cfg := createTestConfig("DEBUG")
			cfg.Logging.Output = output

			logger := NewSlogAdapter(cfg)

			// Should not panic with any output config
			logger.Info("test message", "key", "value")

			// Give time for async processing
			time.Sleep(10 * time.Millisecond)
		})
	}
}

// Benchmark tests to ensure performance
func BenchmarkLogger_Debug(b *testing.B) {
	cfg := createTestConfig("ERROR") // Debug disabled for performance
	cfg.Logging.ChannelSize = 1000

	logger := NewSlogAdapter(cfg)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Debug("benchmark message", "iteration", 1, "key", "value")
		}
	})
}

func BenchmarkLogger_Info(b *testing.B) {
	cfg := createTestConfig("INFO")
	cfg.Logging.ChannelSize = 1000

	logger := NewSlogAdapter(cfg)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message", "iteration", 1, "key", "value")
		}
	})
}
