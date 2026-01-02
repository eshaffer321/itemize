package cli

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/service"
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

	// Initialize clients for sync service
	var syncService *service.SyncService
	serviceClients, err := clients.NewClients(cfg)
	if err != nil {
		logger.Warn("failed to initialize clients, sync endpoints will be disabled", slog.Any("error", err))
	} else {
		// Create provider factory map
		providerFactory := map[string]service.ProviderFactory{
			"walmart": func(c *config.Config, verbose bool) (providers.OrderProvider, error) {
				return NewWalmartProvider(c, verbose)
			},
			"costco": func(c *config.Config, verbose bool) (providers.OrderProvider, error) {
				return NewCostcoProvider(c, verbose)
			},
			"amazon": func(c *config.Config, verbose bool) (providers.OrderProvider, error) {
				return NewAmazonProvider(c, verbose)
			},
		}

		// Create sync service
		syncService = service.NewSyncService(cfg, serviceClients, store, logger, providerFactory)

		// Start background cleanup for stale jobs (checks every 5 minutes)
		syncService.StartBackgroundCleanup(5 * time.Minute)

		logger.Info("sync service initialized", "providers", []string{"walmart", "costco", "amazon"})
	}

	// Create API config
	apiCfg := api.Config{
		Port:           flags.Port,
		AllowedOrigins: []string{"http://localhost:3000", "http://localhost:5173"},
	}

	// Get monarch client for transactions API (may be nil if client init failed)
	var monarchClient *monarch.Client
	if serviceClients != nil {
		monarchClient = serviceClients.Monarch
	}

	// Create and start server
	server := api.NewServer(apiCfg, store, syncService, monarchClient, logger)

	// Handle graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("received shutdown signal")

		// Stop background cleanup if sync service is running
		if syncService != nil {
			syncService.StopBackgroundCleanup()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("server shutdown error", slog.Any("error", err))
		}
		close(done)
	}()

	// Log server info
	if syncService != nil {
		fmt.Printf("Sync API available at http://localhost:%d/api/sync\n", flags.Port)
	} else {
		fmt.Printf("Sync API disabled (client initialization failed)\n")
	}

	// Start server (blocks until shutdown)
	if err := server.Start(); err != nil {
		return err
	}

	<-done
	logger.Info("server stopped")
	return nil
}
