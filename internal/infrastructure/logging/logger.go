// Package logging provides structured logging utilities.
//
// Logs are formatted in Maven-style with colors:
// [LEVEL] [SYSTEM] [HH:MM:SS] message key=value
package logging

import (
	"log/slog"
	"os"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
)

// NewLogger creates a structured logger based on config
func NewLogger(cfg config.LoggingConfig) *slog.Logger {
	// Parse log level
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Use Maven-style handler for better readability
	handler := NewMavenHandler(os.Stdout, opts)

	return slog.New(handler)
}

// NewLoggerWithSystem creates a logger with a system prefix (e.g., "sync", "costco", "walmart")
// This is useful for creating scoped loggers that can be injected into external libraries
func NewLoggerWithSystem(cfg config.LoggingConfig, system string) *slog.Logger {
	logger := NewLogger(cfg)
	return logger.With("system", system)
}
