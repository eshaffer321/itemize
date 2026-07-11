package handlers

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/eshaffer321/itemize/internal/domain/categorizer"
	"github.com/eshaffer321/itemize/internal/domain/matcher"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Mocks (Walmart-specific, prefixed to avoid collision with amazon_test.go)
// =============================================================================

// walmartTestOrder implements WalmartOrder for testing
type walmartTestOrder struct {
	id             string
	date           time.Time
	total          float64
	subtotal       float64
	tax            float64
	items          []providers.OrderItem
	charges        []float64
	refundCharges  []float64
	refundItems    []providers.OrderItem
	chargesErr     error
	refundsErr     error
	isMultiDeliver bool
}

func (m *walmartTestOrder) GetID() string                   { return m.id }
func (m *walmartTestOrder) GetDate() time.Time              { return m.date }
func (m *walmartTestOrder) GetTotal() float64               { return m.total }
func (m *walmartTestOrder) GetSubtotal() float64            { return m.subtotal }
func (m *walmartTestOrder) GetTax() float64                 { return m.tax }
func (m *walmartTestOrder) GetTip() float64                 { return 0 }
func (m *walmartTestOrder) GetFees() float64                { return 0 }
func (m *walmartTestOrder) GetItems() []providers.OrderItem { return m.items }
func (m *walmartTestOrder) GetProviderName() string         { return "walmart" }
func (m *walmartTestOrder) GetRawData() interface{}         { return nil }

func (m *walmartTestOrder) GetFinalCharges() ([]float64, error) {
	if m.chargesErr != nil {
		return nil, m.chargesErr
	}
	return m.charges, nil
}

func (m *walmartTestOrder) GetRefundCharges() ([]float64, error) {
	if m.refundsErr != nil {
		return nil, m.refundsErr
	}
	return m.refundCharges, nil
}

func (m *walmartTestOrder) GetRefundItems() ([]providers.OrderItem, error) {
	return m.refundItems, nil
}

func (m *walmartTestOrder) IsMultiDelivery() (bool, error) {
	return m.isMultiDeliver, nil
}

// walmartTestItem implements providers.OrderItem
type walmartTestItem struct {
	name     string
	price    float64
	quantity float64
}

func (m *walmartTestItem) GetName() string        { return m.name }
func (m *walmartTestItem) GetPrice() float64      { return m.price }
func (m *walmartTestItem) GetQuantity() float64   { return m.quantity }
func (m *walmartTestItem) GetUnitPrice() float64  { return m.price / m.quantity }
func (m *walmartTestItem) GetDescription() string { return "" }
func (m *walmartTestItem) GetSKU() string         { return "" }
func (m *walmartTestItem) GetCategory() string    { return "" }

// walmartTestSplitter implements CategorySplitter
type walmartTestSplitter struct {
	splits     []*monarch.TransactionSplit
	categoryID string
	notes      string
	err        error
	calls      []walmartSplitterCall
}

type walmartSplitterCall struct {
	orderTotal        float64
	transactionID     string
	transactionAmount float64
	itemTotal         float64
	itemNames         []string
}

func (m *walmartTestSplitter) CreateSplits(ctx context.Context, order providers.Order, transaction *monarch.Transaction, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory) ([]*monarch.TransactionSplit, error) {
	if m.err != nil {
		return nil, m.err
	}
	itemTotal := 0.0
	itemNames := make([]string, 0, len(order.GetItems()))
	for _, item := range order.GetItems() {
		itemTotal += item.GetPrice()
		itemNames = append(itemNames, item.GetName())
	}
	m.calls = append(m.calls, walmartSplitterCall{
		orderTotal:        order.GetTotal(),
		transactionID:     transaction.ID,
		transactionAmount: transaction.Amount,
		itemTotal:         itemTotal,
		itemNames:         itemNames,
	})
	return m.splits, nil
}

func (m *walmartTestSplitter) GetSingleCategoryInfo(ctx context.Context, order providers.Order, categories []categorizer.Category) (string, string, error) {
	return m.categoryID, m.notes, nil
}

// walmartTestMonarch implements MonarchClient
type walmartTestMonarch struct {
	updateCalled      bool
	updateSplitsCaled bool
	updateCount       int
	updateSplitsCount int
	err               error
}

