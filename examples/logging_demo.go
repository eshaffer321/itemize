// Example demonstrating the Maven-style logging format
//
// Run with: go run examples/logging_demo.go
package main

import (
	"log/slog"
	"os"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/logging"
)

func main() {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "maven",
	}

	// Create loggers for different systems
	syncLogger := logging.NewLoggerWithSystem(cfg, "sync")
	costcoLogger := logging.NewLoggerWithSystem(cfg, "costco")
	monarchLogger := logging.NewLoggerWithSystem(cfg, "monarch")
	storageLogger := logging.NewLoggerWithSystem(cfg, "storage")

	// Simulate a sync operation
	syncLogger.Info("Starting sync", "provider", "Costco", "lookback_days", 17, "dry_run", false)

	costcoLogger.Info("Fetching online orders", "start_date", "2025-09-30", "end_date", "2025-10-17", "page_number", 1)
	costcoLogger.Info("Fetched online orders", "order_count", 6)

	monarchLogger.Info("Fetching Monarch transactions")
	monarchLogger.Info("Fetched transactions", "total", 126, "provider_transactions", 4)

	syncLogger.Info("Processing order", "index", 1, "total", 6)
	syncLogger.Info("Matched transaction", "order_id", "21134300600592510141205", "transaction_id", "224736066360937319", "amount", 284.46)
	syncLogger.Info("Multiple categories detected - creating splits", "order_id", "21134300600592510141205", "split_count", 4)
	monarchLogger.Info("Applying splits to Monarch", "transaction_id", "224736066360937319")
	syncLogger.Info("Successfully applied splits", "order_id", "21134300600592510141205")

	syncLogger.Warn("No matching transaction found", "order_id", "21134300600532510011111")

	// Example error
	storageLogger.Error("Failed to save record", "error", "database locked")

	// Debug example (won't show unless LOG_LEVEL=debug)
	debugLogger := slog.New(logging.NewMavenHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})).With("system", "debug")
	debugLogger.Debug("Detailed trace information", "step", 1, "state", "initialized")
}
