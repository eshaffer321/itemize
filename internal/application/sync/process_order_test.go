package sync

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/categorizer"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/matcher"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/domain/splitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for processOrder's generic path
// Provider-specific behavior (Walmart, Amazon) is tested in handlers/ package

// =============================================================================
// Test Helpers and Mocks
// =============================================================================

// toMonarchDate converts time.Time to monarch.Date
func toMonarchDate(t time.Time) monarch.Date {
	return monarch.Date{Time: t}
}

// mockSimpleOrder implements providers.Order for basic orders (no multi-delivery)
// This simulates providers like Costco that don't have special handling
type mockSimpleOrder struct {
	id           string
	date         time.Time
	total        float64
	subtotal     float64
	tax          float64
	items        []providers.OrderItem
	providerName string
}

func (m *mockSimpleOrder) GetID() string                   { return m.id }
func (m *mockSimpleOrder) GetDate() time.Time              { return m.date }
func (m *mockSimpleOrder) GetTotal() float64               { return m.total }
func (m *mockSimpleOrder) GetSubtotal() float64            { return m.subtotal }
func (m *mockSimpleOrder) GetTax() float64                 { return m.tax }
func (m *mockSimpleOrder) GetTip() float64                 { return 0 }
func (m *mockSimpleOrder) GetFees() float64                { return 0 }
func (m *mockSimpleOrder) GetItems() []providers.OrderItem { return m.items }
func (m *mockSimpleOrder) GetProviderName() string         { return m.providerName }
func (m *mockSimpleOrder) GetRawData() interface{}         { return nil }

// mockOrderItem implements providers.OrderItem
type mockOrderItem struct {
	name     string
	price    float64
	quantity float64
}

func (m *mockOrderItem) GetName() string        { return m.name }
func (m *mockOrderItem) GetPrice() float64      { return m.price }
func (m *mockOrderItem) GetQuantity() float64   { return m.quantity }
func (m *mockOrderItem) GetUnitPrice() float64  { return m.price / m.quantity }
func (m *mockOrderItem) GetDescription() string { return "" }
func (m *mockOrderItem) GetSKU() string         { return "" }
func (m *mockOrderItem) GetCategory() string    { return "" }

// mockCategorizer implements splitter.Categorizer for testing
type mockCategorizer struct {
	categoryID   string
	categoryName string
}

func (m *mockCategorizer) CategorizeItems(ctx context.Context, items []categorizer.Item, categories []categorizer.Category) (*categorizer.CategorizationResult, error) {
	result := &categorizer.CategorizationResult{
		Categorizations: make([]categorizer.ItemCategorization, len(items)),
	}

	catID := m.categoryID
	catName := m.categoryName
	if catID == "" {
		if len(categories) > 0 {
			catID = categories[0].ID
			catName = categories[0].Name
		} else {
			catID = "default-category"
			catName = "Default"
		}
	}

	for i, item := range items {
		result.Categorizations[i] = categorizer.ItemCategorization{
			ItemName:     item.Name,
			CategoryID:   catID,
			CategoryName: catName,
			Confidence:   1.0,
		}
	}
	return result, nil
}

// createTestOrchestrator creates an orchestrator with mocked dependencies
func createTestOrchestrator(t *testing.T) *Orchestrator {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	matcherCfg := matcher.Config{
		AmountTolerance: 0.50,
		DateTolerance:   5,
	}

	// Create a real splitter with a mock categorizer
	mockCat := &mockCategorizer{categoryID: "groceries", categoryName: "Groceries"}
	spl := splitter.NewSplitter(mockCat)

	return &Orchestrator{
		matcher:  matcher.NewMatcher(matcherCfg),
		splitter: spl,
		logger:   logger,
	}
}

// =============================================================================
// Test: Generic Order Matching (Costco-like providers)
// =============================================================================

func TestProcessOrder_GenericMatching_Success(t *testing.T) {
	// A simple order with matching transaction should match
	orch := createTestOrchestrator(t)

	orderDate := time.Now()
	order := &mockSimpleOrder{
		id:           "ORDER-001",
		date:         orderDate,
		total:        50.00,
		subtotal:     45.00,
		tax:          5.00,
		providerName: "Costco",
		items: []providers.OrderItem{
			&mockOrderItem{name: "Milk", price: 5.00, quantity: 1},
			&mockOrderItem{name: "Bread", price: 40.00, quantity: 1},
		},
	}

	transactions := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: toMonarchDate(orderDate)},
	}

	usedTxnIDs := make(map[string]bool)
	catCategories := []categorizer.Category{{ID: "cat-1", Name: "Groceries"}}
	monarchCategories := []*monarch.TransactionCategory{{ID: "cat-1", Name: "Groceries"}}
	opts := Options{DryRun: true}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		transactions,
		usedTxnIDs,
		catCategories,
		monarchCategories,
		opts,
	)

	assert.NoError(t, err)
	assert.True(t, processed, "Order should be marked as processed")
	assert.False(t, skipped)
	assert.True(t, usedTxnIDs["txn-1"], "Transaction should be marked as used after matching")
}

