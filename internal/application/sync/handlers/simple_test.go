package handlers

import (
	"context"
	"fmt"
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
// Test Mocks (Simple-specific, prefixed to avoid collision)
// =============================================================================

// simpleTestOrder implements providers.Order for testing
type simpleTestOrder struct {
	id           string
	date         time.Time
	total        float64
	subtotal     float64
	tax          float64
	items        []providers.OrderItem
	providerName string
}

func (m *simpleTestOrder) GetID() string                   { return m.id }
func (m *simpleTestOrder) GetDate() time.Time              { return m.date }
func (m *simpleTestOrder) GetTotal() float64               { return m.total }
func (m *simpleTestOrder) GetSubtotal() float64            { return m.subtotal }
func (m *simpleTestOrder) GetTax() float64                 { return m.tax }
func (m *simpleTestOrder) GetTip() float64                 { return 0 }
func (m *simpleTestOrder) GetFees() float64                { return 0 }
func (m *simpleTestOrder) GetItems() []providers.OrderItem { return m.items }
func (m *simpleTestOrder) GetProviderName() string         { return m.providerName }
func (m *simpleTestOrder) GetRawData() interface{}         { return nil }

// simpleTestItem implements providers.OrderItem
type simpleTestItem struct {
	name     string
	price    float64
	quantity float64
}

func (m *simpleTestItem) GetName() string        { return m.name }
func (m *simpleTestItem) GetPrice() float64      { return m.price }
func (m *simpleTestItem) GetQuantity() float64   { return m.quantity }
func (m *simpleTestItem) GetUnitPrice() float64  { return m.price / m.quantity }
func (m *simpleTestItem) GetDescription() string { return "" }
func (m *simpleTestItem) GetSKU() string         { return "" }
func (m *simpleTestItem) GetCategory() string    { return "" }

// simpleTestSplitter implements CategorySplitter
type simpleTestSplitter struct {
	splits     []*monarch.TransactionSplit
	categoryID string
	notes      string
	err        error
}

func (m *simpleTestSplitter) CreateSplits(ctx context.Context, order providers.Order, transaction *monarch.Transaction, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory) ([]*monarch.TransactionSplit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.splits, nil
}

func (m *simpleTestSplitter) GetSingleCategoryInfo(ctx context.Context, order providers.Order, categories []categorizer.Category) (string, string, error) {
	return m.categoryID, m.notes, nil
}

// simpleTestMonarch implements MonarchClient
type simpleTestMonarch struct {
	updateCalled      bool
	updateSplitsCaled bool
	err               error
}

func (m *simpleTestMonarch) UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error {
	m.updateCalled = true
	return m.err
}

func (m *simpleTestMonarch) UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error {
	m.updateSplitsCaled = true
	return m.err
}

// =============================================================================
// Test Helpers
// =============================================================================

func simpleToMonarchDate(t time.Time) monarch.Date {
	return monarch.Date{Time: t}
}

func createTestSimpleHandler(t *testing.T, splitter *simpleTestSplitter, monarch *simpleTestMonarch) *SimpleHandler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	matcherCfg := matcher.Config{
		AmountTolerance: 0.01,
		DateTolerance:   5,
	}
	return NewSimpleHandler(
		matcher.NewMatcher(matcherCfg),
		splitter,
		monarch,
		logger,
	)
}

// =============================================================================
// Tests: Basic Order Processing
// =============================================================================

