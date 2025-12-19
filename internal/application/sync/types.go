package sync

import (
	"context"
	"log/slog"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/clients"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/application/sync/handlers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
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
	provider      providers.OrderProvider
	clients       *clients.Clients
	splitter      *splitter.Splitter
	matcher       *matcher.Matcher
	consolidator  *Consolidator
	amazonHandler *handlers.AmazonHandler
	storage       *storage.Storage
	logger        *slog.Logger
	runID         int64 // Current sync run ID for API logging
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
		consolidator = NewConsolidator(clients.Monarch, logger, storage, 0)
	}

	// Create Amazon handler (if all dependencies available)
	var amazonHandler *handlers.AmazonHandler
	if clients != nil && clients.Monarch != nil && spl != nil && consolidator != nil {
		amazonHandler = handlers.NewAmazonHandler(
			transactionMatcher,
			&consolidatorAdapter{consolidator},
			&splitterAdapter{spl},
			&monarchAdapter{clients.Monarch},
			logger,
		)
	}

	return &Orchestrator{
		provider:      provider,
		clients:       clients,
		splitter:      spl,
		matcher:       transactionMatcher,
		consolidator:  consolidator,
		amazonHandler: amazonHandler,
		storage:       storage,
		logger:        logger,
	}
}

// consolidatorAdapter wraps Consolidator to implement handlers.TransactionConsolidator
type consolidatorAdapter struct {
	consolidator *Consolidator
}

func (a *consolidatorAdapter) ConsolidateTransactions(ctx context.Context, transactions []*monarch.Transaction, order providers.Order, dryRun bool) (*handlers.ConsolidationResult, error) {
	result, err := a.consolidator.ConsolidateTransactions(ctx, transactions, order, dryRun)
	if err != nil {
		return nil, err
	}
	return &handlers.ConsolidationResult{
		ConsolidatedTransaction: result.ConsolidatedTransaction,
		FailedDeletions:         result.FailedDeletions,
	}, nil
}

// splitterAdapter wraps splitter.Splitter to implement handlers.CategorySplitter
type splitterAdapter struct {
	splitter *splitter.Splitter
}

func (a *splitterAdapter) CreateSplits(ctx context.Context, order providers.Order, transaction *monarch.Transaction, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory) ([]*monarch.TransactionSplit, error) {
	return a.splitter.CreateSplits(ctx, order, transaction, catCategories, monarchCategories)
}

func (a *splitterAdapter) GetSingleCategoryInfo(ctx context.Context, order providers.Order, categories []categorizer.Category) (string, string, error) {
	return a.splitter.GetSingleCategoryInfo(ctx, order, categories)
}

// monarchAdapter wraps monarch.Client to implement handlers.MonarchClient
type monarchAdapter struct {
	client *monarch.Client
}

func (a *monarchAdapter) UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error {
	_, err := a.client.Transactions.Update(ctx, id, params)
	return err
}

func (a *monarchAdapter) UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error {
	return a.client.Transactions.UpdateSplits(ctx, id, splits)
}
