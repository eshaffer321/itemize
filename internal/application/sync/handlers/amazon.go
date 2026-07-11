// Package handlers provides provider-specific order processing logic.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
	"github.com/eshaffer321/itemize/internal/domain/allocator"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/itemize/internal/domain/matcher"
	"github.com/eshaffer321/itemize/internal/domain/validator"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

// AmazonOrder extends providers.Order with Amazon-specific methods
type AmazonOrder interface {
	providers.Order
	GetFinalCharges() ([]float64, error)
	GetNonBankAmount() (float64, error)
	IsMultiDelivery() (bool, error)
	// GetItemsForCharge returns only the items that belong to the shipment
	// matching the given charge amount. Falls back to all items when shipment
	// data is unavailable or the order has a single shipment.
	GetItemsForCharge(chargeAmount float64) []providers.OrderItem
}

// TransactionConsolidator consolidates multiple transactions into one
type TransactionConsolidator interface {
	ConsolidateTransactions(ctx context.Context, transactions []*monarch.Transaction, order providers.Order, dryRun bool) (*ConsolidationResult, error)
}

// ConsolidationResult holds the result of consolidating transactions
type ConsolidationResult struct {
	ConsolidatedTransaction *monarch.Transaction
	FailedDeletions         []string
}

// CategorySplitter creates transaction splits from categorized items
type CategorySplitter interface {
	CreateSplits(ctx context.Context, order providers.Order, transaction *monarch.Transaction, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory) ([]*monarch.TransactionSplit, error)
	GetSingleCategoryInfo(ctx context.Context, order providers.Order, categories []categorizer.Category) (string, string, error)
}

// MonarchClient provides access to Monarch API
type MonarchClient interface {
	UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error
	UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error
}

// LedgerData represents ledger data that can be saved
type LedgerData struct {
	OrderID            string
	Provider           string
	RawJSON            string
	PaymentMethods     []PaymentMethodData
	TotalCharged       float64
	ChargeCount        int
	PaymentMethodTypes string
	HasRefunds         bool
	IsValid            bool
	ValidationNotes    string
}

// PaymentMethodData represents a single payment method's charges
type PaymentMethodData struct {
	PaymentType  string
	CardType     string
	CardLastFour string
	FinalCharges []float64
	ChargedDates []time.Time
	TotalCharged float64
}

// LedgerStorage provides access to ledger persistence
type LedgerStorage interface {
	SaveLedger(ledger *LedgerData, syncRunID int64) error
}

// ProcessResult holds the result of processing an order
type ProcessResult struct {
	Processed   bool
	Skipped     bool
	SkipReason  string
	Allocations *allocator.Result
	Splits      []*monarch.TransactionSplit

	// Transaction tracking for audit trail - the matched/consolidated transaction
	Transaction *monarch.Transaction

	// Audit trail fields for single-category orders
	CategoryID   string // Category ID assigned (for single-category updates)
	CategoryName string // Human-readable category name
	MonarchNotes string // Notes sent to Monarch

	// Debug/reconciliation audit fields
	MatchDiagnosticsJSON   string
	ReconciledTransactions []*monarch.Transaction
}

// AmazonHandler processes Amazon orders with pro-rata allocation
type AmazonHandler struct {
	matcher      *matcher.Matcher
	consolidator TransactionConsolidator
	splitter     CategorySplitter
	monarch      MonarchClient
	logger       *slog.Logger
}

// NewAmazonHandler creates a new Amazon order handler
func NewAmazonHandler(
	matcher *matcher.Matcher,
	consolidator TransactionConsolidator,
	splitter CategorySplitter,
	monarch MonarchClient,
	logger *slog.Logger,
) *AmazonHandler {
	return &AmazonHandler{
		matcher:      matcher,
		consolidator: consolidator,
		splitter:     splitter,
		monarch:      monarch,
		logger:       logger,
	}
}

