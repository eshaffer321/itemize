package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/sync"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/cli"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/logging"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	providerName := os.Args[1]
	// Shift args for flag parsing
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	// Parse common flags
	flags := cli.ParseSyncFlags()

	// Load config
	cfg := config.LoadOrEnv()

	// Initialize shared dependencies
	serviceClients, err := clients.NewClients(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize clients: %v", err)
	}

	store, err := storage.NewStorage(cfg.Storage.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Create provider based on subcommand
	var provider providers.OrderProvider
	switch providerName {
	case "costco":
		provider, err = cli.NewCostcoProvider(cfg, flags.Verbose)
	case "walmart":
		provider, err = cli.NewWalmartProvider(cfg, flags.Verbose)
	case "amazon":
		provider, err = cli.NewAmazonProvider(cfg, flags.Verbose)
	default:
		fmt.Printf("Unknown provider: %s\n", providerName)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Print header
	cli.PrintHeader(provider.DisplayName(), flags.DryRun)

	// Print database info
	fmt.Printf("Database: %s\n", cfg.Storage.DatabasePath)

	// Print configuration
	cli.PrintConfiguration(provider.DisplayName(), flags.LookbackDays, flags.MaxOrders, flags.Force)

	// Create orchestrator with sync-scoped logger and run
	opts := flags.ToSyncOptions()
	loggingCfg := cfg.Observability.Logging
	if flags.Verbose {
		loggingCfg.Level = "debug"
	}
	syncLogger := logging.NewLoggerWithSystem(loggingCfg, "sync")
	orchestrator := sync.NewOrchestrator(provider, serviceClients, store, syncLogger)
	result, err := orchestrator.Run(ctx, opts)

	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}

	// Print results
	cli.PrintSyncSummary(result, store, flags.DryRun)
}

func printUsage() {
	fmt.Println("Usage: monarch-sync <provider> [flags]")
	fmt.Println()
	fmt.Println("Providers:")
	fmt.Println("  amazon      Sync Amazon orders")
	fmt.Println("  costco      Sync Costco orders")
	fmt.Println("  walmart     Sync Walmart orders")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -dry-run         Run without making changes")
	fmt.Println("  -days int        Number of days to look back (default 14)")
	fmt.Println("  -max int         Maximum orders to process (default 0 = all)")
	fmt.Println("  -force           Force reprocess already processed orders")
	fmt.Println("  -verbose         Verbose output")
	fmt.Println("  -order-id string Process only this specific order ID (limits blast radius)")
}