func TestSimpleHandler_ProcessOrder_Success(t *testing.T) {
	splitter := &simpleTestSplitter{categoryID: "groceries", notes: "Groceries: Milk, Bread"}
	monarchClient := &simpleTestMonarch{}
	handler := createTestSimpleHandler(t, splitter, monarchClient)

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-001",
		date:         orderDate,
		total:        50.00,
		subtotal:     45.00,
		tax:          5.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&simpleTestItem{name: "Milk", price: 5.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: simpleToMonarchDate(orderDate)},
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

func TestSimpleHandler_ProcessOrder_NoMatch_Skipped(t *testing.T) {
	handler := createTestSimpleHandler(t, nil, nil)

	order := &simpleTestOrder{
		id:           "ORDER-NOMATCH",
		date:         time.Now(),
		total:        50.00,
		providerName: "Costco",
	}

	// No matching transactions (wrong amount)
	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -100.00, Date: simpleToMonarchDate(time.Now())},
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

func TestSimpleHandler_ProcessOrder_TransactionAlreadyHasSplits_Skipped(t *testing.T) {
	handler := createTestSimpleHandler(t, nil, nil)

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-SPLITS",
		date:         orderDate,
		total:        50.00,
		providerName: "Costco",
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: simpleToMonarchDate(orderDate), HasSplits: true},
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
	assert.Contains(t, result.SkipReason, "already has splits")
}

// =============================================================================
// Tests: Date and Amount Tolerance
// =============================================================================

func TestSimpleHandler_ProcessOrder_DateTolerance(t *testing.T) {
	splitter := &simpleTestSplitter{categoryID: "groceries", notes: "Groceries"}
	handler := createTestSimpleHandler(t, splitter, &simpleTestMonarch{})

	orderDate := time.Now()
	// Transaction is 3 days after order (within 5 day tolerance)
	txnDate := orderDate.AddDate(0, 0, 3)

	order := &simpleTestOrder{
		id:           "ORDER-DATE",
		date:         orderDate,
		total:        50.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&simpleTestItem{name: "Item", price: 45.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: simpleToMonarchDate(txnDate)},
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

func TestSimpleHandler_ProcessOrder_OutsideDateTolerance(t *testing.T) {
	handler := createTestSimpleHandler(t, nil, nil)

	orderDate := time.Now()
	// Transaction is 10 days after order (outside 5 day tolerance)
	txnDate := orderDate.AddDate(0, 0, 10)

	order := &simpleTestOrder{
		id:           "ORDER-DATE-OUT",
		date:         orderDate,
		total:        50.00,
		providerName: "Costco",
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: simpleToMonarchDate(txnDate)},
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
}

func TestSimpleHandler_ProcessOrder_AmountTolerance(t *testing.T) {
	splitter := &simpleTestSplitter{categoryID: "groceries", notes: "Groceries"}
	handler := createTestSimpleHandler(t, splitter, &simpleTestMonarch{})

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-AMT",
		date:         orderDate,
		total:        50.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&simpleTestItem{name: "Item", price: 45.00, quantity: 1}},
	}

	// Transaction is $0.005 off (within $0.01 tolerance)
	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.005, Date: simpleToMonarchDate(orderDate)},
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
// Tests: Split Application
// =============================================================================

func TestSimpleHandler_ProcessOrder_AppliesSplits(t *testing.T) {
	splits := []*monarch.TransactionSplit{
		{CategoryID: "cat-1", Amount: 30.00, Notes: "Groceries"},
		{CategoryID: "cat-2", Amount: 20.00, Notes: "Household"},
	}
	splitter := &simpleTestSplitter{splits: splits}
	monarchClient := &simpleTestMonarch{}
	handler := createTestSimpleHandler(t, splitter, monarchClient)

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-SPLITS",
		date:         orderDate,
		total:        50.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&simpleTestItem{name: "Item", price: 45.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: simpleToMonarchDate(orderDate)},
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

func TestSimpleHandler_ProcessOrder_DryRun_DoesNotApply(t *testing.T) {
	splitter := &simpleTestSplitter{categoryID: "groceries", notes: "Groceries"}
	monarchClient := &simpleTestMonarch{}
	handler := createTestSimpleHandler(t, splitter, monarchClient)

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-DRYRUN",
		date:         orderDate,
		total:        50.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&simpleTestItem{name: "Item", price: 45.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: simpleToMonarchDate(orderDate)},
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

func TestSimpleHandler_ProcessOrder_TransactionAlreadyUsed(t *testing.T) {
	handler := createTestSimpleHandler(t, nil, nil)

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-USED",
		date:         orderDate,
		total:        50.00,
		providerName: "Costco",
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: simpleToMonarchDate(orderDate)},
	}

	// Mark transaction as already used
	usedTxnIDs := map[string]bool{"txn-1": true}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		usedTxnIDs,
		nil, nil,
		true,
	)

	require.NoError(t, err)
	assert.False(t, result.Processed)
	assert.True(t, result.Skipped)
	assert.Contains(t, result.SkipReason, "no matching transaction")
}