// ProcessOrder processes an Amazon order with pro-rata allocation
func (h *AmazonHandler) ProcessOrder(
	ctx context.Context,
	order AmazonOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	dryRun bool,
) (*ProcessResult, error) {
	result := &ProcessResult{}

	// Step 1: Get bank charges
	bankCharges, err := order.GetFinalCharges()
	if err != nil {
		switch {
		case errors.Is(err, amazonprovider.ErrPaymentPending):
			h.logInfo("Order payment pending (not yet shipped)",
				"order_id", order.GetID(),
				"order_total", order.GetTotal())
			result.Skipped = true
			result.SkipReason = "payment pending"
			return result, nil
		case errors.Is(err, amazonprovider.ErrGiftCardOrder):
			h.logInfo("Order paid entirely with gift cards/points — no bank transaction to match",
				"order_id", order.GetID(),
				"order_total", order.GetTotal())
			result.Skipped = true
			result.SkipReason = "paid entirely with gift cards/points"
			return result, nil
		default:
			return nil, fmt.Errorf("failed to get bank charges: %w", err)
		}
	}

	h.logDebug("Got bank charges",
		"order_id", order.GetID(),
		"charges", bankCharges,
		"charge_count", len(bankCharges))

	// Step 2: Get non-bank amount (points, gift cards, etc.)
	nonBankAmount, err := order.GetNonBankAmount()
	if err != nil {
		return nil, fmt.Errorf("failed to get non-bank amount: %w", err)
	}

	h.logDebug("Got non-bank amount",
		"order_id", order.GetID(),
		"non_bank_amount", nonBankAmount)

	// Step 3: Validate charges
	validation := validator.ValidateCharges(bankCharges, order.GetTotal(), nonBankAmount)

	// Step 4: Match to Monarch transactions
	var matchedTxns []*monarch.Transaction
	var consolidatedTxn *monarch.Transaction
	monarchDiscovered := false // true when we matched via subset search rather than provider charges

	if !validation.Valid {
		// Provider charges are incomplete (common for multi-shipment orders where later
		// charges post after the provider visited the order details page). Try to find the
		// matching Monarch transactions by searching for a subset that sums to the order total.
		h.logDebug("Provider charges incomplete, attempting Monarch-side discovery",
			"order_id", order.GetID(),
			"provider_charge_sum", validation.BankChargesSum,
			"expected", validation.ExpectedSum)

		discovered, discoverErr := h.matcher.FindSubsetByTotal(order, monarchTxns, usedTxnIDs)
		if discoverErr != nil {
			h.logWarn("Charge validation failed and Monarch discovery found no match",
				"order_id", order.GetID(),
				"reason", validation.Reason,
				"bank_sum", validation.BankChargesSum,
				"expected", validation.ExpectedSum,
				"difference", validation.Difference)
			result.Skipped = true
			result.SkipReason = validation.Reason
			return result, nil
		}

		matchedTxns = discovered
		monarchDiscovered = true
		for _, t := range matchedTxns {
			usedTxnIDs[t.ID] = true
		}
		h.logInfo("Monarch-side discovery found matching transactions",
			"order_id", order.GetID(),
			"count", len(matchedTxns))
	} else {
		h.logDebug("Charge validation passed",
			"order_id", order.GetID(),
			"bank_sum", validation.BankChargesSum,
			"expected", validation.ExpectedSum)

		if len(bankCharges) > 1 {
			// Multi-delivery order - find multiple matches
			multiResult, err := h.matcher.FindMultipleMatches(order, monarchTxns, usedTxnIDs, bankCharges)
			if err != nil {
				return nil, fmt.Errorf("multi-match error: %w", err)
			}

			if !multiResult.AllFound {
				result.Skipped = true
				result.SkipReason = fmt.Sprintf("could not find all transactions: expected %d, found %d",
					len(bankCharges), len(multiResult.Matches))
				h.logWarn("Not all transactions found",
					"order_id", order.GetID(),
					"expected", len(bankCharges),
					"found", len(multiResult.Matches))
				return result, nil
			}

			for _, match := range multiResult.Matches {
				matchedTxns = append(matchedTxns, match.Transaction)
				usedTxnIDs[match.Transaction.ID] = true
			}
			h.logInfo("Matched all transactions for multi-delivery order",
				"order_id", order.GetID(),
				"transaction_count", len(matchedTxns))
		} else {
			// Single charge - find one match
			// Use a wrapper order that returns the bank charge amount for matching
			// This handles gift card orders where order total differs from bank charge
			matchOrder := &bankChargeOrder{
				Order:      order,
				bankCharge: bankCharges[0],
			}

			matchResult, err := h.matcher.FindMatch(matchOrder, monarchTxns, usedTxnIDs)
			if err != nil {
				return nil, fmt.Errorf("match error: %w", err)
			}

			if matchResult == nil {
				result.Skipped = true
				result.SkipReason = "no matching transaction found"
				h.logWarn("No matching transaction found",
					"order_id", order.GetID(),
					"expected_amount", bankCharges[0])
				return result, nil
			}

			consolidatedTxn = matchResult.Transaction
			usedTxnIDs[consolidatedTxn.ID] = true

			h.logDebug("Matched single transaction",
				"order_id", order.GetID(),
				"transaction_id", consolidatedTxn.ID,
				"amount", math.Abs(consolidatedTxn.Amount))
		}
	}

	// Step 5: Consolidate multi-transaction matches
	if consolidatedTxn == nil {
		if len(matchedTxns) > 1 {
			consolidationResult, err := h.consolidator.ConsolidateTransactions(ctx, matchedTxns, order, dryRun)
			if err != nil {
				return nil, fmt.Errorf("consolidation error: %w", err)
			}
			consolidatedTxn = consolidationResult.ConsolidatedTransaction
			h.logInfo("Consolidated transactions",
				"order_id", order.GetID(),
				"consolidated_id", consolidatedTxn.ID,
				"original_count", len(matchedTxns))
		} else if len(matchedTxns) == 1 {
			consolidatedTxn = matchedTxns[0]
		}
	}

	// Step 6: Pro-rata allocation
	// For Monarch-discovered charges, use the order total and all items since we
	// don't have per-shipment mapping. For provider-validated charges, use the
	// per-shipment breakdown when available.
	var chargeAmount float64
	var allocationTotal float64
	if monarchDiscovered {
		chargeAmount = order.GetTotal()
		for _, t := range matchedTxns {
			allocationTotal += math.Abs(t.Amount)
		}
	} else {
		chargeAmount = validation.BankChargesSum
		if len(bankCharges) == 1 {
			chargeAmount = bankCharges[0]
		}
		allocationTotal = validation.BankChargesSum
	}
	orderItems := order.GetItemsForCharge(chargeAmount)
	items := make([]allocator.Item, len(orderItems))
	for i, item := range orderItems {
		items[i] = allocator.Item{
			Name:      item.GetName(),
			ListPrice: item.GetPrice(),
		}
	}

	allocResult, err := allocator.Allocate(items, allocationTotal)
	if err != nil {
		return nil, fmt.Errorf("allocation error: %w", err)
	}

	result.Allocations = allocResult

	h.logDebug("Allocated costs",
		"order_id", order.GetID(),
		"multiplier", allocResult.Multiplier,
		"total_allocated", allocResult.TotalAllocated,
		"monarch_discovered", monarchDiscovered)

	// Step 7: Create an allocated order for the splitter using the per-shipment items
	allocatedOrder := &allocatedAmazonOrder{
		Order:       order,
		allocations: allocResult.Allocations,
		baseItems:   orderItems,
	}

	// Step 8: Categorize and create splits
	splits, err := h.splitter.CreateSplits(ctx, allocatedOrder, consolidatedTxn, catCategories, monarchCategories)
	if err != nil {
		return nil, fmt.Errorf("split creation error: %w", err)
	}

	result.Splits = splits

	// Step 9: Apply to Monarch
	if splits == nil {
		// Single category - update transaction category
		categoryID, notes, err := h.splitter.GetSingleCategoryInfo(ctx, allocatedOrder, catCategories)
		if err != nil {
			return nil, fmt.Errorf("get category info error: %w", err)
		}

		// Populate audit trail fields
		result.CategoryID = categoryID
		result.MonarchNotes = notes
		// Extract category name from notes (format: "CategoryName:\n...")
		if colonIdx := len(notes); colonIdx > 0 {
			if idx := strings.Index(notes, ":"); idx > 0 {
				result.CategoryName = notes[:idx]
			}
		}

		if !dryRun {
			reviewed := false
			params := &monarch.UpdateTransactionParams{
				Notes:       &notes,
				NeedsReview: &reviewed,
			}
			// Only set category if the LLM returned a valid Monarch category ID.
			// An empty ID means the categorizer couldn't map to a known category —
			// we still write notes so the order is recorded, but skip the category
			// update to avoid a Monarch API error.
			if categoryID != "" {
				params.CategoryID = &categoryID
			} else {
				h.logWarn("Skipping category update — no valid Monarch category ID returned by LLM",
					"order_id", order.GetID(),
					"transaction_id", consolidatedTxn.ID,
					"category_name", result.CategoryName)
			}
			if err := h.monarch.UpdateTransaction(ctx, consolidatedTxn.ID, params); err != nil {
				return nil, fmt.Errorf("update transaction error: %w", err)
			}
			h.logDebug("Updated transaction notes",
				"order_id", order.GetID(),
				"transaction_id", consolidatedTxn.ID,
				"category_id", categoryID)
		} else {
			h.logDebug("[DRY RUN] Would update transaction",
				"order_id", order.GetID(),
				"category_id", categoryID)
		}
	} else {
		// Multiple categories - apply splits
		if !dryRun {
			if err := h.monarch.UpdateSplits(ctx, consolidatedTxn.ID, splits); err != nil {
				return nil, fmt.Errorf("update splits error: %w", err)
			}
			// Mark the parent transaction as reviewed so Monarch's rule engine
			// doesn't re-categorize it after the split is applied.
			reviewed := false
			if err := h.monarch.UpdateTransaction(ctx, consolidatedTxn.ID, &monarch.UpdateTransactionParams{
				NeedsReview: &reviewed,
			}); err != nil {
				h.logWarn("Failed to mark split transaction as reviewed",
					"order_id", order.GetID(),
					"transaction_id", consolidatedTxn.ID,
					"error", err)
			}
			h.logDebug("Applied splits",
				"order_id", order.GetID(),
				"transaction_id", consolidatedTxn.ID,
				"split_count", len(splits))
		} else {
			h.logDebug("[DRY RUN] Would apply splits",
				"order_id", order.GetID(),
				"split_count", len(splits))
		}
	}

	result.Transaction = consolidatedTxn
	result.Processed = true
	return result, nil
}

