// Package handlers provides provider-specific order processing logic.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	walmartprovider "github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers/walmart"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/matcher"
)

// WalmartOrder extends providers.Order with Walmart-specific methods
type WalmartOrder interface {
	providers.Order
	GetFinalCharges() ([]float64, error)
	IsMultiDelivery() (bool, error)
}

// WalmartOrderWithLedger extends WalmartOrder with ledger access for persistence
type WalmartOrderWithLedger interface {
	WalmartOrder
	GetRawLedger() interface{} // Returns *walmartclient.OrderLedger but using interface{} to avoid import
}

// WalmartHandler processes Walmart orders with multi-delivery and gift card support
type WalmartHandler struct {
	matcher       *matcher.Matcher
	consolidator  TransactionConsolidator
	splitter      CategorySplitter
	monarch       MonarchClient
	ledgerStorage LedgerStorage
	syncRunID     int64
	logger        *slog.Logger
}

// NewWalmartHandler creates a new Walmart order handler
func NewWalmartHandler(
	matcher *matcher.Matcher,
	consolidator TransactionConsolidator,
	splitter CategorySplitter,
	monarch MonarchClient,
	logger *slog.Logger,
) *WalmartHandler {
	return &WalmartHandler{
		matcher:      matcher,
		consolidator: consolidator,
		splitter:     splitter,
		monarch:      monarch,
		logger:       logger,
	}
}

// SetLedgerStorage sets the ledger storage for persisting ledger data
func (h *WalmartHandler) SetLedgerStorage(storage LedgerStorage, syncRunID int64) {
	h.ledgerStorage = storage
	h.syncRunID = syncRunID
}

// ProcessOrder processes a Walmart order
func (h *WalmartHandler) ProcessOrder(
	ctx context.Context,
	order WalmartOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	dryRun bool,
) (*ProcessResult, error) {
	result := &ProcessResult{}

	// Step 1: Get bank charges from ledger
	bankCharges, err := order.GetFinalCharges()
	if err != nil {
		// Check if this is a pending order (not yet charged)
		if strings.Contains(err.Error(), "payment pending") {
			h.logInfo("Skipping order - not yet charged", "order_id", order.GetID())
			result.Skipped = true
			result.SkipReason = "payment pending"
			return result, nil
		}
		// For other ledger errors, fall through to regular matching using order total
		h.logWarn("Failed to get ledger charges, falling back to order total",
			"order_id", order.GetID(),
			"error", err)
		return h.processWithOrderTotal(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, dryRun)
	}

	h.logDebug("Got bank charges",
		"order_id", order.GetID(),
		"charges", bankCharges,
		"charge_count", len(bankCharges))

	// Save ledger data if storage is configured
	h.saveLedgerIfAvailable(order)

	// Step 2: Handle based on number of charges
	if len(bankCharges) > 1 {
		// Multi-delivery order
		return h.processMultiDeliveryOrder(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, bankCharges, dryRun)
	}

	// Single charge - check if it differs from order total (gift card scenario)
	bankCharge := bankCharges[0]
	orderTotal := order.GetTotal()

	const epsilon = 0.01 // Allow 1 cent difference for floating point
	if math.Abs(bankCharge-orderTotal) > epsilon {
		h.logInfo("Using ledger amount for matching (differs from order total)",
			"order_id", order.GetID(),
			"order_total", orderTotal,
			"ledger_charge", bankCharge,
			"difference", math.Abs(bankCharge-orderTotal))

		return h.processWithLedgerAmount(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, bankCharge, dryRun)
	}

	// Ledger amount equals order total - use regular matching
	return h.processWithOrderTotal(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, dryRun)
}

// processWithOrderTotal handles standard orders using GetTotal() for matching
func (h *WalmartHandler) processWithOrderTotal(
	ctx context.Context,
	order WalmartOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	dryRun bool,
) (*ProcessResult, error) {
	result := &ProcessResult{}

	matchResult, err := h.matcher.FindMatch(order, monarchTxns, usedTxnIDs)
	if err != nil {
		return nil, fmt.Errorf("match error: %w", err)
	}

	if matchResult == nil {
		result.Skipped = true
		result.SkipReason = "no matching transaction found"
		h.logWarn("No matching transaction found",
			"order_id", order.GetID(),
			"expected_amount", order.GetTotal())
		return result, nil
	}

	usedTxnIDs[matchResult.Transaction.ID] = true

	h.logDebug("Matched transaction",
		"order_id", order.GetID(),
		"transaction_id", matchResult.Transaction.ID,
		"amount", math.Abs(matchResult.Transaction.Amount),
		"date_diff_days", matchResult.DateDiff)

	return h.categorizeAndApplySplits(ctx, order, matchResult.Transaction, catCategories, monarchCategories, dryRun)
}

