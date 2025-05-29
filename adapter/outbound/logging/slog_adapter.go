package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

type LogLevel int

const (
	LevelError LogLevel = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

// represents a single log entry to be processed asynchronously
type LogMessage struct {
	Level LogLevel
	Msg   string
	Args  []any
	Time  time.Time
}

// implements the Logger interface using Go's structured logging (slog)
// with asynchronous processing to avoid blocking hot paths
type SlogAdapter struct {
	logger  *slog.Logger
	config  *config.Config
	logChan chan LogMessage
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewSlogAdapter(config *config.Config) outbound.Logger {
	ctx, cancel := context.WithCancel(context.Background())

	adapter := &SlogAdapter{
		logger:  slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
		config:  config,
		logChan: make(chan LogMessage, config.Logging.ChannelSize),
		ctx:     ctx,
		cancel:  cancel,
	}

	go adapter.processLogs()

	return adapter
}

// hadles messages asynchronously
func (s *SlogAdapter) processLogs() {
	defer close(s.logChan)

	for {
		select {
		case msg := <-s.logChan:
			s.writeLog(msg)
		case <-s.ctx.Done():
			for len(s.logChan) > 0 {
				msg := <-s.logChan
				s.writeLog(msg)
			}
			return
		}
	}
}

// performs the logging operation
func (s *SlogAdapter) writeLog(msg LogMessage) {
	switch msg.Level {
	case LevelError:
		s.logger.Error(msg.Msg, msg.Args...)
	case LevelWarn:
		s.logger.Warn(msg.Msg, msg.Args...)
	case LevelInfo:
		s.logger.Info(msg.Msg, msg.Args...)
	case LevelDebug:
		s.logger.Debug(msg.Msg, msg.Args...)
	}
}

func (s *SlogAdapter) sendLog(level LogLevel, msg string, args ...any) {
	select {
	case s.logChan <- LogMessage{
		Level: level,
		Msg:   msg,
		Args:  args,
		Time:  time.Now(),
	}:
	default:
		// chan full
		// TODO: increase "dropped logs" stats
	}
}

func (s *SlogAdapter) shouldLog(level LogLevel) bool {
	switch strings.ToUpper(s.config.Logging.Level) {
	case "ERROR":
		return level == LevelError
	case "WARN":
		return level <= LevelWarn
	case "INFO":
		return level <= LevelInfo
	case "DEBUG":
		return level <= LevelDebug
	default:
		return level == LevelError
	}
}

func (s SlogAdapter) Error(msg string, args ...any) {
	if !s.shouldLog(LevelError) {
		return
	}
	s.sendLog(LevelError, msg, args...)
}

func (s *SlogAdapter) Warn(msg string, args ...any) {
	if !s.shouldLog(LevelWarn) {
		return
	}
	s.sendLog(LevelWarn, msg, args...)
}

func (s *SlogAdapter) Info(msg string, args ...any) {
	if !s.shouldLog(LevelInfo) {
		return
	}
	s.sendLog(LevelInfo, msg, args...)
}

func (s *SlogAdapter) Debug(msg string, args ...any) {
	if !s.shouldLog(LevelDebug) {
		return
	}
	s.sendLog(LevelDebug, msg, args...)
}

func (s *SlogAdapter) Shutdown() {
	s.cancel()
}