func TestProcessOrder_GenericMatching_NoMatch(t *testing.T) {
	// Order with no matching transaction returns error
	orch := createTestOrchestrator(t)

	orderDate := time.Now()
	order := &mockSimpleOrder{
		id:           "ORDER-002",
		date:         orderDate,
		total:        50.00,
		subtotal:     45.00,
		tax:          5.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&mockOrderItem{name: "Item", price: 45.00, quantity: 1}},
	}

	// Transaction with wrong amount
	transactions := []*monarch.Transaction{
		{ID: "txn-1", Amount: -100.00, Date: toMonarchDate(orderDate)},
	}

	usedTxnIDs := make(map[string]bool)
	opts := Options{DryRun: true}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		transactions,
		usedTxnIDs,
		nil, nil,
		opts,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching transaction")
	assert.False(t, processed)
	assert.False(t, skipped)
}

func TestProcessOrder_GenericMatching_DateTolerance(t *testing.T) {
	// Orders within date tolerance should match
	orch := createTestOrchestrator(t)

	orderDate := time.Now()
	// Transaction is 3 days after order (within 5 day tolerance)
	txnDate := orderDate.AddDate(0, 0, 3)

	order := &mockSimpleOrder{
		id:           "ORDER-003",
		date:         orderDate,
		total:        75.00,
		subtotal:     70.00,
		tax:          5.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&mockOrderItem{name: "Item", price: 70.00, quantity: 1}},
	}

	transactions := []*monarch.Transaction{
		{ID: "txn-1", Amount: -75.00, Date: toMonarchDate(txnDate)},
	}

	usedTxnIDs := make(map[string]bool)
	opts := Options{DryRun: true}

	_, _, err := orch.processOrder(
		context.Background(),
		order,
		transactions,
		usedTxnIDs,
		nil, nil,
		opts,
	)

	assert.True(t, usedTxnIDs["txn-1"], "Transaction within date tolerance should match")
	assert.NoError(t, err)
}

func TestProcessOrder_GenericMatching_OutsideDateTolerance(t *testing.T) {
	// Orders outside date tolerance should NOT match
	orch := createTestOrchestrator(t)

	orderDate := time.Now()
	// Transaction is 10 days after order (outside 5 day tolerance)
	txnDate := orderDate.AddDate(0, 0, 10)

	order := &mockSimpleOrder{
		id:           "ORDER-004",
		date:         orderDate,
		total:        75.00,
		subtotal:     70.00,
		tax:          5.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&mockOrderItem{name: "Item", price: 70.00, quantity: 1}},
	}

	transactions := []*monarch.Transaction{
		{ID: "txn-1", Amount: -75.00, Date: toMonarchDate(txnDate)},
	}

	usedTxnIDs := make(map[string]bool)
	opts := Options{DryRun: true}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		transactions,
		usedTxnIDs,
		nil, nil,
		opts,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching transaction")
	assert.False(t, processed)
	assert.False(t, skipped)
	assert.False(t, usedTxnIDs["txn-1"], "Transaction outside date tolerance should not match")
}

func TestProcessOrder_GenericMatching_AmountTolerance(t *testing.T) {
	// Orders within amount tolerance should match
	orch := createTestOrchestrator(t)

	orderDate := time.Now()
	order := &mockSimpleOrder{
		id:           "ORDER-005",
		date:         orderDate,
		total:        50.00,
		subtotal:     45.00,
		tax:          5.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&mockOrderItem{name: "Item", price: 45.00, quantity: 1}},
	}

	// Transaction is $0.30 off (within $0.50 tolerance)
	transactions := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.30, Date: toMonarchDate(orderDate)},
	}

	usedTxnIDs := make(map[string]bool)
	opts := Options{DryRun: true}

	_, _, _ = orch.processOrder(
		context.Background(),
		order,
		transactions,
		usedTxnIDs,
		nil, nil,
		opts,
	)

	assert.True(t, usedTxnIDs["txn-1"], "Transaction within amount tolerance should match")
}

func TestProcessOrder_GenericMatching_TransactionAlreadyUsed(t *testing.T) {
	// Already-used transactions should not be matched again
	orch := createTestOrchestrator(t)

	orderDate := time.Now()
	order := &mockSimpleOrder{
		id:           "ORDER-006",
		date:         orderDate,
		total:        50.00,
		subtotal:     45.00,
		tax:          5.00,
		providerName: "Costco",
		items:        []providers.OrderItem{&mockOrderItem{name: "Item", price: 45.00, quantity: 1}},
	}

	transactions := []*monarch.Transaction{
		{ID: "txn-1", Amount: -50.00, Date: toMonarchDate(orderDate)},
	}

	// Mark transaction as already used
	usedTxnIDs := map[string]bool{"txn-1": true}
	opts := Options{DryRun: true}

	processed, skipped, err := orch.processOrder(
		context.Background(),
		order,
		transactions,
		usedTxnIDs,
		nil, nil,
		opts,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching transaction")
	assert.False(t, processed)
	assert.False(t, skipped)
}
