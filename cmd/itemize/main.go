package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

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
	amazonSetup := providerName == "amazon" && len(os.Args) > 2 && os.Args[2] == "setup"
	amazonReturns := providerName == "amazon" && len(os.Args) > 2 && os.Args[2] == "returns"
	// Shift args for flag parsing
	if amazonSetup || amazonReturns {
		os.Args = append([]string{os.Args[0]}, os.Args[3:]...)
	} else if providerName == "amazon" && len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") {
		os.Args = append([]string{os.Args[0], "-account", os.Args[2]}, os.Args[3:]...)
	} else {
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	// Parse common flags
	flags := cli.ParseSyncFlags(providerName)

	// Load config
	cfg := config.LoadOrEnv()
	if flags.CookieFile != "" {
		cfg.Providers.Amazon.CookieFile = flags.CookieFile
	}

	var err error
	if providerName != "amazon" && len(flags.ExtraArgs) > 0 {
		log.Fatalf("Unexpected positional arguments are only supported for the amazon provider")
	}

	amazonAccount := ""
	if providerName == "amazon" {
		if amazonSetup {
			amazonAccount, err = cli.ResolveAmazonSetupAccount(flags.Account, flags.ExtraArgs)
		} else {
			amazonAccount, err = cli.ResolveAmazonAccount(cfg, flags.Account, flags.ExtraArgs)
		}
		if err != nil {
			log.Fatalf("Invalid Amazon account arguments: %v", err)
		}
	}

	if flags.ListAccounts {
		if providerName != "amazon" {
			fmt.Printf("-list-accounts is only supported for the amazon provider\n")
			os.Exit(1)
		}
		if len(flags.ExtraArgs) > 0 {
			log.Fatalf("-list-accounts does not accept account arguments; use -account when syncing")
		}
		accounts, err := cli.ListAmazonAccounts(cfg)
		if err != nil {
			log.Fatalf("Failed to list Amazon accounts: %v", err)
		}
		if len(accounts) == 0 {
			fmt.Println("No saved Amazon accounts found.")
			fmt.Println("Run 'itemize amazon setup -account <name>' to create one.")
			return
		}
		fmt.Println("Saved Amazon accounts:")
		for _, account := range accounts {
			fmt.Printf("  %s\n", account)
		}
		fmt.Println()
		fmt.Println("Use with: itemize amazon -account <name>")
		return
	}

	if amazonSetup {
		if flags.ImportBrowserProfile != "" {
			log.Fatalf("amazon setup creates its own browser profile; omit -import-browser-profile")
		}
		if err := cli.RunAmazonSetup(cfg, cli.AmazonImportOptions{
			Account:        amazonAccount,
			CookieFile:     cfg.Providers.Amazon.CookieFile,
			PlaywrightRoot: flags.PlaywrightRoot,
			Headless:       flags.Headless,
			SkipAuthCheck:  flags.SkipAuthCheck,
		}); err != nil {
			log.Fatalf("Amazon setup failed: %v", err)
		}
		return
	}

	if amazonReturns {
		provider, createErr := cli.NewAmazonProvider(cfg, flags.Verbose, amazonAccount)
		if createErr != nil {
			log.Fatalf("Failed to create Amazon provider: %v", createErr)
		}
		returns, fetchErr := provider.FetchReturns(context.Background())
		if fetchErr != nil {
			log.Fatalf("Failed to fetch Amazon returns: %v", fetchErr)
		}
		if printErr := cli.PrintAmazonReturns(os.Stdout, returns); printErr != nil {
			log.Fatalf("Failed to print Amazon returns: %v", printErr)
		}
		return
	}

	if flags.ImportBrowserProfile != "" {
		if providerName != "amazon" {
			fmt.Printf("-import-browser-profile is only supported for the amazon provider\n")
			os.Exit(1)
		}
		if err := cli.RunAmazonBrowserProfileImport(cfg, cli.AmazonImportOptions{
			ProfileDir:     flags.ImportBrowserProfile,
			Account:        amazonAccount,
			CookieFile:     cfg.Providers.Amazon.CookieFile,
			PlaywrightRoot: flags.PlaywrightRoot,
			Headless:       flags.Headless,
			SkipAuthCheck:  flags.SkipAuthCheck,
		}); err != nil {
			// Auth import failures can include local browser profile paths.
			// Keep them on the user's terminal instead of sending telemetry.
			log.Fatalf("Amazon authentication failed: %v", err)
		}
		return
	}

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
		provider, err = cli.NewAmazonProvider(cfg, flags.Verbose, amazonAccount)
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

	// Print configuration (shows the resolved Amazon account, if any, so it's
	// obvious which profile is in use without digging through env vars)
	resolvedAccount := ""
	if providerName == "amazon" {
		resolvedAccount = amazonAccount
		if resolvedAccount == "" {
			resolvedAccount = "default"
		}
	}
	cli.PrintConfiguration(provider.DisplayName(), flags.LookbackDays, flags.MaxOrders, flags.Force, resolvedAccount)

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
	fmt.Println("  amazon setup -account <name>")
	fmt.Println("              Create an Amazon account and open Chromium for sign-in")
	fmt.Println("  amazon returns -account <name>")
	fmt.Println("              Read Amazon's return/refund ledger as JSON")
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
	fmt.Println("  -account string  Amazon cookie account name (amazon only)")
	fmt.Println("  -cookie-file string")
	fmt.Println("                  Explicit Amazon cookie file (amazon only)")
	fmt.Println("  -list-accounts   List saved Amazon cookie accounts and exit (amazon only)")
	fmt.Println()
	fmt.Println("Advanced Amazon Authentication:")
	fmt.Println("  -import-browser-profile string")
	fmt.Println("                  Import Amazon cookies from a Chromium/Playwright profile and exit (amazon only)")
	fmt.Println("  -playwright-root string")
	fmt.Println("                  Directory containing node_modules/playwright for Amazon cookie import")
	fmt.Println("  -headless        Run Amazon browser profile import headlessly")
	fmt.Println("  -skip-auth-check Skip Amazon auth validation after importing cookies")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  MONARCH_TOKEN              Monarch API token (required)")
	fmt.Println("  OPENAI_API_KEY             OpenAI API key")
	fmt.Println("  ANTHROPIC_API_KEY          Anthropic Claude API key")
	fmt.Println("  CATEGORIZER_PROVIDER       Force backend: 'openai' or 'anthropic'")
	fmt.Println("  ITEMIZE_NO_TELEMETRY       Set to 1 to disable anonymous usage telemetry")
	fmt.Println()
	fmt.Println("Provider-Specific Environment Variables:")
	fmt.Println("  AMAZON_ACCOUNT_NAME        Amazon cookie account name (optional)")
	fmt.Println("                             Run 'itemize amazon -import-browser-profile <profile-dir> -account <name>' first")
	fmt.Println("  AMAZON_COOKIE_FILE         Explicit amazon-go cookie file (optional)")
}
