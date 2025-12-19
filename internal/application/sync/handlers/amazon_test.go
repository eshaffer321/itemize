package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/matcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toMonarchDate converts time.Time to monarch.Date
func toMonarchDate(t time.Time) monarch.Date {
	return monarch.Date{Time: t}
}

// mockAmazonOrder implements AmazonOrder for testing
type mockAmazonOrder struct {
	id             string
	date           time.Time
	total          float64
	subtotal       float64
	tax            float64
	items          []providers.OrderItem
	bankCharges    []float64
	nonBankAmount  float64
	bankChargesErr error
	nonBankErr     error
}

func (m *mockAmazonOrder) GetID() string                    { return m.id }
func (m *mockAmazonOrder) GetDate() time.Time               { return m.date }
func (m *mockAmazonOrder) GetTotal() float64                { return m.total }
func (m *mockAmazonOrder) GetSubtotal() float64             { return m.subtotal }
func (m *mockAmazonOrder) GetTax() float64                  { return m.tax }
func (m *mockAmazonOrder) GetTip() float64                  { return 0 }
func (m *mockAmazonOrder) GetFees() float64                 { return 0 }
func (m *mockAmazonOrder) GetItems() []providers.OrderItem  { return m.items }
func (m *mockAmazonOrder) GetProviderName() string          { return "Amazon" }
func (m *mockAmazonOrder) GetRawData() interface{}          { return nil }
func (m *mockAmazonOrder) GetFinalCharges() ([]float64, error) {
	return m.bankCharges, m.bankChargesErr
}
func (m *mockAmazonOrder) GetNonBankAmount() (float64, error) {
	return m.nonBankAmount, m.nonBankErr
}
func (m *mockAmazonOrder) IsMultiDelivery() (bool, error) {
	return len(m.bankCharges) > 1, nil
}

// mockItem implements providers.OrderItem
type mockItem struct {
	name  string
	price float64
}

func (m *mockItem) GetName() string        { return m.name }
func (m *mockItem) GetPrice() float64      { return m.price }
func (m *mockItem) GetQuantity() float64   { return 1 }
func (m *mockItem) GetUnitPrice() float64  { return m.price }
func (m *mockItem) GetDescription() string { return "" }
func (m *mockItem) GetSKU() string         { return "" }
func (m *mockItem) GetCategory() string    { return "" }

// mockConsolidator implements TransactionConsolidator
type mockConsolidator struct {
	result *ConsolidationResult
	err    error
}

func (m *mockConsolidator) ConsolidateTransactions(ctx context.Context, transactions []*monarch.Transaction, order providers.Order, dryRun bool) (*ConsolidationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	// Default: return first transaction as consolidated
	if len(transactions) > 0 {
		return &ConsolidationResult{
			ConsolidatedTransaction: transactions[0],
		}, nil
	}
	return nil, nil
}

// mockSplitter implements CategorySplitter
type mockSplitter struct {
	splits     []*monarch.TransactionSplit
	categoryID string
	notes      string
	err        error
}

func (m *mockSplitter) CreateSplits(ctx context.Context, order providers.Order, transaction *monarch.Transaction, catCategories []categorizer.Category, monarchCategories []*monarch.TransactionCategory) ([]*monarch.TransactionSplit, error) {
	return m.splits, m.err
}

func (m *mockSplitter) GetSingleCategoryInfo(ctx context.Context, order providers.Order, categories []categorizer.Category) (string, string, error) {
	return m.categoryID, m.notes, m.err
}

// mockMonarch implements MonarchClient
type mockMonarch struct {
	updateCalled      bool
	updateSplitsCalled bool
	updateErr         error
	updateSplitsErr   error
}

func (m *mockMonarch) UpdateTransaction(ctx context.Context, id string, params *monarch.UpdateTransactionParams) error {
	m.updateCalled = true
	return m.updateErr
}

func (m *mockMonarch) UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error {
	m.updateSplitsCalled = true
	return m.updateSplitsErr
}

