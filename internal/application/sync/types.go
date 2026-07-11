package sync

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/clients"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/eshaffer321/itemize/internal/application/sync/handlers"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/itemize/internal/domain/matcher"
	"github.com/eshaffer321/itemize/internal/domain/splitter"
	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

// ProgressUpdate represents a progress update during sync
type ProgressUpdate struct {
	Phase           string // "fetching_orders", "processing_orders"
	TotalOrders     int
	ProcessedOrders int
	SkippedOrders   int
	ErroredOrders   int
}

// ProgressCallback is called to report progress during sync
type ProgressCallback func(update ProgressUpdate)

// Options holds sync configuration
type Options struct {
	DryRun           bool
	LookbackDays     int
	MaxOrders        int
	Force            bool
	Verbose          bool
	OrderID          string           // If set, only process this specific order (for testing)
	ProgressCallback ProgressCallback // Optional callback for progress updates
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
	provider       providers.OrderProvider
	clients        *clients.Clients
	splitter       *splitter.Splitter
	matcher        *matcher.Matcher
	consolidator   *Consolidator
	amazonHandler  *handlers.AmazonHandler
	walmartHandler *handlers.WalmartHandler
	simpleHandler  *handlers.SimpleHandler
	monarchAdapter *monarchAdapter
	storage        storage.Repository // Interface instead of concrete type
	logger         *slog.Logger
	runID          int64 // Current sync run ID for API logging
}

// NewOrchestrator creates a new sync orchestrator
func NewOrchestrator(
	provider providers.OrderProvider,
	clients *clients.Clients,
	store storage.Repository,
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
		consolidator = NewConsolidator(clients.Monarch, logger, store, 0)
	}

	var mAdapter *monarchAdapter
	if clients != nil && clients.Monarch != nil {
		mAdapter = &monarchAdapter{
			client:  clients.Monarch,
			storage: store,
			logger:  logger,
		}
	}

	// Create Amazon handler (if all dependencies available)
	var amazonHandler *handlers.AmazonHandler
	if clients != nil && clients.Monarch != nil && spl != nil && consolidator != nil {
		amazonHandler = handlers.NewAmazonHandler(
			transactionMatcher,
			&consolidatorAdapter{consolidator},
			&splitterAdapter{spl},
			mAdapter,
			logger,
		)
	}

	// Create Walmart handler (if all dependencies available)
	var walmartHandler *handlers.WalmartHandler
	if clients != nil && clients.Monarch != nil && spl != nil && consolidator != nil {
		walmartHandler = handlers.NewWalmartHandler(
			transactionMatcher,
			&consolidatorAdapter{consolidator},
			&splitterAdapter{spl},
			mAdapter,
			logger,
		)
	}

	// Create Simple handler for providers without special handling (Costco, etc.)
	var simpleHandler *handlers.SimpleHandler
	if clients != nil && clients.Monarch != nil && spl != nil {
		simpleHandler = handlers.NewSimpleHandler(
			transactionMatcher,
			&splitterAdapter{spl},
			mAdapter,
			logger,
		)
	}

	return &Orchestrator{
		provider:       provider,
		clients:        clients,
		splitter:       spl,
		matcher:        transactionMatcher,
		consolidator:   consolidator,
		amazonHandler:  amazonHandler,
		walmartHandler: walmartHandler,
		simpleHandler:  simpleHandler,
		monarchAdapter: mAdapter,
		storage:        store,
		logger:         logger,
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
	client  *monarch.Client
	storage storage.Repository
	logger  *slog.Logger
	runID   int64
}

func (a *monarchAdapter) UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error {
	start := time.Now()
	updated, err := a.client.Transactions.Update(ctx, id, params)
	a.logAPICall(ctx, id, "Transactions.Update", params, updated, err, time.Since(start))
	return err
}

func (a *monarchAdapter) UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error {
	start := time.Now()
	err := a.client.Transactions.UpdateSplits(ctx, id, splits)
	response := map[string]any{"ok": err == nil, "split_count": len(splits)}
	a.logAPICall(ctx, id, "Transactions.UpdateSplits", splits, response, err, time.Since(start))
	return err
}

