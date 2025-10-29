package sync

import (
	"log/slog"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/matcher"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/splitter"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/storage"
)

// Options holds sync configuration
type Options struct {
	DryRun       bool
	LookbackDays int
	MaxOrders    int
	Force        bool
	Verbose      bool
	OrderID      string // If set, only process this specific order (for testing)
}

// Result holds sync results
type Result struct {
	ProcessedCount int
	SkippedCount   int
	ErrorCount     int
	Errors         []error
}

// Orchestrator runs the sync process
type Orchestrator struct {
	provider     providers.OrderProvider
	clients      *clients.Clients
	splitter     *splitter.Splitter
	matcher      *matcher.Matcher
	consolidator *Consolidator
	storage      *storage.Storage
	logger       *slog.Logger
}

// NewOrchestrator creates a new sync orchestrator
func NewOrchestrator(
	provider providers.OrderProvider,
	clients *clients.Clients,
	storage *storage.Storage,
	logger *slog.Logger,
) *Orchestrator {
	// Create splitter with categorizer from clients (if available)
	var spl *splitter.Splitter
	if clients != nil && clients.Categorizer != nil {
		spl = splitter.NewSplitter(clients.Categorizer)
	}

	// Create matcher with standard config (reused across all orders)
	matcherConfig := matcher.Config{
		AmountTolerance: 0.01,
		DateTolerance:   5,
	}
	transactionMatcher := matcher.NewMatcher(matcherConfig)

	// Create consolidator (if clients available)
	var consolidator *Consolidator
	if clients != nil && clients.Monarch != nil {
		consolidator = NewConsolidator(clients.Monarch, logger)
	}

	return &Orchestrator{
		provider:     provider,
		clients:      clients,
		splitter:     spl,
		matcher:      transactionMatcher,
		consolidator: consolidator,
		storage:      storage,
		logger:       logger,
	}
}
