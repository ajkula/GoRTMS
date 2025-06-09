package model

// Logger defines the interface for structured logging operations.
type Logger interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
	UpdateLevel(logLvl string)
	Shutdown()
}
