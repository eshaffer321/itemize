package sync

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOrder is a minimal Order implementation for testing
type mockOrder struct {
	id    string
	date  time.Time
	total float64
}

func (m *mockOrder) GetID() string                       { return m.id }
func (m *mockOrder) GetDate() time.Time                  { return m.date }
func (m *mockOrder) GetTotal() float64                   { return m.total }
func (m *mockOrder) GetSubtotal() float64                { return 0 }
func (m *mockOrder) GetTax() float64                     { return 0 }
func (m *mockOrder) GetTip() float64                     { return 0 }
func (m *mockOrder) GetFees() float64                    { return 0 }
func (m *mockOrder) GetItems() []providers.OrderItem     { return nil }
func (m *mockOrder) GetProviderName() string             { return "test" }
func (m *mockOrder) GetRawData() interface{}             { return nil }

// mockMonarchClient simulates the Monarch client for testing
type mockMonarchClient struct {
	updateCalled    int
	deleteCalled    int
	updateError     error
	deleteError     error
	deleteFailAfter int // Fail after N deletes (for partial failure testing)
}

// mockTransactionQueryBuilder implements monarch.TransactionQueryBuilder
type mockTransactionQueryBuilder struct{}

func (m *mockTransactionQueryBuilder) Between(start, end time.Time) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) WithAccounts(accountIDs ...string) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) WithCategories(categoryIDs ...string) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) WithTags(tagIDs ...string) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) WithMinAmount(amount float64) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) WithMaxAmount(amount float64) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) WithMerchant(merchant string) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) Search(query string) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) Limit(limit int) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) Offset(offset int) monarch.TransactionQueryBuilder {
	return m
}
func (m *mockTransactionQueryBuilder) Execute(ctx context.Context) (*monarch.TransactionList, error) {
	return nil, nil
}
func (m *mockTransactionQueryBuilder) Stream(ctx context.Context) (<-chan *monarch.Transaction, <-chan error) {
	return nil, nil
}

// Query returns a mock query builder
func (m *mockMonarchClient) Query() monarch.TransactionQueryBuilder {
	return &mockTransactionQueryBuilder{}
}

// Get retrieves a transaction (returns nil for mock)
func (m *mockMonarchClient) Get(ctx context.Context, id string) (*monarch.TransactionDetails, error) {
	return nil, nil
}

// Create is not used in consolidator
func (m *mockMonarchClient) Create(ctx context.Context, params *monarch.CreateTransactionParams) (*monarch.Transaction, error) {
	return nil, nil
}

// Update updates a transaction
func (m *mockMonarchClient) Update(ctx context.Context, id string, params *monarch.UpdateTransactionParams) (*monarch.Transaction, error) {
	m.updateCalled++
	if m.updateError != nil {
		return nil, m.updateError
	}

	// Return updated transaction
	return &monarch.Transaction{
		ID:     id,
		Amount: *params.Amount,
		Notes:  *params.Notes,
	}, nil
}

// Delete deletes a transaction
func (m *mockMonarchClient) Delete(ctx context.Context, id string) error {
	m.deleteCalled++
	if m.deleteFailAfter > 0 && m.deleteCalled > m.deleteFailAfter {
		return fmt.Errorf("mock delete error after %d deletions", m.deleteFailAfter)
	}
	if m.deleteError != nil {
		return m.deleteError
	}
	return nil
}

// GetSummary is not used in consolidator
func (m *mockMonarchClient) GetSummary(ctx context.Context) (*monarch.TransactionSummary, error) {
	return nil, nil
}

// GetSplits is not used in consolidator
func (m *mockMonarchClient) GetSplits(ctx context.Context, id string) ([]*monarch.TransactionSplit, error) {
	return nil, nil
}

// UpdateSplits updates transaction splits
func (m *mockMonarchClient) UpdateSplits(ctx context.Context, id string, splits []*monarch.TransactionSplit) error {
	return nil
}

// mockTransactionCategoryService implements monarch.TransactionCategoryService
type mockTransactionCategoryService struct{}

func (m *mockTransactionCategoryService) List(ctx context.Context) ([]*monarch.TransactionCategory, error) {
	return nil, nil
}

func (m *mockTransactionCategoryService) Create(ctx context.Context, params *monarch.CreateCategoryParams) (*monarch.TransactionCategory, error) {
	return nil, nil
}