func TestAmazonHandler_ProcessOrder_ValidOrder(t *testing.T) {
	// Setup: Real order 112-4559127-2161020
	// Items sum to $107.26, bank charges sum to $103.27
	order := &mockAmazonOrder{
		id:       "112-4559127-2161020",
		date:     time.Now(),
		total:    103.27, // Order total after points
		subtotal: 107.26,
		tax:      0,
		items: []providers.OrderItem{
			&mockItem{name: "Hot Wheels", price: 19.99},
			&mockItem{name: "Item 2", price: 12.72},
			&mockItem{name: "Item 3", price: 16.99},
			&mockItem{name: "Item 4", price: 7.99},
			&mockItem{name: "Peppa Pig", price: 6.49},
			&mockItem{name: "Paw Patrol Stickers", price: 26.99},
			&mockItem{name: "Paw Patrol Racers", price: 16.09},
		},
		bankCharges:   []float64{52.55, 50.72},
		nonBankAmount: 0, // Points already deducted from order total
	}

	// Create matching transactions
	monarchTxns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -52.55, Date: toMonarchDate(time.Now())},
		{ID: "txn-2", Amount: -50.72, Date: toMonarchDate(time.Now())},
	}

	consolidator := &mockConsolidator{
		result: &ConsolidationResult{
			ConsolidatedTransaction: &monarch.Transaction{
				ID:     "consolidated-txn",
				Amount: -103.27,
			},
		},
	}

	splitter := &mockSplitter{
		splits: []*monarch.TransactionSplit{
			{Amount: 50.00, CategoryID: "cat-1"},
			{Amount: 53.27, CategoryID: "cat-2"},
		},
	}

	monarch := &mockMonarch{}

	matcherCfg := matcher.Config{AmountTolerance: 0.50, DateTolerance: 5}
	handler := NewAmazonHandler(
		matcher.NewMatcher(matcherCfg),
		consolidator,
		splitter,
		monarch,
		nil, // logger
	)

	usedTxnIDs := make(map[string]bool)
	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		monarchTxns,
		usedTxnIDs,
		nil, // catCategories
		nil, // monarchCategories
		false, // dryRun
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.False(t, result.Skipped)
	assert.NotNil(t, result.Allocations)
	assert.Len(t, result.Splits, 2)
	assert.True(t, monarch.updateSplitsCalled)

	// Verify allocation multiplier is ~0.9628 (103.27 / 107.26)
	assert.InDelta(t, 0.9628, result.Allocations.Multiplier, 0.001)
}

func TestAmazonHandler_ProcessOrder_InvalidCharges(t *testing.T) {
	// Order with missing bank charge
	order := &mockAmazonOrder{
		id:            "test-missing-charge",
		date:          time.Now(),
		total:         103.27,
		items:         []providers.OrderItem{&mockItem{name: "Item", price: 107.26}},
		bankCharges:   []float64{52.55}, // Missing $50.72
		nonBankAmount: 0,
	}

	handler := NewAmazonHandler(nil, nil, nil, nil, nil)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		nil,
		make(map[string]bool),
		nil, nil,
		false,
	)

	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Contains(t, result.SkipReason, "less than expected")
}

func TestAmazonHandler_ProcessOrder_MissingTransactions(t *testing.T) {
	order := &mockAmazonOrder{
		id:            "test-order",
		date:          time.Now(),
		total:         100.00,
		items:         []providers.OrderItem{&mockItem{name: "Item", price: 100.00}},
		bankCharges:   []float64{50.00, 50.00}, // Multi-delivery
		nonBankAmount: 0,
	}

	// Only one transaction available
	monarchTxns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: toMonarchDate(time.Now())},
	}

	matcherCfg := matcher.Config{AmountTolerance: 0.50, DateTolerance: 5}
	handler := NewAmazonHandler(
		matcher.NewMatcher(matcherCfg),
		nil, nil, nil, nil,
	)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		monarchTxns,
		make(map[string]bool),
		nil, nil,
		false,
	)

	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Contains(t, result.SkipReason, "could not find all transactions")
}

