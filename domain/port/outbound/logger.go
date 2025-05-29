package outbound

// Logger defines the interface for structured logging operations.
// Methods are designed to be asynchronous to avoid hot path pollution.
type Logger interface {
	// logs messages with optional structured arguments
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
}