// processWithLedgerAmount handles orders where bank charge differs from order total (gift cards)
func (h *WalmartHandler) processWithLedgerAmount(
	ctx context.Context,
	order WalmartOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	ledgerAmount float64,
	dryRun bool,
) (*ProcessResult, error) {
	result := &ProcessResult{}

	// Create wrapper order that returns ledger amount for matching
	matchOrder := &ledgerAmountOrder{
		Order:        order,
		ledgerAmount: ledgerAmount,
	}

	matchResult, err := h.matcher.FindMatch(matchOrder, monarchTxns, usedTxnIDs)
	if err != nil {
		return nil, fmt.Errorf("match error: %w", err)
	}

	if matchResult == nil {
		result.Skipped = true
		result.SkipReason = "no matching transaction found for ledger amount"
		h.logWarn("No matching transaction found for ledger amount",
			"order_id", order.GetID(),
			"ledger_amount", ledgerAmount)
		return result, nil
	}

	usedTxnIDs[matchResult.Transaction.ID] = true

	h.logDebug("Matched transaction using ledger amount",
		"order_id", order.GetID(),
		"transaction_id", matchResult.Transaction.ID,
		"ledger_amount", ledgerAmount,
		"transaction_amount", math.Abs(matchResult.Transaction.Amount),
		"date_diff_days", matchResult.DateDiff)

	// Use original order (not wrapper) for categorization
	return h.categorizeAndApplySplits(ctx, order, matchResult.Transaction, catCategories, monarchCategories, dryRun)
}

// processMultiDeliveryOrder handles orders split into multiple bank charges
func (h *WalmartHandler) processMultiDeliveryOrder(
	ctx context.Context,
	order WalmartOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	charges []float64,
	dryRun bool,
) (*ProcessResult, error) {
	result := &ProcessResult{}

	h.logInfo("Processing multi-delivery order",
		"order_id", order.GetID(),
		"charge_count", len(charges),
		"charges", charges)

	// Find matching transactions for each charge
	multiResult, err := h.matcher.FindMultipleMatches(order, monarchTxns, usedTxnIDs, charges)
	if err != nil {
		return nil, fmt.Errorf("multi-match error: %w", err)
	}

	if !multiResult.AllFound {
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("could not find all transactions: expected %d, found %d",
			len(charges), len(multiResult.Matches))
		h.logWarn("Not all transactions found",
			"order_id", order.GetID(),
			"expected", len(charges),
			"found", len(multiResult.Matches))
		return result, nil
	}

	// Extract matched transactions
	var matchedTxns []*monarch.Transaction
	for _, match := range multiResult.Matches {
		matchedTxns = append(matchedTxns, match.Transaction)
		usedTxnIDs[match.Transaction.ID] = true
	}

	h.logInfo("Matched all transactions for multi-delivery order",
		"order_id", order.GetID(),
		"transaction_count", len(matchedTxns))

	// Consolidate transactions into one
	consolidationResult, err := h.consolidator.ConsolidateTransactions(ctx, matchedTxns, order, dryRun)
	if err != nil {
		return nil, fmt.Errorf("consolidation error: %w", err)
	}

	consolidatedTxn := consolidationResult.ConsolidatedTransaction

	h.logInfo("Consolidated transactions",
		"order_id", order.GetID(),
		"consolidated_id", consolidatedTxn.ID,
		"original_count", len(matchedTxns))

	// Categorize and apply splits to consolidated transaction
	return h.categorizeAndApplySplits(ctx, order, consolidatedTxn, catCategories, monarchCategories, dryRun)
}