func (m *mockTransactionCategoryService) Delete(ctx context.Context, categoryID string) error {
	return nil
}

func (m *mockTransactionCategoryService) DeleteMultiple(ctx context.Context, categoryIDs ...string) error {
	return nil
}

func (m *mockTransactionCategoryService) GetGroups(ctx context.Context) ([]*monarch.CategoryGroup, error) {
	return nil, nil
}

// Categories returns mock category service
func (m *mockMonarchClient) Categories() monarch.TransactionCategoryService {
	return &mockTransactionCategoryService{}
}

// TestConsolidator_ConsolidateTransactions tests transaction consolidation
func TestConsolidator_ConsolidateTransactions(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	t.Run("consolidates two transactions successfully", func(t *testing.T) {
		mockClient := &mockMonarchClient{}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{
			id:    "ORDER123",
			date:  time.Now(),
			total: 126.98,
		}

		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.67},
			{ID: "txn2", Amount: -8.31},
		}

		result, err := consolidator.ConsolidateTransactions(ctx, transactions, order, false)
		require.NoError(t, err)

		// Verify result
		assert.NotNil(t, result.ConsolidatedTransaction)
		assert.Equal(t, "txn1", result.ConsolidatedTransaction.ID)
		assert.Equal(t, -126.98, result.ConsolidatedTransaction.Amount)
		assert.Contains(t, result.ConsolidatedTransaction.Notes, "Multi-delivery order")
		assert.Contains(t, result.ConsolidatedTransaction.Notes, "2 charges")
		assert.Nil(t, result.FailedDeletions)

		// Verify API calls
		assert.Equal(t, 1, mockClient.updateCalled, "Should update primary transaction")
		assert.Equal(t, 1, mockClient.deleteCalled, "Should delete extra transaction")
	})

	t.Run("consolidates three transactions", func(t *testing.T) {
		mockClient := &mockMonarchClient{}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{
			id:    "ORDER456",
			total: 100.00,
		}

		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -50.00},
			{ID: "txn2", Amount: -30.00},
			{ID: "txn3", Amount: -20.00},
		}

		result, err := consolidator.ConsolidateTransactions(ctx, transactions, order, false)
		require.NoError(t, err)

		assert.Equal(t, "txn1", result.ConsolidatedTransaction.ID)
		assert.Equal(t, -100.00, result.ConsolidatedTransaction.Amount)
		assert.Nil(t, result.FailedDeletions)

		assert.Equal(t, 1, mockClient.updateCalled)
		assert.Equal(t, 2, mockClient.deleteCalled, "Should delete 2 extra transactions")
	})

	t.Run("dry-run mode does not call API", func(t *testing.T) {
		mockClient := &mockMonarchClient{}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{
			id:    "ORDER789",
			total: 126.98,
		}

		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.67},
			{ID: "txn2", Amount: -8.31},
		}

		result, err := consolidator.ConsolidateTransactions(ctx, transactions, order, true) // DRY RUN
		require.NoError(t, err)

		// Should return updated transaction (dry-run copy)
		assert.NotNil(t, result.ConsolidatedTransaction)
		assert.Equal(t, -126.98, result.ConsolidatedTransaction.Amount)

		// Should NOT call API
		assert.Equal(t, 0, mockClient.updateCalled, "Should not update in dry-run")
		assert.Equal(t, 0, mockClient.deleteCalled, "Should not delete in dry-run")
	})

	t.Run("handles positive transaction amounts (returns)", func(t *testing.T) {
		mockClient := &mockMonarchClient{}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{
			id:    "ORDER999",
			total: -126.98, // Negative order total (return)
		}

		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: 118.67}, // Positive (return)
			{ID: "txn2", Amount: 8.31},   // Positive (return)
		}

		result, err := consolidator.ConsolidateTransactions(ctx, transactions, order, false)
		require.NoError(t, err)

		// Should match sign of primary transaction (positive)
		assert.Equal(t, 126.98, result.ConsolidatedTransaction.Amount)
	})

	t.Run("single transaction - no consolidation needed", func(t *testing.T) {
		mockClient := &mockMonarchClient{}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{
			id:    "ORDER111",
			total: 100.00,
		}

		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -100.00},
		}

		result, err := consolidator.ConsolidateTransactions(ctx, transactions, order, false)
		require.NoError(t, err)

		// Should return original transaction without modifications
		assert.Equal(t, "txn1", result.ConsolidatedTransaction.ID)
		assert.Nil(t, result.FailedDeletions)

		// Should not call API
		assert.Equal(t, 0, mockClient.updateCalled)
		assert.Equal(t, 0, mockClient.deleteCalled)
	})

	t.Run("error - no transactions provided", func(t *testing.T) {
		mockClient := &mockMonarchClient{}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{id: "ORDER", total: 100}
		transactions := []*monarch.Transaction{}

		_, err := consolidator.ConsolidateTransactions(ctx, transactions, order, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no transactions to consolidate")
	})

	t.Run("error - update fails", func(t *testing.T) {
		mockClient := &mockMonarchClient{
			updateError: fmt.Errorf("API error"),
		}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{total: 126.98}
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.67},
			{ID: "txn2", Amount: -8.31},
		}

		_, err := consolidator.ConsolidateTransactions(ctx, transactions, order, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update primary transaction")
	})

	t.Run("partial failure - some deletions fail", func(t *testing.T) {
		mockClient := &mockMonarchClient{
			deleteFailAfter: 1, // First delete succeeds, second fails
		}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{total: 100.00}
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -50.00},
			{ID: "txn2", Amount: -30.00},
			{ID: "txn3", Amount: -20.00},
		}

		result, err := consolidator.ConsolidateTransactions(ctx, transactions, order, false)
		require.NoError(t, err, "Should not fail on partial deletion failure")

		// Update should succeed
		assert.NotNil(t, result.ConsolidatedTransaction)
		assert.Equal(t, "txn1", result.ConsolidatedTransaction.ID)

		// Should report failed deletion
		assert.NotNil(t, result.FailedDeletions)
		assert.Len(t, result.FailedDeletions, 1, "Should report 1 failed deletion")
		assert.Equal(t, "txn3", result.FailedDeletions[0])

		// Should have attempted all deletes
		assert.Equal(t, 1, mockClient.updateCalled)
		assert.Equal(t, 2, mockClient.deleteCalled)
	})

	t.Run("skips transactions with splits", func(t *testing.T) {
		mockClient := &mockMonarchClient{}
		client := &monarch.Client{
			Transactions: mockClient,
		}
		consolidator := NewConsolidator(client, logger)

		order := &mockOrder{total: 126.98}
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.67},
			{ID: "txn2", Amount: -8.31, HasSplits: true}, // Has splits - should not delete
		}

		result, err := consolidator.ConsolidateTransactions(ctx, transactions, order, false)
		require.NoError(t, err)

		// Should report failed deletion
		assert.Len(t, result.FailedDeletions, 1)
		assert.Equal(t, "txn2", result.FailedDeletions[0])

		// Should only call update, not delete
		assert.Equal(t, 1, mockClient.updateCalled)
		assert.Equal(t, 0, mockClient.deleteCalled, "Should not delete transaction with splits")
	})
}