// =============================================================================
// Tests: SplitDetails Population (TDD - Red Phase)
// =============================================================================

func TestSimpleHandler_ProcessOrder_PopulatesSplitDetailsAfterSuccess(t *testing.T) {
	// Create splits with category info
	splits := []*monarch.TransactionSplit{
		{CategoryID: "cat-groceries", Amount: -30.00, Notes: "Groceries:\n- Milk $5.00\n- Bread $5.00"},
		{CategoryID: "cat-household", Amount: -20.00, Notes: "Household:\n- Soap $10.00"},
	}

	// Create splitter that also returns split details with items
	splitter := &simpleTestSplitterWithDetails{
		splits: splits,
		splitDetails: []SplitDetail{
			{
				CategoryID:   "cat-groceries",
				CategoryName: "Groceries",
				Amount:       30.00,
				Items: []SplitDetailItem{
					{Name: "Milk", Quantity: 1, UnitPrice: 5.00, TotalPrice: 5.00},
					{Name: "Bread", Quantity: 1, UnitPrice: 5.00, TotalPrice: 5.00},
				},
			},
			{
				CategoryID:   "cat-household",
				CategoryName: "Household",
				Amount:       20.00,
				Items: []SplitDetailItem{
					{Name: "Soap", Quantity: 1, UnitPrice: 10.00, TotalPrice: 10.00},
				},
			},
		},
	}
	monarchClient := &simpleTestMonarch{}
	handler := createTestSimpleHandlerWithDetailedSplitter(t, splitter, monarchClient)

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-SPLITS-DETAILS",
		date:         orderDate,
		total:        50.00,
		subtotal:     45.00,
		tax:          5.00,
		providerName: "Costco",
		items: []providers.OrderItem{
			&simpleTestItem{name: "Milk", price: 5.00, quantity: 1},
			&simpleTestItem{name: "Bread", price: 5.00, quantity: 1},
			&simpleTestItem{name: "Soap", price: 10.00, quantity: 1},
		},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: simpleToMonarchDate(orderDate)},
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		false, // NOT dry run - should call Monarch and populate SplitDetails
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.True(t, monarchClient.updateSplitsCaled, "Should have called UpdateSplits")

	// Verify SplitDetails were populated AFTER successful API call
	require.NotNil(t, result.SplitDetails, "SplitDetails should be populated after successful sync")
	require.Len(t, result.SplitDetails, 2, "Should have 2 split details")

	// Verify first split details
	groceriesSplit := findSplitDetailByCategory(result.SplitDetails, "cat-groceries")
	require.NotNil(t, groceriesSplit, "Should have groceries split detail")
	assert.Equal(t, "Groceries", groceriesSplit.CategoryName)
	assert.Equal(t, 30.00, groceriesSplit.Amount)
	assert.Len(t, groceriesSplit.Items, 2, "Groceries split should have 2 items")

	// Verify second split details
	householdSplit := findSplitDetailByCategory(result.SplitDetails, "cat-household")
	require.NotNil(t, householdSplit, "Should have household split detail")
	assert.Equal(t, "Household", householdSplit.CategoryName)
	assert.Equal(t, 20.00, householdSplit.Amount)
	assert.Len(t, householdSplit.Items, 1, "Household split should have 1 item")
}