func (m *walmartTestMonarch) UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error {
	m.updateCalled = true
	m.updateCount++
	return m.err
}

func (m *walmartTestMonarch) UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error {
	m.updateSplitsCaled = true
	m.updateSplitsCount++
	return m.err
}

// walmartTestConsolidator implements TransactionConsolidator
type walmartTestConsolidator struct {
	result *ConsolidationResult
	err    error
}

func (m *walmartTestConsolidator) ConsolidateTransactions(ctx context.Context, transactions []*monarch.Transaction, order providers.Order, dryRun bool) (*ConsolidationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// =============================================================================
// Test Helpers
// =============================================================================

func walmartToMonarchDate(t time.Time) monarch.Date {
	return monarch.Date{Time: t}
}

func createTestWalmartHandler(t *testing.T, splitter *walmartTestSplitter, consolidator *walmartTestConsolidator, monarch *walmartTestMonarch) *WalmartHandler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	matcherCfg := matcher.Config{
		AmountTolerance: 0.01,
		DateTolerance:   5,
	}
	return NewWalmartHandler(
		matcher.NewMatcher(matcherCfg),
		consolidator,
		splitter,
		monarch,
		logger,
	)
}

// =============================================================================
// Tests: Regular Order Processing
// =============================================================================

func TestWalmartHandler_ProcessOrder_SingleCharge_Success(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries: Milk, Bread"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:       "ORDER-001",
		date:     orderDate,
		total:    50.00,
		subtotal: 45.00,
		tax:      5.00,
		items:    []providers.OrderItem{&walmartTestItem{name: "Milk", price: 5.00, quantity: 1}},
		charges:  []float64{50.00}, // Single charge = order total
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: walmartToMonarchDate(orderDate)},
	}

	usedTxnIDs := make(map[string]bool)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		usedTxnIDs,
		nil, nil,
		true, // dry run
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.False(t, result.Skipped)
	assert.True(t, usedTxnIDs["txn-1"])
}

func TestWalmartHandler_ProcessOrder_PendingOrder_Skipped(t *testing.T) {
	handler := createTestWalmartHandler(t, nil, nil, nil)

	order := &walmartTestOrder{
		id:         "ORDER-PENDING",
		date:       time.Now(),
		total:      50.00,
		chargesErr: errors.New("order not yet charged (payment pending)"),
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		nil,
		make(map[string]bool),
		nil, nil,
		true,
	)

	require.NoError(t, err)
	assert.False(t, result.Processed)
	assert.True(t, result.Skipped)
	assert.Equal(t, "payment pending", result.SkipReason)
}

func TestWalmartHandler_ProcessOrder_LedgerError_FallbackToOrderTotal(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:         "ORDER-ERR",
		date:       orderDate,
		total:      50.00,
		subtotal:   45.00,
		tax:        5.00,
		items:      []providers.OrderItem{&walmartTestItem{name: "Item", price: 45.00, quantity: 1}},
		chargesErr: errors.New("network error"), // Non-pending error
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: walmartToMonarchDate(orderDate)},
	}

	usedTxnIDs := make(map[string]bool)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		usedTxnIDs,
		nil, nil,
		true,
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.True(t, usedTxnIDs["txn-1"])
}

// =============================================================================
// Tests: Gift Card / Ledger Amount Matching
// =============================================================================

func TestWalmartHandler_ProcessOrder_GiftCard_UsesLedgerAmount(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	// Order total is $100, but only $70 charged to bank (rest was gift card)
	order := &walmartTestOrder{
		id:       "ORDER-GC",
		date:     orderDate,
		total:    100.00,
		subtotal: 90.00,
		tax:      10.00,
		items:    []providers.OrderItem{&walmartTestItem{name: "Item", price: 90.00, quantity: 1}},
		charges:  []float64{70.00}, // Only $70 charged
	}

	txns := []*monarch.Transaction{
		{ID: "txn-gc", Amount: -70.00, Date: walmartToMonarchDate(orderDate)},     // Matches ledger amount
		{ID: "txn-other", Amount: -100.00, Date: walmartToMonarchDate(orderDate)}, // Would match order total
	}

	usedTxnIDs := make(map[string]bool)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		usedTxnIDs,
		nil, nil,
		true,
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	// Should match the $70 transaction, not the $100 one
	assert.True(t, usedTxnIDs["txn-gc"], "Should match using ledger amount")
	assert.False(t, usedTxnIDs["txn-other"], "Should not match order total amount")
}

