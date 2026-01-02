package handlers

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/matcher"
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
	chargesErr     error
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
}

func (m *walmartTestSplitter) CreateSplits(ctx context.Context, order providers.Order, transaction *monarch.Transaction, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory) ([]*monarch.TransactionSplit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.splits, nil
}

func (m *walmartTestSplitter) GetSingleCategoryInfo(ctx context.Context, order providers.Order, categories []categorizer.Category) (string, string, error) {
	return m.categoryID, m.notes, nil
}

// walmartTestMonarch implements MonarchClient
type walmartTestMonarch struct {
	updateCalled      bool
	updateSplitsCaled bool
	err               error
}

func (m *walmartTestMonarch) UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error {
	m.updateCalled = true
	return m.err
}

func (m *walmartTestMonarch) UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error {
	m.updateSplitsCaled = true
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
		{ID: "txn-gc", Amount: -70.00, Date: walmartToMonarchDate(orderDate)}, // Matches ledger amount
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
