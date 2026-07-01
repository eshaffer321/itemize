package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/eshaffer321/itemize/internal/application/sync"
)

// SyncFlags are common flags for all sync commands
type SyncFlags struct {
	DryRun       bool
	LookbackDays int
	MaxOrders    int
	Force        bool
	Verbose      bool
	OrderID      string
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
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Provider-Specific Environment Variables:")
		fmt.Fprintln(os.Stderr, "  AMAZON_ACCOUNT_NAME        Amazon browser profile name (optional)")
		fmt.Fprintln(os.Stderr, "                             Run 'amazon-scraper --login --profile <name>' first")
	}

	flag.Parse()
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