func TestWalmartHandler_ProcessOrder_ProcessesRefundCharge(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:            "ORDER-REFUND",
		date:          orderDate,
		total:         86.06,
		subtotal:      80.00,
		tax:           6.06,
		items:         []providers.OrderItem{&walmartTestItem{name: "Milk", price: 50.00, quantity: 1}, &walmartTestItem{name: "Bread", price: 30.00, quantity: 1}},
		charges:       []float64{86.06},
		refundCharges: []float64{5.58},
		refundItems:   []providers.OrderItem{&walmartTestItem{name: "Milk", price: 5.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "purchase-txn", Amount: -86.06, Date: walmartToMonarchDate(orderDate)},
		{ID: "refund-txn", Amount: 5.58, Date: walmartToMonarchDate(orderDate.AddDate(0, 0, 1))},
	}

	usedTxnIDs := make(map[string]bool)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		usedTxnIDs,
		nil, nil,
		false,
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.True(t, usedTxnIDs["purchase-txn"], "purchase transaction should be matched")
	assert.True(t, usedTxnIDs["refund-txn"], "refund credit transaction should be matched")
	require.Len(t, result.Refunds, 1)
	assert.Equal(t, "refund-txn", result.Refunds[0].Transaction.ID)
	assert.Equal(t, 2, monarchClient.updateCount, "purchase and refund should both be categorized")
	require.Len(t, splitter.calls, 2)
	assert.Equal(t, -5.58, splitter.calls[1].orderTotal, "refund order view should match positive Monarch credits")
	assert.InDelta(t, 5.58, splitter.calls[1].itemTotal, 0.01, "refund item prices should be scaled to the refund amount")
}

func TestWalmartHandler_ProcessOrder_CategorizesIdentifiedRefundItemOnly(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries: Chobani creamer"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:            "ORDER-REFUND-ITEM",
		date:          orderDate,
		total:         20,
		items:         []providers.OrderItem{&walmartTestItem{name: "Unrelated item", price: 14.74, quantity: 1}, &walmartTestItem{name: "Chobani Coffee Creamer", price: 5.26, quantity: 1}},
		charges:       []float64{20},
		refundCharges: []float64{5.58},
		refundItems:   []providers.OrderItem{&walmartTestItem{name: "Chobani Coffee Creamer", price: 5.26, quantity: 1}},
	}
	txns := []*monarch.Transaction{
		{ID: "purchase", Amount: -20, Date: walmartToMonarchDate(orderDate)},
		{ID: "refund", Amount: 5.58, Date: walmartToMonarchDate(orderDate)},
	}

	_, err := handler.ProcessOrder(context.Background(), order, txns, map[string]bool{}, nil, nil, true)
	require.NoError(t, err)
	require.Len(t, splitter.calls, 2)
	assert.Equal(t, []string{"Chobani Coffee Creamer"}, splitter.calls[1].itemNames)
	assert.InDelta(t, 5.58, splitter.calls[1].itemTotal, 0.01, "refund amount should include the refunded item's tax")
}

func TestWalmartHandler_ProcessOrder_ProcessesRefundOnlyLedger(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:            "ORDER-FULL-REFUND",
		date:          orderDate,
		total:         15.00,
		subtotal:      15.00,
		items:         []providers.OrderItem{&walmartTestItem{name: "Returned item", price: 15.00, quantity: 1}},
		chargesErr:    errors.New("no positive charges found (order may be fully refunded or paid entirely with gift card)"),
		refundCharges: []float64{15.00},
		refundItems:   []providers.OrderItem{&walmartTestItem{name: "Returned item", price: 15.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "refund-only-txn", Amount: 15.00, Date: walmartToMonarchDate(orderDate)},
	}

	usedTxnIDs := make(map[string]bool)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		usedTxnIDs,
		nil, nil,
		false,
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.False(t, result.Skipped)
	assert.True(t, usedTxnIDs["refund-only-txn"])
	require.Len(t, result.Refunds, 1)
	assert.Equal(t, "refund-only-txn", result.Transaction.ID)
	assert.Equal(t, 1, monarchClient.updateCount)
}

