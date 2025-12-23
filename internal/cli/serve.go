package cli

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/logging"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// ServeFlags holds the CLI flags for the serve command.
type ServeFlags struct {
	Port    int
	Verbose bool
}

// ParseServeFlags parses command line flags for the serve command.
func ParseServeFlags() *ServeFlags {
	flags := &ServeFlags{}
	flag.IntVar(&flags.Port, "port", 8080, "Port to listen on")
	flag.BoolVar(&flags.Verbose, "verbose", false, "Verbose output")
	flag.Parse()
	return flags
}

// RunServe runs the API server.
func RunServe(cfg *config.Config, flags *ServeFlags) error {
	// Set up logging
	loggingCfg := cfg.Observability.Logging
	if flags.Verbose {
		loggingCfg.Level = "debug"
	}
	logger := logging.NewLoggerWithSystem(loggingCfg, "api")

	// Initialize storage
	store, err := storage.NewStorage(cfg.Storage.DatabasePath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	// Create API config
	apiCfg := api.Config{
		Port:           flags.Port,
		AllowedOrigins: []string{"http://localhost:3000", "http://localhost:5173"},
	}

	// Create and start server
	server := api.NewServer(apiCfg, store, logger)

	// Handle graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("received shutdown signal")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("server shutdown error", slog.Any("error", err))
		}
		close(done)
	}()

	// Start server (blocks until shutdown)
	if err := server.Start(); err != nil {
		return err
	}

	<-done
	logger.Info("server stopped")
	return nil
}
