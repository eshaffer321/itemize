package sync

import (
	"log/slog"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/clients"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/storage"
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
	return &Orchestrator{
		provider: provider,
		clients:  clients,
		storage:  storage,
		logger:   logger,
	}
}
