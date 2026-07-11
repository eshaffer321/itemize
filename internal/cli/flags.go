package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/eshaffer321/itemize/internal/application/sync"
)

// SyncFlags are common flags for all sync commands
type SyncFlags struct {
	DryRun               bool
	LookbackDays         int
	MaxOrders            int
	Force                bool
	Verbose              bool
	OrderID              string
	Account              string
	CookieFile           string
	ListAccounts         bool
	ImportBrowserProfile string
	PlaywrightRoot       string
	Headless             bool
	SkipAuthCheck        bool
	ExtraArgs            []string
}

// ParseSyncFlags parses common sync flags from command line
func ParseSyncFlags() SyncFlags {
	var flags SyncFlags
	flag.BoolVar(&flags.DryRun, "dry-run", false, "Run without making changes")
	flag.IntVar(&flags.LookbackDays, "days", 14, "Number of days to look back")
	flag.IntVar(&flags.MaxOrders, "max", 0, "Maximum orders to process (0 = all)")
	flag.BoolVar(&flags.Force, "force", false, "Force reprocess already processed orders")
	flag.BoolVar(&flags.Verbose, "verbose", false, "Verbose output")
	flag.StringVar(&flags.OrderID, "order-id", "", "Process only this specific order ID (limits blast radius)")
	flag.StringVar(&flags.Account, "account", "", "Amazon cookie account name (overrides AMAZON_ACCOUNT_NAME; run -list-accounts to see saved accounts)")
	flag.StringVar(&flags.CookieFile, "cookie-file", "", "Explicit Amazon cookie file (overrides AMAZON_COOKIE_FILE)")
	flag.BoolVar(&flags.ListAccounts, "list-accounts", false, "List saved Amazon cookie accounts and exit")
	flag.StringVar(&flags.ImportBrowserProfile, "import-browser-profile", "", "Import Amazon cookies from this Chromium/Playwright browser profile and exit")
	flag.StringVar(&flags.PlaywrightRoot, "playwright-root", "", "Directory containing node_modules/playwright for Amazon cookie import")
	flag.BoolVar(&flags.Headless, "headless", false, "Run Amazon browser profile import headlessly")
	flag.BoolVar(&flags.SkipAuthCheck, "skip-auth-check", false, "Skip Amazon auth validation after importing cookies")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: itemize <command> [flags]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Sync Flags:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Environment Variables:")
		fmt.Fprintln(os.Stderr, "  MONARCH_TOKEN              Monarch API token (required)")
		fmt.Fprintln(os.Stderr, "  OPENAI_API_KEY             OpenAI API key")
		fmt.Fprintln(os.Stderr, "  ANTHROPIC_API_KEY          Anthropic Claude API key")
		fmt.Fprintln(os.Stderr, "  CATEGORIZER_PROVIDER       Force backend: 'openai' or 'anthropic'")
		fmt.Fprintln(os.Stderr, "  ITEMIZE_NO_TELEMETRY       Set to 1 to disable anonymous usage telemetry")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Provider-Specific Environment Variables:")
		fmt.Fprintln(os.Stderr, "  AMAZON_ACCOUNT_NAME        Amazon cookie account name (optional)")
		fmt.Fprintln(os.Stderr, "                             Run 'itemize amazon -import-browser-profile <profile-dir> -account <name>' first")
		fmt.Fprintln(os.Stderr, "  AMAZON_COOKIE_FILE         Explicit amazon-go cookie file (optional)")
	}

	flag.Parse()
	flags.ExtraArgs = flag.Args()
	return flags
}

// ToSyncOptions converts SyncFlags to sync.Options
func (f SyncFlags) ToSyncOptions() sync.Options {
	return sync.Options{
		DryRun:       f.DryRun,
		LookbackDays: f.LookbackDays,
		MaxOrders:    f.MaxOrders,
		Force:        f.Force,
		Verbose:      f.Verbose,
		OrderID:      f.OrderID,
	}
}
