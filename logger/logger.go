package logger

import (
	"context"
	"log/slog"
	"os"
)

// Logger interface defines structured logging methods
type Logger interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
	With(args ...any) Logger
}

// SlogLogger wraps slog.Logger to implement our Logger interface
type SlogLogger struct {
	logger *slog.Logger
	ctx    context.Context
}

// NewSlogLogger creates a new structured logger using slog
func NewSlogLogger(level slog.Level) *SlogLogger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &SlogLogger{
		logger: logger,
		ctx:    context.Background(),
	}
}

// NewTextLogger creates a text-based logger (more human-readable)
func NewTextLogger(level slog.Level) *SlogLogger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &SlogLogger{
		logger: logger,
		ctx:    context.Background(),
	}
}

func (l *SlogLogger) Error(msg string, args ...any) {
	l.logger.ErrorContext(l.ctx, msg, args...)
}

func (l *SlogLogger) Warn(msg string, args ...any) {
	l.logger.WarnContext(l.ctx, msg, args...)
}

func (l *SlogLogger) Info(msg string, args ...any) {
	l.logger.InfoContext(l.ctx, msg, args...)
}

func (l *SlogLogger) Debug(msg string, args ...any) {
	l.logger.DebugContext(l.ctx, msg, args...)
}

// With returns a new Logger with additional structured context
func (l *SlogLogger) With(args ...any) Logger {
	return &SlogLogger{
		logger: l.logger.With(args...),
		ctx:    l.ctx,
	}
}

// NoOpLogger implements Logger but does nothing (useful for testing)
type NoOpLogger struct{}

func (l *NoOpLogger) Error(msg string, args ...any) {}
func (l *NoOpLogger) Warn(msg string, args ...any)  {}
func (l *NoOpLogger) Info(msg string, args ...any)  {}
func (l *NoOpLogger) Debug(msg string, args ...any) {}
func (l *NoOpLogger) With(args ...any) Logger       { return l }

// NewNoOpLogger creates a logger that does nothing
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}