// categorizeAndApplySplits applies categorization and splits to a transaction
func (h *WalmartHandler) categorizeAndApplySplits(
	ctx context.Context,
	order WalmartOrder,
	transaction *monarch.Transaction,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	dryRun bool,
) (*ProcessResult, error) {
	result := &ProcessResult{}

	// Create splits using the splitter
	splits, err := h.splitter.CreateSplits(ctx, order, transaction, catCategories, monarchCategories)
	if err != nil {
		return nil, fmt.Errorf("split creation error: %w", err)
	}

	result.Splits = splits

	// Apply to Monarch
	if splits == nil {
		// Single category - update transaction category
		categoryID, notes, err := h.splitter.GetSingleCategoryInfo(ctx, order, catCategories)
		if err != nil {
			return nil, fmt.Errorf("get category info error: %w", err)
		}

		if !dryRun {
			params := &monarch.UpdateTransactionParams{
				CategoryID: &categoryID,
				Notes:      &notes,
			}
			if err := h.monarch.UpdateTransaction(ctx, transaction.ID, params); err != nil {
				return nil, fmt.Errorf("update transaction error: %w", err)
			}
			h.logDebug("Updated transaction category",
				"order_id", order.GetID(),
				"transaction_id", transaction.ID,
				"category_id", categoryID)
		} else {
			h.logDebug("[DRY RUN] Would update transaction category",
				"order_id", order.GetID(),
				"category_id", categoryID)
		}
	} else {
		// Multiple categories - apply splits
		if !dryRun {
			if err := h.monarch.UpdateSplits(ctx, transaction.ID, splits); err != nil {
				return nil, fmt.Errorf("update splits error: %w", err)
			}
			h.logDebug("Applied splits",
				"order_id", order.GetID(),
				"transaction_id", transaction.ID,
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

// ledgerAmountOrder wraps an order to return a ledger amount for matching
type ledgerAmountOrder struct {
	providers.Order
	ledgerAmount float64
}

// GetTotal returns the ledger amount instead of order total
func (l *ledgerAmountOrder) GetTotal() float64 {
	return l.ledgerAmount
}

// IsWalmartOrder checks if an order is a Walmart order
func IsWalmartOrder(order providers.Order) bool {
	_, ok := order.(*walmartprovider.Order)
	return ok
}

// AsWalmartOrder converts a providers.Order to WalmartOrder if possible
func AsWalmartOrder(order providers.Order) (WalmartOrder, bool) {
	walmartOrder, ok := order.(*walmartprovider.Order)
	if !ok {
		return nil, false
	}
	return walmartOrder, true
}

// Nil-safe logging helpers
func (h *WalmartHandler) logDebug(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Debug(msg, args...)
	}
}

func (h *WalmartHandler) logInfo(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Info(msg, args...)
	}
}

func (h *WalmartHandler) logWarn(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Warn(msg, args...)
	}
}

// saveLedgerIfAvailable extracts and saves ledger data if storage is configured
func (h *WalmartHandler) saveLedgerIfAvailable(order WalmartOrder) {
	// Skip if no storage configured
	if h.ledgerStorage == nil {
		return
	}

	// Try to get the raw ledger from the concrete type
	walmartOrder, ok := order.(*walmartprovider.Order)
	if !ok {
		h.logDebug("Cannot save ledger - order is not a Walmart provider order")
		return
	}

	rawLedger := walmartOrder.GetRawLedger()
	if rawLedger == nil {
		h.logDebug("Cannot save ledger - no ledger data available")
		return
	}

	// Convert the raw ledger to LedgerData
	ledgerData := h.convertToLedgerData(order.GetID(), rawLedger)

	// Save it
	if err := h.ledgerStorage.SaveLedger(ledgerData, h.syncRunID); err != nil {
		h.logWarn("Failed to save ledger data",
			"order_id", order.GetID(),
			"error", err)
	} else {
		h.logDebug("Saved ledger data",
			"order_id", order.GetID(),
			"charge_count", ledgerData.ChargeCount)
	}
}

// convertToLedgerData converts raw Walmart ledger to the handler's LedgerData format
func (h *WalmartHandler) convertToLedgerData(orderID string, rawLedger interface{}) *LedgerData {
	ledgerData := &LedgerData{
		OrderID:  orderID,
		Provider: "walmart",
		IsValid:  true,
	}

	// Use reflection-free approach with type assertion
	// The rawLedger is *walmartclient.OrderLedger
	type walmartLedger struct {
		OrderID        string
		PaymentMethods []struct {
			PaymentType  string
			CardType     string
			LastFour     string
			FinalCharges []float64
			TotalCharged float64
		}
	}

	// Marshal to JSON and back to extract the data
	// This avoids importing the walmart client in handlers
	import_json, _ := json.Marshal(rawLedger)
	ledgerData.RawJSON = string(import_json)

	// Parse for payment method extraction
	var parsed walmartLedger
	if err := json.Unmarshal(import_json, &parsed); err != nil {
		h.logWarn("Failed to parse ledger JSON", "error", err)
		return ledgerData
	}

	// Collect payment method types and charges
	var paymentTypes []string
	totalCharged := 0.0
	chargeCount := 0
	hasRefunds := false

	for _, pm := range parsed.PaymentMethods {
		paymentTypes = append(paymentTypes, pm.PaymentType)
		totalCharged += pm.TotalCharged

		pmData := PaymentMethodData{
			PaymentType:  pm.PaymentType,
			CardType:     pm.CardType,
			CardLastFour: pm.LastFour,
			FinalCharges: pm.FinalCharges,
			TotalCharged: pm.TotalCharged,
		}
		ledgerData.PaymentMethods = append(ledgerData.PaymentMethods, pmData)

		for _, charge := range pm.FinalCharges {
			if charge > 0 {
				chargeCount++
			} else if charge < 0 {
				hasRefunds = true
			}
		}
	}

	ledgerData.TotalCharged = totalCharged
	ledgerData.ChargeCount = chargeCount
	ledgerData.PaymentMethodTypes = strings.Join(paymentTypes, ",")
	ledgerData.HasRefunds = hasRefunds

	return ledgerData
}
