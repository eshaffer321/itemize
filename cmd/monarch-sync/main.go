package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/config"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/observability"
)

// CLI represents the main CLI application
type CLI struct {
	configFile string
	verbose    bool
}

func main() {
	cli := &CLI{}

	// Global flags
	flag.StringVar(&cli.configFile, "config", "", "Configuration file path")
	flag.BoolVar(&cli.verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	// Get subcommand
	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	subArgs := args[1:]

	// Setup logging
	logLevel := "info"
	if cli.verbose {
		logLevel = "debug"
	}

	logger := observability.NewLogger(config.LoggingConfig{
		Level:  logLevel,
		Format: "text",
	})

	// Load configuration
	cfg := loadConfig(cli.configFile, logger)

	// Route to subcommand
	switch subcommand {
	case "costco":
		handleCostcoCommand(subArgs, cfg, logger)
	case "walmart":
		handleWalmartCommand(subArgs, cfg, logger)
	case "sync":
		handleSyncCommand(subArgs, cfg, logger)
	case "api":
		handleAPICommand(subArgs, cfg, logger)
	case "audit":
		handleAuditCommand(subArgs, cfg, logger)
	case "consolidate":
		handleConsolidateCommand(subArgs, cfg, logger)
	case "enrich":
		handleEnrichCommand(subArgs, cfg, logger)
	default:
		fmt.Printf("Unknown subcommand: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Monarch Money Sync CLI")
	fmt.Println("======================")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  monarch-sync <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  costco <action>     Costco provider commands")
	fmt.Println("    sync              Sync Costco orders")
	fmt.Println("    dry-run           Preview Costco sync without applying")
	fmt.Println()
	fmt.Println("  walmart <action>    Walmart provider commands")
	fmt.Println("    sync              Sync Walmart orders")
	fmt.Println()
	fmt.Println("  sync               General sync command (all providers)")
	fmt.Println("  api                Start API server")
	fmt.Println("  audit              Generate audit report")
	fmt.Println("  consolidate        Consolidate databases")
	fmt.Println("  enrich             Enrich processing history")
	fmt.Println()
	fmt.Println("Global Options:")
	fmt.Println("  -config string      Configuration file path")
	fmt.Println("  -verbose            Enable verbose logging")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  monarch-sync costco sync")
	fmt.Println("  monarch-sync costco dry-run")
	fmt.Println("  monarch-sync sync -config config.yaml")
	fmt.Println("  monarch-sync api")
	fmt.Println("  monarch-sync audit")
}

func loadConfig(configFile string, logger *slog.Logger) *config.Config {
	if configFile == "" {
		// Try to find config file
		candidates := []string{"config.yaml", "config.yml", "config.json"}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				configFile = candidate
				break
			}
		}
	}

	if configFile == "" {
		logger.Warn("No config file found, using environment variables")
		return config.LoadFromEnv()
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	return cfg
}

func handleCostcoCommand(args []string, cfg *config.Config, logger *slog.Logger) {
	if len(args) == 0 {
		fmt.Println("Costco commands:")
		fmt.Println("  sync     Sync Costco orders")
		fmt.Println("  dry-run  Preview Costco sync without applying")
		os.Exit(1)
	}

	action := args[0]
	switch action {
	case "sync":
		runCostcoSync(cfg, logger, false)
	case "dry-run":
		runCostcoSync(cfg, logger, true)
	default:
		fmt.Printf("Unknown Costco action: %s\n", action)
		fmt.Println("Available actions: sync, dry-run")
		os.Exit(1)
	}
}

func handleWalmartCommand(args []string, cfg *config.Config, logger *slog.Logger) {
	if len(args) == 0 {
		fmt.Println("Walmart commands:")
		fmt.Println("  sync     Sync Walmart orders")
		os.Exit(1)
	}

	action := args[0]
	switch action {
	case "sync":
		runWalmartSync(cfg, logger)
	default:
		fmt.Printf("Unknown Walmart action: %s\n", action)
		fmt.Println("Available actions: sync")
		os.Exit(1)
	}
}

func handleSyncCommand(args []string, cfg *config.Config, logger *slog.Logger) {
	// Parse flags for sync command
	var (
		dryRun       = flag.Bool("dry-run", true, "Preview changes without applying")
		lookbackDays = flag.Int("days", 14, "Number of days to look back")
		maxOrders    = flag.Int("max", 0, "Maximum orders to process (0 = all)")
		force        = flag.Bool("force", false, "Force reprocess already processed orders")
		provider     = flag.String("provider", "", "Specific provider to sync (empty = all)")
	)

	// Create a new flag set for sync subcommand
	syncFlags := flag.NewFlagSet("sync", flag.ExitOnError)
	syncFlags.BoolVar(dryRun, "dry-run", true, "Preview changes without applying")
	syncFlags.IntVar(lookbackDays, "days", 14, "Number of days to look back")
	syncFlags.IntVar(maxOrders, "max", 0, "Maximum orders to process (0 = all)")
	syncFlags.BoolVar(force, "force", false, "Force reprocess already processed orders")
	syncFlags.StringVar(provider, "provider", "", "Specific provider to sync (empty = all)")

	if err := syncFlags.Parse(args); err != nil {
		logger.Error("Failed to parse sync flags", "error", err)
		os.Exit(1)
	}

	runGeneralSync(cfg, logger, *dryRun, *lookbackDays, *maxOrders, *force, *provider)
}

func handleAPICommand(args []string, cfg *config.Config, logger *slog.Logger) {
	runAPIServer(cfg, logger)
}

func handleAuditCommand(args []string, cfg *config.Config, logger *slog.Logger) {
	runAuditReport(cfg, logger)
}

func handleConsolidateCommand(args []string, cfg *config.Config, logger *slog.Logger) {
	runConsolidateDB(cfg, logger)
}

func handleEnrichCommand(args []string, cfg *config.Config, logger *slog.Logger) {
	runEnrichHistory(cfg, logger)
}

// Import the actual command implementations
// These will be moved from their current locations

func runCostcoSync(cfg *config.Config, logger *slog.Logger, dryRun bool) {
	fmt.Println("🔄 Running Costco sync...")
	if dryRun {
		fmt.Println("   Mode: DRY RUN")
	} else {
		fmt.Println("   Mode: PRODUCTION")
	}

	// TODO: Import and call the actual costco-sync logic
	fmt.Println("   Costco sync functionality will be implemented here")
}

func runWalmartSync(cfg *config.Config, logger *slog.Logger) {
	fmt.Println("🔄 Running Walmart sync...")

	// TODO: Import and call the actual walmart-sync logic
	fmt.Println("   Walmart sync functionality will be implemented here")
}

func runGeneralSync(cfg *config.Config, logger *slog.Logger, dryRun bool, lookbackDays, maxOrders int, force bool, provider string) {
	fmt.Println("🔄 Running general sync...")
	fmt.Printf("   Dry run: %v\n", dryRun)
	fmt.Printf("   Lookback days: %d\n", lookbackDays)
	fmt.Printf("   Max orders: %d\n", maxOrders)
	fmt.Printf("   Force: %v\n", force)
	fmt.Printf("   Provider: %s\n", provider)

	// TODO: Import and call the actual sync logic
	fmt.Println("   General sync functionality will be implemented here")
}

func runAPIServer(cfg *config.Config, logger *slog.Logger) {
	fmt.Println("🚀 Starting API server...")

	// TODO: Import and call the actual API server logic
	fmt.Println("   API server functionality will be implemented here")
}

func runAuditReport(cfg *config.Config, logger *slog.Logger) {
	fmt.Println("📊 Generating audit report...")

	// TODO: Import and call the actual audit report logic
	fmt.Println("   Audit report functionality will be implemented here")
}

func runConsolidateDB(cfg *config.Config, logger *slog.Logger) {
	fmt.Println("🗄️  Consolidating databases...")

	// TODO: Import and call the actual consolidate logic
	fmt.Println("   Database consolidation functionality will be implemented here")
}

func runEnrichHistory(cfg *config.Config, logger *slog.Logger) {
	fmt.Println("📈 Enriching processing history...")

	// TODO: Import and call the actual enrich logic
	fmt.Println("   History enrichment functionality will be implemented here")
}