func (a *monarchAdapter) logAPICall(ctx context.Context, transactionID, method string, request, response any, callErr error, duration time.Duration) {
	if a == nil || a.storage == nil {
		return
	}

	requestJSON, _ := json.Marshal(request)
	responseJSON, _ := json.Marshal(response)
	errText := ""
	if callErr != nil {
		errText = callErr.Error()
	}

	call := &storage.APICall{
		RunID:         a.runID,
		OrderID:       auditOrderID(ctx),
		TransactionID: transactionID,
		Method:        method,
		RequestJSON:   string(requestJSON),
		ResponseJSON:  string(responseJSON),
		Error:         errText,
		DurationMs:    duration.Milliseconds(),
		DryRun:        auditDryRun(ctx),
	}
	if err := a.storage.LogAPICall(call); err != nil && a.logger != nil {
		a.logger.Warn("Failed to log Monarch API call",
			"order_id", call.OrderID,
			"transaction_id", transactionID,
			"method", method,
			"error", err)
	}
}

type auditContextKey string

const (
	auditOrderIDKey auditContextKey = "order_id"
	auditDryRunKey  auditContextKey = "dry_run"
)

func withAuditContext(ctx context.Context, orderID string, dryRun bool) context.Context {
	ctx = context.WithValue(ctx, auditOrderIDKey, orderID)
	return context.WithValue(ctx, auditDryRunKey, dryRun)
}

func auditOrderID(ctx context.Context) string {
	if value, ok := ctx.Value(auditOrderIDKey).(string); ok {
		return value
	}
	return ""
}

func auditDryRun(ctx context.Context) bool {
	if value, ok := ctx.Value(auditDryRunKey).(bool); ok {
		return value
	}
	return false
}

// ledgerStorageAdapter wraps storage.Repository to implement handlers.LedgerStorage
type ledgerStorageAdapter struct {
	repo storage.Repository
}

func (a *ledgerStorageAdapter) SaveLedger(ledger *handlers.LedgerData, syncRunID int64) error {
	if a.repo == nil {
		return nil
	}

	// Convert handlers.LedgerData to storage.OrderLedger
	orderLedger := &storage.OrderLedger{
		OrderID:            ledger.OrderID,
		SyncRunID:          syncRunID,
		Provider:           ledger.Provider,
		LedgerJSON:         ledger.RawJSON,
		TotalCharged:       ledger.TotalCharged,
		ChargeCount:        ledger.ChargeCount,
		PaymentMethodTypes: ledger.PaymentMethodTypes,
		HasRefunds:         ledger.HasRefunds,
		IsValid:            ledger.IsValid,
		ValidationNotes:    ledger.ValidationNotes,
	}

	// Determine ledger state
	if ledger.ChargeCount == 0 {
		orderLedger.LedgerState = storage.LedgerStatePending
	} else if ledger.HasRefunds {
		orderLedger.LedgerState = storage.LedgerStatePartialRefund
	} else {
		orderLedger.LedgerState = storage.LedgerStateCharged
	}

	// Convert payment methods to charges
	chargeSeq := 0
	for _, pm := range ledger.PaymentMethods {
		for i, charge := range pm.FinalCharges {
			chargeSeq++
			chargeType := "payment"
			if charge < 0 {
				chargeType = "refund"
			}
			ledgerCharge := storage.LedgerCharge{
				OrderID:        ledger.OrderID,
				SyncRunID:      syncRunID,
				ChargeSequence: chargeSeq,
				ChargeAmount:   charge,
				ChargeType:     chargeType,
				PaymentMethod:  pm.PaymentType,
				CardType:       pm.CardType,
				CardLastFour:   pm.CardLastFour,
			}
			// Add charged date if available (parallel array to FinalCharges)
			if i < len(pm.ChargedDates) {
				ledgerCharge.ChargedAt = pm.ChargedDates[i]
			}
			orderLedger.Charges = append(orderLedger.Charges, ledgerCharge)
		}
	}

	return a.repo.SaveLedger(orderLedger)
}