func TestSimpleHandler_ProcessOrder_NoSplitDetailsOnDryRun(t *testing.T) {
	splits := []*monarch.TransactionSplit{
		{CategoryID: "cat-groceries", Amount: -30.00, Notes: "Groceries"},
	}
	splitter := &simpleTestSplitterWithDetails{
		splits: splits,
		splitDetails: []SplitDetail{
			{CategoryID: "cat-groceries", CategoryName: "Groceries", Amount: 30.00},
		},
	}
	monarchClient := &simpleTestMonarch{}
	handler := createTestSimpleHandlerWithDetailedSplitter(t, splitter, monarchClient)

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-DRYRUN-DETAILS",
		date:         orderDate,
		total:        30.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&simpleTestItem{name: "Milk", price: 25.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -30.00, Date: simpleToMonarchDate(orderDate)},
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		true, // DRY RUN - should NOT populate SplitDetails since Monarch wasn't actually called
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.False(t, monarchClient.updateSplitsCaled, "Should NOT have called UpdateSplits in dry run")
	// SplitDetails should still be nil or empty in dry run mode
	// because we only save them after SUCCESSFUL Monarch API call
	assert.Empty(t, result.SplitDetails, "SplitDetails should be empty on dry run")
}

func TestSimpleHandler_ProcessOrder_NoSplitDetailsOnMonarchError(t *testing.T) {
	splits := []*monarch.TransactionSplit{
		{CategoryID: "cat-groceries", Amount: -30.00, Notes: "Groceries"},
	}
	splitter := &simpleTestSplitterWithDetails{
		splits: splits,
		splitDetails: []SplitDetail{
			{CategoryID: "cat-groceries", CategoryName: "Groceries", Amount: 30.00},
		},
	}
	// Monarch client that returns an error
	monarchClient := &simpleTestMonarch{err: fmt.Errorf("API error: rate limited")}
	handler := createTestSimpleHandlerWithDetailedSplitter(t, splitter, monarchClient)

	orderDate := time.Now()
	order := &simpleTestOrder{
		id:           "ORDER-API-ERROR",
		date:         orderDate,
		total:        30.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&simpleTestItem{name: "Milk", price: 25.00, quantity: 1}},
	}

	txns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -30.00, Date: simpleToMonarchDate(orderDate)},
	}

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		txns,
		make(map[string]bool),
		nil, nil,
		false, // NOT dry run
	)

	// Should return error because Monarch API failed
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update splits error")

	// Result may be nil or SplitDetails should be empty since API failed
	if result != nil {
		assert.Empty(t, result.SplitDetails, "SplitDetails should NOT be populated when Monarch API fails")
	}
}

// Helper function to find a split detail by category ID
func findSplitDetailByCategory(details []SplitDetail, categoryID string) *SplitDetail {
	for _, d := range details {
		if d.CategoryID == categoryID {
			return &d
		}
	}
	return nil
}

// simpleTestSplitterWithDetails is a mock splitter that also returns SplitDetails
type simpleTestSplitterWithDetails struct {
	splits       []*monarch.TransactionSplit
	splitDetails []SplitDetail
	categoryID   string
	notes        string
	err          error
}

func (m *simpleTestSplitterWithDetails) CreateSplits(ctx context.Context, order providers.Order, transaction *monarch.Transaction, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory) ([]*monarch.TransactionSplit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.splits, nil
}

func (m *simpleTestSplitterWithDetails) GetSingleCategoryInfo(ctx context.Context, order providers.Order, categories []categorizer.Category) (string, string, error) {
	return m.categoryID, m.notes, nil
}

func (m *simpleTestSplitterWithDetails) GetSplitDetails() []SplitDetail {
	return m.splitDetails
}

func createTestSimpleHandlerWithDetailedSplitter(t *testing.T, splitter CategorySplitterWithDetails, monarch *simpleTestMonarch) *SimpleHandler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	matcherCfg := matcher.Config{
		AmountTolerance: 0.01,
		DateTolerance:   5,
	}
	return NewSimpleHandler(
		matcher.NewMatcher(matcherCfg),
		splitter,
		monarch,
		logger,
	)
}
