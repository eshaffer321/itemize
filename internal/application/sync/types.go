package sync

import (
	"log/slog"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
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
	provider providers.OrderProvider
	clients  *clients.Clients
	splitter *splitter.Splitter
	storage  *storage.Storage
	logger   *slog.Logger
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

	return &Orchestrator{
		provider: provider,
		clients:  clients,
		splitter: spl,
		storage:  storage,
		logger:   logger,
	}
}
