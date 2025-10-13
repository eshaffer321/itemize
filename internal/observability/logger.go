// Package observability provides simple logging utilities.
//
// This is a minimal implementation focused on structured logging with slog.
// Metrics and tracing can be added later when needed.
package observability

import (
	"log/slog"
	"os"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/config"
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

	// Choose format
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