func TestWalmartHandler_ProcessOrder_ProcessesRefundWhenPurchaseAlreadyConsolidated(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:             "ORDER-REFUND-AFTER-CONSOLIDATION",
		date:           orderDate,
		total:          86.06,
		subtotal:       80.00,
		tax:            6.06,
		items:          []providers.OrderItem{&walmartTestItem{name: "Milk", price: 50.00, quantity: 1}, &walmartTestItem{name: "Bread", price: 30.00, quantity: 1}},
		charges:        []float64{4.63, 3.48, 16.90, 5.80, 47.73, 5.27, 2.25},
		refundCharges:  []float64{5.58},
		refundItems:    []providers.OrderItem{&walmartTestItem{name: "Milk", price: 5.00, quantity: 1}},
		isMultiDeliver: true,
	}

	txns := []*monarch.Transaction{
		// This is the already-consolidated purchase. It no longer matches the
		// individual ledger charge rows during a forced historical rerun.
		{ID: "consolidated-purchase", Amount: -86.06, Date: walmartToMonarchDate(orderDate)},
		{ID: "refund-txn", Amount: 5.58, Date: walmartToMonarchDate(orderDate.AddDate(0, 0, 1))},
	}

	usedTxnIDs := make(map[string]bool)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		usedTxnIDs,
		nil, nil,
		false,
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.False(t, result.Skipped)
	assert.False(t, usedTxnIDs["consolidated-purchase"], "already-consolidated purchase should not be forced into component matching")
	assert.True(t, usedTxnIDs["refund-txn"], "refund should still be processed")
	require.Len(t, result.Refunds, 1)
	assert.Equal(t, "refund-txn", result.Transaction.ID)
	assert.Equal(t, 1, monarchClient.updateCount)
}

func TestWalmartHandler_ProcessOrder_NoMatch_ReturnsSkipped(t *testing.T) {
	handler := createTestWalmartHandler(t, nil, nil, nil)

	order := &walmartTestOrder{
		id:      "ORDER-NOMATCH",
		date:    time.Now(),
		total:   50.00,
		charges: []float64{50.00},
	}

	// No matching transactions
	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -100.00, Date: walmartToMonarchDate(time.Now())},
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		true,
	)

	require.NoError(t, err)
	assert.False(t, result.Processed)
	assert.True(t, result.Skipped)
	assert.Contains(t, result.SkipReason, "no matching transaction")
}

// =============================================================================
// Tests: Multi-Delivery Orders
// =============================================================================

func TestWalmartHandler_ProcessOrder_MultiDelivery_Success(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &walmartTestMonarch{}
	consolidator := &walmartTestConsolidator{
		result: &ConsolidationResult{
			ConsolidatedTransaction: &monarch.Transaction{ID: "consolidated-txn", Amount: -150.00},
		},
	}
	handler := createTestWalmartHandler(t, splitter, consolidator, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:             "ORDER-MD",
		date:           orderDate,
		total:          150.00,
		subtotal:       140.00,
		tax:            10.00,
		items:          []providers.OrderItem{&walmartTestItem{name: "Item", price: 140.00, quantity: 1}},
		charges:        []float64{80.00, 70.00}, // Two charges
		isMultiDeliver: true,
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -80.00, Date: walmartToMonarchDate(orderDate)},
		{ID: "txn-2", Amount: -70.00, Date: walmartToMonarchDate(orderDate.AddDate(0, 0, 1))},
	}

	usedTxnIDs := make(map[string]bool)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		usedTxnIDs,
		nil, nil,
		true,
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.True(t, usedTxnIDs["txn-1"])
	assert.True(t, usedTxnIDs["txn-2"])
}

func TestWalmartHandler_ProcessOrder_MultiDelivery_MissingTransaction(t *testing.T) {
	handler := createTestWalmartHandler(t, nil, nil, nil)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:             "ORDER-MD-MISSING",
		date:           orderDate,
		total:          150.00,
		charges:        []float64{80.00, 70.00}, // Two charges expected
		isMultiDeliver: true,
	}

	// Only one matching transaction
	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -80.00, Date: walmartToMonarchDate(orderDate)},
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		true,
	)

	require.NoError(t, err)
	assert.False(t, result.Processed)
	assert.True(t, result.Skipped)
	assert.Contains(t, result.SkipReason, "could not find all transactions")
}

