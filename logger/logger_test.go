package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewSlogLogger(t *testing.T) {
	logger := NewSlogLogger(slog.LevelInfo)
	if logger == nil {
		t.Fatal("NewSlogLogger() returned nil")
	}
	if logger.logger == nil {
		t.Error("NewSlogLogger() logger field is nil")
	}
}

func TestNewTextLogger(t *testing.T) {
	logger := NewTextLogger(slog.LevelDebug)
	if logger == nil {
		t.Fatal("NewTextLogger() returned nil")
	}
	if logger.logger == nil {
		t.Error("NewTextLogger() logger field is nil")
	}
}

func TestNewNoOpLogger(t *testing.T) {
	logger := NewNoOpLogger()
	if logger == nil {
		t.Fatal("NewNoOpLogger() returned nil")
	}
}

func TestSlogLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelError}
	handler := slog.NewJSONHandler(&buf, opts)
	logger := &SlogLogger{
		logger: slog.New(handler),
	}

	logger.Error("test error", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Errorf("Error() output doesn't contain message: %s", output)
	}
	if !strings.Contains(output, "ERROR") {
		t.Errorf("Error() output doesn't contain level: %s", output)
	}
	if !strings.Contains(output, "key") {
		t.Errorf("Error() output doesn't contain key: %s", output)
	}
}

func TestSlogLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	handler := slog.NewJSONHandler(&buf, opts)
	logger := &SlogLogger{
		logger: slog.New(handler),
	}

	logger.Warn("test warning", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test warning") {
		t.Errorf("Warn() output doesn't contain message: %s", output)
	}
	if !strings.Contains(output, "WARN") {
		t.Errorf("Warn() output doesn't contain level: %s", output)
	}
}

func TestSlogLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(&buf, opts)
	logger := &SlogLogger{
		logger: slog.New(handler),
	}

	logger.Info("test info", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test info") {
		t.Errorf("Info() output doesn't contain message: %s", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("Info() output doesn't contain level: %s", output)
	}
}

func TestSlogLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewJSONHandler(&buf, opts)
	logger := &SlogLogger{
		logger: slog.New(handler),
	}

	logger.Debug("test debug", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test debug") {
		t.Errorf("Debug() output doesn't contain message: %s", output)
	}
	if !strings.Contains(output, "DEBUG") {
		t.Errorf("Debug() output doesn't contain level: %s", output)
	}
}

func TestSlogLogger_With(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(&buf, opts)
	logger := &SlogLogger{
		logger: slog.New(handler),
	}

	childLogger := logger.With("component", "test")
	if childLogger == nil {
		t.Fatal("With() returned nil")
	}

	// Cast to SlogLogger to access logger field
	slogChild, ok := childLogger.(*SlogLogger)
	if !ok {
		t.Fatal("With() did not return *SlogLogger")
	}

	slogChild.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "component") {
		t.Errorf("With() context not included in output: %s", output)
	}
	if !strings.Contains(output, "test") {
		t.Errorf("With() context value not included in output: %s", output)
	}
}

func TestSlogLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	handler := slog.NewJSONHandler(&buf, opts)
	logger := &SlogLogger{
		logger: slog.New(handler),
	}

	// Debug and Info should be filtered out
	logger.Debug("debug message")
	logger.Info("info message")

	debugOutput := buf.String()
	if strings.Contains(debugOutput, "debug message") {
		t.Error("Debug message should be filtered out at Warn level")
	}
	if strings.Contains(debugOutput, "info message") {
		t.Error("Info message should be filtered out at Warn level")
	}

	// Warn should pass through
	logger.Warn("warn message")
	warnOutput := buf.String()
	if !strings.Contains(warnOutput, "warn message") {
		t.Error("Warn message should not be filtered out")
	}
}

func TestNoOpLogger_AllMethods(t *testing.T) {
	logger := NewNoOpLogger()

	// These should not panic
	logger.Error("error")
	logger.Warn("warn")
	logger.Info("info")
	logger.Debug("debug")

	child := logger.With("key", "value")
	if child == nil {
		t.Error("With() returned nil")
	}

	// Child logger should also work
	child.Error("error")
	child.Warn("warn")
	child.Info("info")
	child.Debug("debug")
}

func TestNoOpLogger_With(t *testing.T) {
	logger := NewNoOpLogger()
	child := logger.With("key", "value")

	// Should return the same logger instance
	if child != logger {
		t.Error("NoOpLogger.With() should return the same instance")
	}
}

func TestTextLogger_Output(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewTextHandler(&buf, opts)
	logger := &SlogLogger{
		logger: slog.New(handler),
	}

	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("TextLogger output doesn't contain message: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("TextLogger output doesn't contain key=value: %s", output)
	}
}

func TestSlogLogger_MultipleArgs(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(&buf, opts)
	logger := &SlogLogger{
		logger: slog.New(handler),
	}

	logger.Info("test", "key1", "value1", "key2", "value2", "key3", 123)

	output := buf.String()
	if !strings.Contains(output, "key1") || !strings.Contains(output, "value1") {
		t.Errorf("Output doesn't contain key1/value1: %s", output)
	}
	if !strings.Contains(output, "key2") || !strings.Contains(output, "value2") {
		t.Errorf("Output doesn't contain key2/value2: %s", output)
	}
	if !strings.Contains(output, "key3") || !strings.Contains(output, "123") {
		t.Errorf("Output doesn't contain key3/123: %s", output)
	}
}
