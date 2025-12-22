// Package logging provides structured logging utilities.
//
// Logs are formatted in Maven-style with colors:
// [LEVEL] [SYSTEM] [HH:MM:SS] message key=value
package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
)

// logFile holds the open log file handle for cleanup
var logFile *os.File

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

	// Determine output writer
	var writer io.Writer = os.Stdout

	if cfg.FilePath != "" {
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fall back to stdout only, log warning to stderr
			_, _ = os.Stderr.WriteString("Warning: could not open log file " + cfg.FilePath + ": " + err.Error() + "\n")
		} else {
			logFile = file
			writer = io.MultiWriter(os.Stdout, file)
		}
	}

	// Use Maven-style handler for better readability
	handler := NewMavenHandler(writer, opts)

	return slog.New(handler)
}

// CloseLogFile closes the log file if one was opened
func CloseLogFile() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

// NewLoggerWithSystem creates a logger with a system prefix (e.g., "sync", "costco", "walmart")
// This is useful for creating scoped loggers that can be injected into external libraries
func NewLoggerWithSystem(cfg config.LoggingConfig, system string) *slog.Logger {
	logger := NewLogger(cfg)
	return logger.With("system", system)
}
