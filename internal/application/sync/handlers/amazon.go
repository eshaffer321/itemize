// Package handlers provides provider-specific order processing logic.
package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	amazonprovider "github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers/amazon"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/allocator"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/matcher"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/validator"
)

// AmazonOrder extends providers.Order with Amazon-specific methods
type AmazonOrder interface {
	providers.Order
	GetFinalCharges() ([]float64, error)
	GetNonBankAmount() (float64, error)
	IsMultiDelivery() (bool, error)
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

// MonarchClient provides access to Monarch Money API
type MonarchClient interface {
	UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error
	UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error
}

// ProcessResult holds the result of processing an order
type ProcessResult struct {
	Processed   bool
	Skipped     bool
	SkipReason  string
	Allocations *allocator.Result
	Splits      []*monarch.TransactionSplit
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
		return nil, fmt.Errorf("failed to get bank charges: %w", err)
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
	if !validation.Valid {
		h.logWarn("Charge validation failed",
			"order_id", order.GetID(),
			"reason", validation.Reason,
			"bank_sum", validation.BankChargesSum,
			"expected", validation.ExpectedSum,
			"difference", validation.Difference)
		result.Skipped = true
		result.SkipReason = validation.Reason
		return result, nil
	}

	h.logDebug("Charge validation passed",
		"order_id", order.GetID(),
		"bank_sum", validation.BankChargesSum,
		"expected", validation.ExpectedSum)

	// Step 4: Match to Monarch transactions
	var matchedTxns []*monarch.Transaction
	var consolidatedTxn *monarch.Transaction

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

		// Extract matched transactions
		for _, match := range multiResult.Matches {
			matchedTxns = append(matchedTxns, match.Transaction)
			usedTxnIDs[match.Transaction.ID] = true
		}

		h.logInfo("Matched all transactions for multi-delivery order",
			"order_id", order.GetID(),
			"transaction_count", len(matchedTxns))

		// Step 5: Consolidate transactions
		consolidationResult, err := h.consolidator.ConsolidateTransactions(ctx, matchedTxns, order, dryRun)
		if err != nil {
			return nil, fmt.Errorf("consolidation error: %w", err)
		}
		consolidatedTxn = consolidationResult.ConsolidatedTransaction

		h.logInfo("Consolidated transactions",
			"order_id", order.GetID(),
			"consolidated_id", consolidatedTxn.ID,
			"original_count", len(matchedTxns))
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

	// Step 6: Pro-rata allocation
	items := make([]allocator.Item, len(order.GetItems()))
	for i, item := range order.GetItems() {
		items[i] = allocator.Item{
			Name:      item.GetName(),
			ListPrice: item.GetPrice(),
		}
	}

	// Use the sum of bank charges as the order total for allocation
	// This is the actual amount charged to the bank
	allocationTotal := validation.BankChargesSum

	allocResult, err := allocator.Allocate(items, allocationTotal)
	if err != nil {
		return nil, fmt.Errorf("allocation error: %w", err)
	}

	result.Allocations = allocResult

	h.logDebug("Allocated costs",
		"order_id", order.GetID(),
		"multiplier", allocResult.Multiplier,
		"total_allocated", allocResult.TotalAllocated)

	// Step 7: Create an allocated order for the splitter
	allocatedOrder := &allocatedAmazonOrder{
		Order:       order,
		allocations: allocResult.Allocations,
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

		if !dryRun {
			params := &monarch.UpdateTransactionParams{
				CategoryID: &categoryID,
				Notes:      &notes,
			}
			if err := h.monarch.UpdateTransaction(ctx, consolidatedTxn.ID, params); err != nil {
				return nil, fmt.Errorf("update transaction error: %w", err)
			}
			h.logDebug("Updated transaction category",
				"order_id", order.GetID(),
				"transaction_id", consolidatedTxn.ID,
				"category_id", categoryID)
		} else {
			h.logDebug("[DRY RUN] Would update transaction category",
				"order_id", order.GetID(),
				"category_id", categoryID)
		}
	} else {
		// Multiple categories - apply splits
		if !dryRun {
			if err := h.monarch.UpdateSplits(ctx, consolidatedTxn.ID, splits); err != nil {
				return nil, fmt.Errorf("update splits error: %w", err)
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
