package cli

import (
	"flag"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/sync"
)

// SyncFlags are common flags for all sync commands
type SyncFlags struct {
	DryRun       bool
	LookbackDays int
	MaxOrders    int
	Force        bool
	Verbose      bool
}

// ParseSyncFlags parses common sync flags from command line
func ParseSyncFlags() SyncFlags {
	var flags SyncFlags
	flag.BoolVar(&flags.DryRun, "dry-run", false, "Run without making changes")
	flag.IntVar(&flags.LookbackDays, "days", 14, "Number of days to look back")
	flag.IntVar(&flags.MaxOrders, "max", 0, "Maximum orders to process (0 = all)")
	flag.BoolVar(&flags.Force, "force", false, "Force reprocess already processed orders")
	flag.BoolVar(&flags.Verbose, "verbose", false, "Verbose output")
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
	}
}