func TestAmazonHandler_ProcessOrder_DryRun(t *testing.T) {
	order := &mockAmazonOrder{
		id:            "test-dry-run",
		date:          time.Now(),
		total:         100.00,
		items:         []providers.OrderItem{&mockItem{name: "Item", price: 100.00}},
		bankCharges:   []float64{100.00},
		nonBankAmount: 0,
	}

	monarchTxns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -100.00, Date: toMonarchDate(time.Now())},
	}

	splitter := &mockSplitter{
		splits: []*monarch.TransactionSplit{
			{Amount: 100.00, CategoryID: "cat-1"},
		},
	}

	monarchClient := &mockMonarch{}

	matcherCfg := matcher.Config{AmountTolerance: 0.50, DateTolerance: 5}
	handler := NewAmazonHandler(
		matcher.NewMatcher(matcherCfg),
		nil,
		splitter,
		monarchClient,
		nil,
	)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		monarchTxns,
		make(map[string]bool),
		nil, nil,
		true, // dryRun = true
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.False(t, monarchClient.updateSplitsCalled, "Should not call Monarch API in dry-run")
}

func TestAmazonHandler_ProcessOrder_SingleCategory(t *testing.T) {
	order := &mockAmazonOrder{
		id:            "test-single-category",
		date:          time.Now(),
		total:         50.00,
		items:         []providers.OrderItem{&mockItem{name: "Toy", price: 50.00}},
		bankCharges:   []float64{50.00},
		nonBankAmount: 0,
	}

	monarchTxns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: toMonarchDate(time.Now())},
	}

	splitter := &mockSplitter{
		splits:     nil, // nil means single category
		categoryID: "toys-category",
		notes:      "Toy",
	}

	monarchClient := &mockMonarch{}

	matcherCfg := matcher.Config{AmountTolerance: 0.50, DateTolerance: 5}
	handler := NewAmazonHandler(
		matcher.NewMatcher(matcherCfg),
		nil,
		splitter,
		monarchClient,
		nil,
	)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		monarchTxns,
		make(map[string]bool),
		nil, nil,
		false,
	)

	require.NoError(t, err)
	assert.True(t, result.Processed)
	assert.True(t, monarchClient.updateCalled, "Should update transaction category")
	assert.False(t, monarchClient.updateSplitsCalled, "Should not create splits for single category")
}

func TestAmazonHandler_ProcessOrder_WithGiftCard(t *testing.T) {
	// Order total $150, paid with $50 gift card + $100 bank charge
	order := &mockAmazonOrder{
		id:            "test-gift-card",
		date:          time.Now(),
		total:         150.00,
		items:         []providers.OrderItem{&mockItem{name: "Item", price: 150.00}},
		bankCharges:   []float64{100.00},
		nonBankAmount: 50.00, // Gift card
	}

	monarchTxns := []*monarch.Transaction{
		{ID: "txn-1", Amount: -100.00, Date: toMonarchDate(time.Now())},
	}

	splitter := &mockSplitter{
		splits: []*monarch.TransactionSplit{
			{Amount: 100.00, CategoryID: "cat-1"},
		},
	}

	monarchClient := &mockMonarch{}

	matcherCfg := matcher.Config{AmountTolerance: 0.50, DateTolerance: 5}
	handler := NewAmazonHandler(
		matcher.NewMatcher(matcherCfg),
		nil,
		splitter,
		monarchClient,
		nil,
	)

	result, err := handler.ProcessOrder(
		context.Background(),
		order,
		monarchTxns,
		make(map[string]bool),
		nil, nil,
		false,
	)

	require.NoError(t, err)
	if result.Skipped {
		t.Fatalf("Order was skipped: %s", result.SkipReason)
	}
	assert.True(t, result.Processed)
	require.NotNil(t, result.Allocations, "Allocations should not be nil")
	// Allocation should be based on bank charges ($100), not order total ($150)
	assert.InDelta(t, 100.00, result.Allocations.TotalAllocated, 0.01)
}

func TestAllocatedItem(t *testing.T) {
	item := &allocatedItem{
		name:  "Test Item",
		price: 42.50,
	}

	assert.Equal(t, "Test Item", item.GetName())
	assert.Equal(t, 42.50, item.GetPrice())
	assert.Equal(t, 1.0, item.GetQuantity())
	assert.Equal(t, 42.50, item.GetUnitPrice())
	assert.Empty(t, item.GetDescription())
	assert.Empty(t, item.GetSKU())
	assert.Empty(t, item.GetCategory())
}
