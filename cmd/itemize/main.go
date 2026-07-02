package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/eshaffer321/itemize/internal/adapters/clients"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/eshaffer321/itemize/internal/application/sync"
	"github.com/eshaffer321/itemize/internal/cli"
	"github.com/eshaffer321/itemize/internal/infrastructure/config"
	"github.com/eshaffer321/itemize/internal/infrastructure/logging"
	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/eshaffer321/itemize/internal/infrastructure/telemetry"
	"github.com/eshaffer321/itemize/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Handle version reporting separately, both as a subcommand
	// (`itemize version`) and as a flag (`itemize -version`/`--version`),
	// so a stale local binary is obvious without a git log spelunk.
	if command == "version" || command == "-version" || command == "--version" {
		fmt.Println(version.String())
		return
	}

	// Handle serve command separately
	if command == "serve" {
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		cfg := config.LoadOrEnv()
		flags := cli.ParseServeFlags()
		if err := cli.RunServe(cfg, flags); err != nil {
			telemetry.CaptureError(err, "serve", "serve")
			log.Fatalf("Server error: %v", err)
		}
		return
	}

	flush := telemetry.Init()
	defer flush()

	// Provider sync commands
	providerName := command
	// Shift args for flag parsing
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	// Parse common flags
	flags := cli.ParseSyncFlags()

	// Load config
	cfg := config.LoadOrEnv()

	// Initialize shared dependencies
	serviceClients, err := clients.NewClients(cfg)
	if err != nil {
		telemetry.CaptureError(err, providerName, "init")
		log.Fatalf("Failed to initialize clients: %v", err)
	}

	store, err := storage.NewStorage(cfg.Storage.DatabasePath)
	if err != nil {
		telemetry.CaptureError(err, providerName, "init")
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
		telemetry.CaptureError(err, providerName, "provider_create")
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
		telemetry.CaptureError(err, providerName, "sync")
		log.Fatalf("Sync failed: %v", err)
	}

	// Print results
	cli.PrintSyncSummary(result, store, flags.DryRun)
	telemetry.CaptureSync(providerName, flags, result)
}

func printUsage() {
	fmt.Println("Usage: itemize <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve       Start the API server")
	fmt.Println("  amazon      Sync Amazon orders")
	fmt.Println("  costco      Sync Costco orders")
	fmt.Println("  walmart     Sync Walmart orders")
	fmt.Println("  version     Print version, commit, and build date (also: -version, --version)")
	fmt.Println()
	fmt.Println("Serve Flags:")
	fmt.Println("  -port int        Port to listen on (default 8080)")
	fmt.Println("  -verbose         Verbose output")
	fmt.Println()
	fmt.Println("Sync Flags:")
	fmt.Println("  -dry-run         Run without making changes")
	fmt.Println("  -days int        Number of days to look back (default 14)")
	fmt.Println("  -max int         Maximum orders to process (default 0 = all)")
	fmt.Println("  -force           Force reprocess already processed orders")
	fmt.Println("  -verbose         Verbose output")
	fmt.Println("  -order-id string Process only this specific order ID (limits blast radius)")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  MONARCH_TOKEN              Monarch API token (required)")
	fmt.Println("  OPENAI_API_KEY             OpenAI API key")
	fmt.Println("  ANTHROPIC_API_KEY          Anthropic Claude API key")
	fmt.Println("  CATEGORIZER_PROVIDER       Force backend: 'openai' or 'anthropic'")
	fmt.Println("  ITEMIZE_NO_TELEMETRY       Set to 1 to disable anonymous usage telemetry")
	fmt.Println()
	fmt.Println("Provider-Specific Environment Variables:")
	fmt.Println("  AMAZON_ACCOUNT_NAME        Amazon browser profile name (optional)")
	fmt.Println("                             Run 'amazon-scraper --login --profile <name>' first")
}