// bankChargeOrder wraps an order to return the bank charge amount for matching
// This handles gift card orders where the order total differs from the bank charge
type bankChargeOrder struct {
	providers.Order
	bankCharge float64
}

// GetTotal returns the bank charge amount instead of order total
func (b *bankChargeOrder) GetTotal() float64 {
	return b.bankCharge
}

// allocatedAmazonOrder wraps an Amazon order with allocated item prices
type allocatedAmazonOrder struct {
	providers.Order
	allocations []allocator.Allocation
	baseItems   []providers.OrderItem // the per-shipment items used for allocation
}

// GetItems returns items with allocated prices
func (a *allocatedAmazonOrder) GetItems() []providers.OrderItem {
	items := make([]providers.OrderItem, len(a.allocations))
	for i, alloc := range a.allocations {
		items[i] = &allocatedItem{
			name:  alloc.Name,
			price: alloc.AllocatedCost,
		}
	}
	return items
}

// allocatedItem represents an item with its allocated cost
type allocatedItem struct {
	name  string
	price float64
}

func (i *allocatedItem) GetName() string        { return i.name }
func (i *allocatedItem) GetPrice() float64      { return i.price }
func (i *allocatedItem) GetQuantity() float64   { return 1 }
func (i *allocatedItem) GetUnitPrice() float64  { return i.price }
func (i *allocatedItem) GetDescription() string { return "" }
func (i *allocatedItem) GetSKU() string         { return "" }
func (i *allocatedItem) GetCategory() string    { return "" }

// IsAmazonOrder checks if an order is an Amazon order
func IsAmazonOrder(order providers.Order) bool {
	_, ok := order.(*amazonprovider.Order)
	return ok
}

// AsAmazonOrder converts a providers.Order to AmazonOrder if possible
func AsAmazonOrder(order providers.Order) (AmazonOrder, bool) {
	amazonOrder, ok := order.(*amazonprovider.Order)
	if !ok {
		return nil, false
	}
	return amazonOrder, true
}

// Nil-safe logging helpers
func (h *AmazonHandler) logDebug(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Debug(msg, args...)
	}
}

func (h *AmazonHandler) logInfo(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Info(msg, args...)
	}
}

func (h *AmazonHandler) logWarn(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Warn(msg, args...)
	}
}