func TestWalmartHandler_ProcessOrder_MultiDelivery_ErrorMessageReportsActualFoundCount(t *testing.T) {
	// This test verifies the bug fix: when some charges have no matching transactions,
	// the error message should report the actual number found, not len(Matches) which
	// includes nil entries for maintaining index alignment.
	handler := createTestWalmartHandler(t, nil, nil, nil)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:             "ORDER-MD-PARTIAL",
		date:           orderDate,
		total:          150.00,
		charges:        []float64{80.00, 70.00}, // Two charges expected
		isMultiDeliver: true,
	}

	// Only one matching transaction - should find 1 of 2
	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -80.00, Date: walmartToMonarchDate(orderDate)},
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		true,
	)

	require.NoError(t, err)
	assert.True(t, result.Skipped)
	// The bug was reporting "expected 2, found 2" because len(Matches) includes nil entries
	// The fix should report the actual count of non-nil matches
	assert.Contains(t, result.SkipReason, "expected 2, found 1",
		"Error message should report actual found count (1), not slice length (2). Got: %s", result.SkipReason)
}

// =============================================================================
// Tests: Split Application
// =============================================================================

func TestWalmartHandler_ProcessOrder_AppliesSplits(t *testing.T) {
	splits := []*monarch.TransactionSplit{
		{CategoryID: "cat-1", Amount: 30.00, Notes: "Groceries"},
		{CategoryID: "cat-2", Amount: 20.00, Notes: "Household"},
	}
	splitter := &walmartTestSplitter{splits: splits}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:      "ORDER-SPLITS",
		date:    orderDate,
		total:   50.00,
		charges: []float64{50.00},
		items:   []providers.OrderItem{&walmartTestItem{name: "Item", price: 45.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: walmartToMonarchDate(orderDate)},
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		false, // Not dry run - should call monarch
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.Equal(t, splits, result.Splits)
	assert.True(t, monarchClient.updateSplitsCaled, "Should have called UpdateSplits")
}

func TestWalmartHandler_ProcessOrder_DryRun_DoesNotApply(t *testing.T) {
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:      "ORDER-DRYRUN",
		date:    orderDate,
		total:   50.00,
		charges: []float64{50.00},
		items:   []providers.OrderItem{&walmartTestItem{name: "Item", price: 45.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: walmartToMonarchDate(orderDate)},
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		true, // Dry run
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.False(t, monarchClient.updateCalled, "Should not have called UpdateTransaction in dry run")
	assert.False(t, monarchClient.updateSplitsCaled, "Should not have called UpdateSplits in dry run")
}

// =============================================================================
// Tests: Transaction Tracking in ProcessResult
// =============================================================================

func TestWalmartHandler_ProcessOrder_ReturnsMatchedTransaction(t *testing.T) {
	// This test verifies that ProcessResult includes the matched transaction
	// so the orchestrator can record transaction_id in the processing_records table.
	// Bug: Previously transaction_id was not being recorded because ProcessResult
	// didn't include the matched transaction.
	splitter := &walmartTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &walmartTestMonarch{}
	handler := createTestWalmartHandler(t, splitter, nil, monarchClient)

	orderDate := time.Now()
	order := &walmartTestOrder{
		id:       "ORDER-TXN-TRACKING",
		date:     orderDate,
		total:    50.00,
		subtotal: 45.00,
		tax:      5.00,
		items:    []providers.OrderItem{&walmartTestItem{name: "Milk", price: 45.00, quantity: 1}},
		charges:  []float64{50.00},
	}

	expectedTxn := &monarch.Transaction{
		ID:     "txn-matched",
		Amount: -50.00,
		Date:   walmartToMonarchDate(orderDate),
	}

	txns := []*monarch.Transaction{expectedTxn}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		true, // dry run
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	require.NotNil(t, result.Transaction, "ProcessResult should include the matched transaction")
	assert.Equal(t, "txn-matched", result.Transaction.ID, "Transaction ID should match")
	assert.Equal(t, -50.00, result.Transaction.Amount, "Transaction amount should match")
}
