package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/observability"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/processor"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers/costco"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers/walmart"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/splitter"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
	costcogo "github.com/costco-go/pkg/costco"
	walmartclient "github.com/eshaffer321/walmart-client"
)

func main() {
	// Parse flags
	var (
		configFile   = flag.String("config", "", "Configuration file path")
		dryRun       = flag.Bool("dry-run", true, "Preview changes without applying")
		lookbackDays = flag.Int("days", 14, "Number of days to look back")
		maxOrders    = flag.Int("max", 0, "Maximum orders to process (0 = all)")
		force        = flag.Bool("force", false, "Force reprocess already processed orders")
		provider     = flag.String("provider", "", "Specific provider to sync (empty = all)")
		verbose      = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	// Setup logging
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	
	logger := observability.NewLogger(config.LoggingConfig{
		Level:  logLevel.String(),
		Format: "text",
	})

	// Load configuration
	cfg := loadConfig(*configFile, logger)

	// Initialize storage
	store, err := storage.NewStorage(cfg.Storage.DatabasePath)
	if err != nil {
		logger.Error("Failed to initialize storage", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer store.Close()

	// Initialize trace store
	traceStore := observability.NewTraceStore(24 * time.Hour)

	// Initialize clients
	monarchClient, err := monarch.NewClientWithToken(cfg.Monarch.APIKey)
	if err != nil {
		logger.Error("Failed to create Monarch client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize categorizer
	categorizerClient := categorizer.NewCategorizer(cfg.OpenAI.APIKey, logger)

	// Initialize splitter
	splitterClient := splitter.NewSplitter()

	// Create provider registry
	registry := providers.NewRegistry(logger)

	// Register enabled providers
	if cfg.Providers.Walmart != nil && cfg.Providers.Walmart.Enabled {
		walmartClient := walmartclient.NewWalmartClient()
		walmartProvider := walmart.NewProvider(walmartClient, logger)
		
		if err := registry.Register(walmartProvider); err != nil {
			logger.Error("Failed to register Walmart provider", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	// Register Costco provider
	if cfg.Providers.Costco != nil && cfg.Providers.Costco.Enabled {
		costcoConfig := costcogo.Config{
			Email:           cfg.Providers.Costco.Email,
			Password:        cfg.Providers.Costco.Password,
			WarehouseNumber: cfg.Providers.Costco.WarehouseNumber,
		}
		costcoClient := costcogo.NewClient(costcoConfig)
		costcoProvider := costco.NewProvider(costcoClient, logger)
		
		if err := registry.Register(costcoProvider); err != nil {
			logger.Error("Failed to register Costco provider", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	// Create processor
	proc := processor.New(
		registry,
		monarchClient,
		categorizerClient,
		splitterClient,
		store,
		logger,
	)

	// Processing configuration
	processorConfig := processor.Config{
		DryRun:       *dryRun,
		Force:        *force,
		LookbackDays: *lookbackDays,
		MaxOrders:    *maxOrders,
	}

	// Log configuration
	logger.Info("Starting sync",
		slog.Bool("dry_run", *dryRun),
		slog.Int("lookback_days", *lookbackDays),
		slog.Int("max_orders", *maxOrders),
		slog.Bool("force", *force),
		slog.String("provider", *provider),
		slog.Int("registered_providers", len(registry.List())),
	)

	// Create context with tracing
	ctx := context.Background()

	// Process based on provider flag
	if *provider != "" {
		// Process specific provider
		prov, err := registry.Get(*provider)
		if err != nil {
			logger.Error("Provider not found", 
				slog.String("provider", *provider),
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}

		// Create trace for this sync run
		trace := traceStore.StartTrace(*provider, fmt.Sprintf("sync-%d", time.Now().Unix()))
		ctx = observability.ContextWithTrace(ctx, trace)
		
		err = proc.ProcessProvider(ctx, prov, processorConfig)
		
		if err != nil {
			trace.Complete("failed", err)
			logger.Error("Processing failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		
		trace.Complete("success", nil)
		
	} else {
		// Process all providers
		err = proc.ProcessAllProviders(ctx, processorConfig)
		if err != nil {
			logger.Error("Processing failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	logger.Info("Sync completed successfully")
}

func loadConfig(configFile string, logger *slog.Logger) *config.Config {
	// If config file specified, load it
	if configFile != "" {
		cfg, err := config.Load(configFile)
		if err != nil {
			logger.Error("Failed to load config file", 
				slog.String("file", configFile),
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
		return cfg
	}

	// Otherwise, load from environment
	cfg := config.LoadFromEnv()
	
	// Validate required fields
	if cfg.Monarch.APIKey == "" {
		logger.Error("MONARCH_TOKEN environment variable not set")
		os.Exit(1)
	}
	
	if cfg.OpenAI.APIKey == "" {
		logger.Error("OPENAI_APIKEY environment variable not set")
		os.Exit(1)
	}
	
	return cfg
}