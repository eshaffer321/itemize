// Package handlers provides provider-specific order processing logic.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	walmartprovider "github.com/eshaffer321/itemize/internal/adapters/providers/walmart"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/itemize/internal/domain/matcher"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

// WalmartOrder extends providers.Order with Walmart-specific methods
type WalmartOrder interface {
	providers.Order
	GetFinalCharges() ([]float64, error)
	IsMultiDelivery() (bool, error)
}

// WalmartOrderWithRefunds extends WalmartOrder with refund ledger access.
type WalmartOrderWithRefunds interface {
	WalmartOrder
	GetRefundCharges() ([]float64, error)
}

type WalmartOrderWithRefundItems interface {
	WalmartOrder
	GetRefundItems() ([]providers.OrderItem, error)
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
	// Step 1: Get bank charges from ledger
	bankCharges, err := order.GetFinalCharges()
	refundCharges := h.getRefundCharges(order)
	refundItems := h.getRefundItems(order, len(refundCharges))

	h.saveLedgerIfAvailable(order)

	if err != nil {
		// Check if this is a pending order (not yet charged)
		if strings.Contains(err.Error(), "payment pending") && len(refundCharges) == 0 {
			h.logInfo("Skipping order - not yet charged", "order_id", order.GetID())
			result := &ProcessResult{}
			result.Skipped = true
			result.SkipReason = "payment pending"
			return result, nil
		}
		if strings.Contains(err.Error(), "no positive charges found") && len(refundCharges) > 0 {
			h.logInfo("Processing refund-only order",
				"order_id", order.GetID(),
				"refund_count", len(refundCharges),
				"refunds", refundCharges)
			return h.processRefundOnlyOrder(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, refundCharges, refundItems, dryRun)
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

	var result *ProcessResult
	if len(bankCharges) > 1 {
		result, err = h.processMultiDeliveryOrder(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, bankCharges, dryRun)
	} else {
		result, err = h.processSingleChargeOrder(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, bankCharges[0], dryRun)
	}
	if err != nil || len(refundCharges) == 0 || len(refundItems) == 0 {
		if err == nil && len(refundCharges) > 0 && len(refundItems) == 0 {
			h.logInfo("Skipping refund without item-level Walmart detail", "order_id", order.GetID())
		}
		return result, err
	}

	refundResults, refundErr := h.processRefundCharges(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, refundCharges, refundItems, dryRun)
	if refundErr != nil {
		h.logWarn("Failed to process refund charges",
			"order_id", order.GetID(),
			"error", refundErr)
		return result, nil
	}
	result.Refunds = refundResults
	if result.Skipped {
		result.Skipped = false
		result.SkipReason = ""
		result.Processed = true
		if len(refundResults) > 0 {
			result.Transaction = refundResults[0].Transaction
			result.Splits = refundResults[0].Splits
		}
	}
	return result, nil
}

func (h *WalmartHandler) getRefundItems(order WalmartOrder, refundCount int) []providers.OrderItem {
	if refundCount != 1 {
		return nil
	}
	refundOrder, ok := order.(WalmartOrderWithRefundItems)
	if !ok {
		return nil
	}
	items, err := refundOrder.GetRefundItems()
	if err != nil {
		h.logWarn("Failed to get refunded Walmart items", "order_id", order.GetID(), "error", err)
		return nil
	}
	return items
}

func (h *WalmartHandler) processSingleChargeOrder(
	ctx context.Context,
	order WalmartOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	bankCharge float64,
	dryRun bool,
) (*ProcessResult, error) {
	// Single charge - check if it differs from order total (gift card scenario)
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

func (h *WalmartHandler) getRefundCharges(order WalmartOrder) []float64 {
	refundOrder, ok := order.(WalmartOrderWithRefunds)
	if !ok {
		return nil
	}

	refunds, err := refundOrder.GetRefundCharges()
	if err != nil {
		if strings.Contains(err.Error(), "payment pending") {
			h.logDebug("No refund charges available for pending order",
				"order_id", order.GetID())
			return nil
		}
		h.logWarn("Failed to get refund charges",
			"order_id", order.GetID(),
			"error", err)
		return nil
	}

	if len(refunds) > 0 {
		h.logInfo("Got refund charges",
			"order_id", order.GetID(),
			"refund_count", len(refunds),
			"refunds", refunds)
	}

	return refunds
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
		aggregateResult, aggregateErr := h.processMultiDeliveryAggregateFallback(
			ctx,
			order,
			monarchTxns,
			usedTxnIDs,
			catCategories,
			monarchCategories,
			charges,
			multiResult.Matches,
			dryRun,
		)
		if aggregateErr != nil {
			return nil, aggregateErr
		}
		if aggregateResult != nil {
			return aggregateResult, nil
		}

		// Count actual non-nil matches (len(Matches) includes nil entries for index alignment)
		foundCount := countFoundMatches(multiResult.Matches)
		result.Skipped = true
		result.SkipReason = fmt.Sprintf("could not find all transactions: expected %d, found %d",
			len(charges), foundCount)
		h.logWarn("Not all transactions found",
			"order_id", order.GetID(),
			"expected", len(charges),
			"found", foundCount)
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

func (h *WalmartHandler) processMultiDeliveryAggregateFallback(
	ctx context.Context,
	order WalmartOrder,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
	catCategories []categorizer.Category,
	monarchCategories []*monarch.TransactionCategory,
	charges []float64,
	partialMatches []*matcher.MatchResult,
	dryRun bool,
) (*ProcessResult, error) {
	ledgerTotal := sumCharges(charges)
	if ledgerTotal <= 0 {
		return nil, nil
	}

	matchOrder := &ledgerAmountOrder{
		Order:        order,
		ledgerAmount: ledgerTotal,
	}

	matchedTxns, err := h.matcher.FindSubsetByTotal(matchOrder, monarchTxns, usedTxnIDs)
	if err != nil {
		h.logDebug("No aggregate transaction fallback match",
			"order_id", order.GetID(),
			"ledger_total", ledgerTotal,
			"error", err)
		return nil, nil
	}

	for _, txn := range matchedTxns {
		usedTxnIDs[txn.ID] = true
	}

	if len(matchedTxns) == 1 {
		recoveryTxns := interruptedConsolidationTransactions(matchedTxns[0], partialMatches, charges, ledgerTotal)
		if len(recoveryTxns) > 1 {
			if h.consolidator == nil {
				return nil, fmt.Errorf("interrupted consolidation found %d undeleted transactions but consolidator is not configured", len(recoveryTxns)-1)
			}
			for _, txn := range recoveryTxns[1:] {
				usedTxnIDs[txn.ID] = true
			}
			h.logInfo("Resuming interrupted multi-delivery consolidation",
				"order_id", order.GetID(),
				"consolidated_id", matchedTxns[0].ID,
				"undeleted_count", len(recoveryTxns)-1)
			consolidationResult, err := h.consolidator.ConsolidateTransactions(ctx, recoveryTxns, matchOrder, dryRun)
			if err != nil {
				return nil, fmt.Errorf("interrupted consolidation recovery error: %w", err)
			}
			return h.categorizeAndApplySplits(ctx, order, consolidationResult.ConsolidatedTransaction, catCategories, monarchCategories, dryRun)
		}

		h.logInfo("Matched multi-delivery order by aggregate ledger total",
			"order_id", order.GetID(),
			"ledger_charge_count", len(charges),
			"ledger_total", ledgerTotal,
			"transaction_id", matchedTxns[0].ID)
		return h.categorizeAndApplySplits(ctx, order, matchedTxns[0], catCategories, monarchCategories, dryRun)
	}

	if h.consolidator == nil {
		return nil, fmt.Errorf("aggregate match found %d transactions but consolidator is not configured", len(matchedTxns))
	}

	h.logInfo("Matched multi-delivery order by aggregate transaction subset",
		"order_id", order.GetID(),
		"ledger_charge_count", len(charges),
		"ledger_total", ledgerTotal,
		"transaction_count", len(matchedTxns))

	consolidationResult, err := h.consolidator.ConsolidateTransactions(ctx, matchedTxns, matchOrder, dryRun)
	if err != nil {
		return nil, fmt.Errorf("aggregate fallback consolidation error: %w", err)
	}

	return h.categorizeAndApplySplits(ctx, order, consolidationResult.ConsolidatedTransaction, catCategories, monarchCategories, dryRun)
}

func interruptedConsolidationTransactions(
	primary *monarch.Transaction,
	partialMatches []*matcher.MatchResult,
	charges []float64,
	ledgerTotal float64,
) []*monarch.Transaction {
	if primary == nil || math.Abs(math.Abs(primary.Amount)-math.Abs(ledgerTotal)) > 0.01 {
		return nil
	}
	if primary.Notes != multiDeliveryConsolidationNote(charges) {
		return nil
	}

	transactions := []*monarch.Transaction{primary}
	seen := map[string]bool{primary.ID: true}
	for _, match := range partialMatches {
		if match == nil || match.Transaction == nil || seen[match.Transaction.ID] {
			continue
		}
		seen[match.Transaction.ID] = true
		transactions = append(transactions, match.Transaction)
	}
	return transactions
}

func multiDeliveryConsolidationNote(charges []float64) string {
	formatted := make([]string, len(charges))
	for i, charge := range charges {
		formatted[i] = fmt.Sprintf("$%.2f", math.Abs(charge))
	}
	return fmt.Sprintf("Multi-delivery order (%d charges: %s)", len(charges), strings.Join(formatted, ", "))
}

func countFoundMatches(matches []*matcher.MatchResult) int {
	foundCount := 0
	for _, match := range matches {
		if match != nil {
			foundCount++
		}
	}
	return foundCount
}

func sumCharges(charges []float64) float64 {
	total := 0.0
	for _, charge := range charges {
		total += charge
	}
	return math.Round(total*100) / 100
}

func (h *WalmartHandler) processRefundOnlyOrder(ctx context.Context, order WalmartOrder, monarchTxns []*monarch.Transaction, usedTxnIDs map[string]bool, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory, refundCharges []float64, refundItems []providers.OrderItem, dryRun bool) (*ProcessResult, error) {
	result := &ProcessResult{}
	if len(refundItems) == 0 {
		result.Skipped = true
		result.SkipReason = "refund item not identified"
		return result, nil
	}
	refunds, err := h.processRefundCharges(ctx, order, monarchTxns, usedTxnIDs, catCategories, monarchCategories, refundCharges, refundItems, dryRun)
	if err != nil {
		result.Skipped = true
		result.SkipReason = err.Error()
		return result, nil
	}
	result.Processed = true
	result.Refunds = refunds
	if len(refunds) > 0 {
		result.Transaction = refunds[0].Transaction
		result.Splits = refunds[0].Splits
	}
	return result, nil
}

func (h *WalmartHandler) processRefundCharges(ctx context.Context, order WalmartOrder, monarchTxns []*monarch.Transaction, usedTxnIDs map[string]bool, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory, refundCharges []float64, refundItems []providers.OrderItem, dryRun bool) ([]RefundProcessResult, error) {
	refunds := make([]RefundProcessResult, 0, len(refundCharges))
	for _, amount := range refundCharges {
		refundOrder := newRefundOrderView(order, amount, refundItems)
		matchResult, err := h.matcher.FindMatch(refundOrder, monarchTxns, usedTxnIDs)
		if err != nil {
			return refunds, fmt.Errorf("refund match error: %w", err)
		}
		if matchResult == nil {
			return refunds, fmt.Errorf("no matching refund transaction found for amount %.2f", amount)
		}
		usedTxnIDs[matchResult.Transaction.ID] = true
		h.logInfo("Matched refund transaction", "order_id", order.GetID(), "transaction_id", matchResult.Transaction.ID, "refund_amount", amount, "date_diff_days", matchResult.DateDiff)
		refundResult, err := h.categorizeAndApplySplits(ctx, refundOrder, matchResult.Transaction, catCategories, monarchCategories, dryRun)
		if err != nil {
			return refunds, fmt.Errorf("refund split creation error: %w", err)
		}
		refunds = append(refunds, RefundProcessResult{Amount: amount, Transaction: matchResult.Transaction, Splits: refundResult.Splits})
	}
	return refunds, nil
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

	result.Transaction = transaction
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

type refundOrderView struct {
	WalmartOrder
	refundAmount float64
	items        []providers.OrderItem
}

func newRefundOrderView(order WalmartOrder, refundAmount float64, items []providers.OrderItem) *refundOrderView {
	return &refundOrderView{
		WalmartOrder: order,
		refundAmount: refundAmount,
		items:        scaleRefundItems(items, refundAmount),
	}
}

func (r *refundOrderView) GetTotal() float64 {
	return -math.Abs(r.refundAmount)
}

func (r *refundOrderView) GetSubtotal() float64 {
	return math.Abs(r.refundAmount)
}

func (r *refundOrderView) GetTax() float64 {
	return 0
}

func (r *refundOrderView) GetTip() float64 {
	return 0
}

func (r *refundOrderView) GetFees() float64 {
	return 0
}

func (r *refundOrderView) GetItems() []providers.OrderItem {
	return r.items
}

func scaleRefundItems(items []providers.OrderItem, refundAmount float64) []providers.OrderItem {
	if len(items) == 0 {
		return []providers.OrderItem{refundOrderItem{name: "Walmart refund", price: math.Abs(refundAmount), quantity: 1}}
	}

	itemTotal := 0.0
	for _, item := range items {
		itemTotal += math.Abs(item.GetPrice())
	}
	if itemTotal == 0 {
		return []providers.OrderItem{refundOrderItem{name: "Walmart refund", price: math.Abs(refundAmount), quantity: 1}}
	}

	scaled := make([]providers.OrderItem, 0, len(items))
	for _, item := range items {
		scaledPrice := math.Abs(item.GetPrice()) / itemTotal * math.Abs(refundAmount)
		scaled = append(scaled, refundOrderItem{
			original: item,
			name:     item.GetName(),
			price:    scaledPrice,
			quantity: item.GetQuantity(),
		})
	}
	return scaled
}

type refundOrderItem struct {
	original providers.OrderItem
	name     string
	price    float64
	quantity float64
}

func (i refundOrderItem) GetName() string {
	return i.name
}

func (i refundOrderItem) GetPrice() float64 {
	return i.price
}

func (i refundOrderItem) GetQuantity() float64 {
	if i.quantity == 0 {
		return 1
	}
	return i.quantity
}

func (i refundOrderItem) GetUnitPrice() float64 {
	quantity := i.GetQuantity()
	if quantity == 0 {
		return i.price
	}
	return i.price / quantity
}

func (i refundOrderItem) GetDescription() string {
	if i.original == nil {
		return ""
	}
	return i.original.GetDescription()
}

func (i refundOrderItem) GetSKU() string {
	if i.original == nil {
		return ""
	}
	return i.original.GetSKU()
}

func (i refundOrderItem) GetCategory() string {
	if i.original == nil {
		return ""
	}
	return i.original.GetCategory()
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
			ChargedDates []time.Time
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
			ChargedDates: pm.ChargedDates,
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