// TestConsolidator_buildConsolidationNote tests note generation
func TestConsolidator_buildConsolidationNote(t *testing.T) {
	consolidator := NewConsolidator(nil, nil)

	t.Run("two charges", func(t *testing.T) {
		transactions := []*monarch.Transaction{
			{Amount: -118.67},
			{Amount: -8.31},
		}

		note := consolidator.buildConsolidationNote(transactions)
		assert.Equal(t, "Multi-delivery order (2 charges: $118.67, $8.31)", note)
	})

	t.Run("three charges", func(t *testing.T) {
		transactions := []*monarch.Transaction{
			{Amount: -50.00},
			{Amount: -30.00},
			{Amount: -20.00},
		}

		note := consolidator.buildConsolidationNote(transactions)
		assert.Equal(t, "Multi-delivery order (3 charges: $50.00, $30.00, $20.00)", note)
	})

	t.Run("positive amounts (returns)", func(t *testing.T) {
		transactions := []*monarch.Transaction{
			{Amount: 118.67},
			{Amount: 8.31},
		}

		note := consolidator.buildConsolidationNote(transactions)
		assert.Equal(t, "Multi-delivery order (2 charges: $118.67, $8.31)", note)
	})

	t.Run("empty transactions", func(t *testing.T) {
		transactions := []*monarch.Transaction{}

		note := consolidator.buildConsolidationNote(transactions)
		assert.Equal(t, "", note)
	})
}
