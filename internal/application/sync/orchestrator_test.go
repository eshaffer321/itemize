package sync

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	amazonprovider "github.com/eshaffer321/itemize/internal/adapters/providers/amazon"
	"github.com/eshaffer321/itemize/internal/infrastructure/storage"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProvider implements providers.OrderProvider for testing
type MockProvider struct {
	mock.Mock
	returnRecords []amazonprovider.ReturnRecord
	returnErr     error
}

func (m *MockProvider) FetchReturns(context.Context) ([]amazonprovider.ReturnRecord, error) {
	return m.returnRecords, m.returnErr
}

func (m *MockProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockProvider) DisplayName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockProvider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]providers.Order), args.Error(1)
}

func (m *MockProvider) GetOrderDetails(ctx context.Context, orderID string) (providers.Order, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(providers.Order), args.Error(1)
}

func (m *MockProvider) SupportsDeliveryTips() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockProvider) SupportsRefunds() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockProvider) SupportsBulkFetch() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockProvider) GetRateLimit() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockOrder implements providers.Order for testing
type MockOrder struct {
	mock.Mock
}

func (m *MockOrder) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockOrder) GetDate() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

func (m *MockOrder) GetTotal() float64 {
	args := m.Called()
	return args.Get(0).(float64)
}

func (m *MockOrder) GetSubtotal() float64 {
	args := m.Called()
	return args.Get(0).(float64)
}

func (m *MockOrder) GetTax() float64 {
	args := m.Called()
	return args.Get(0).(float64)
}

func (m *MockOrder) GetTip() float64 {
	args := m.Called()
	return args.Get(0).(float64)
}

func (m *MockOrder) GetFees() float64 {
	args := m.Called()
	return args.Get(0).(float64)
}

func (m *MockOrder) GetItems() []providers.OrderItem {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]providers.OrderItem)
}

func (m *MockOrder) GetProviderName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockOrder) GetRawData() interface{} {
	args := m.Called()
	return args.Get(0)
}

// TestOrchestrator_Run_EmptyOrders tests that orchestrator handles empty orders
func TestOrchestrator_Run_EmptyOrders(t *testing.T) {
	// Arrange
	mockProvider := new(MockProvider)
	mockProvider.On("DisplayName").Return("TestProvider")
	mockProvider.On("FetchOrders", mock.Anything, mock.Anything).Return([]providers.Order{}, nil)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// We'll need a real storage and clients for now (will mock later)
	// For now, just test the basic structure
	orchestrator := NewOrchestrator(mockProvider, nil, nil, logger)

	opts := Options{
		DryRun:       true,
		LookbackDays: 7,
		MaxOrders:    0,
		Force:        false,
		Verbose:      false,
	}

	// Act
	result, err := orchestrator.Run(context.Background(), opts)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.ProcessedCount)
	assert.Equal(t, 0, result.ErrorCount)
	mockProvider.AssertExpectations(t)
}

// TestOrchestrator_Run_FetchOrdersError tests error handling when fetching orders fails
func TestOrchestrator_Run_FetchOrdersError(t *testing.T) {
	// Arrange
	mockProvider := new(MockProvider)
	mockProvider.On("DisplayName").Return("TestProvider")
	expectedErr := errors.New("failed to fetch orders")
	mockProvider.On("FetchOrders", mock.Anything, mock.Anything).Return(nil, expectedErr)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	orchestrator := NewOrchestrator(mockProvider, nil, nil, logger)

	opts := Options{
		DryRun:       true,
		LookbackDays: 7,
	}

	// Act
	result, err := orchestrator.Run(context.Background(), opts)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch orders")
	assert.Nil(t, result)
	mockProvider.AssertExpectations(t)
}

// TestOrchestrator_Run_DryRun tests that dry run mode doesn't make changes
func TestOrchestrator_Run_DryRun(t *testing.T) {
	// Arrange
	mockProvider := new(MockProvider)
	mockProvider.On("DisplayName").Return("TestProvider")
	mockOrder := new(MockOrder)

	mockOrder.On("GetID").Return("order123")
	mockOrder.On("GetDate").Return(time.Now())
	mockOrder.On("GetTotal").Return(100.0)
	mockOrder.On("GetSubtotal").Return(90.0)
	mockOrder.On("GetTax").Return(10.0)
	mockOrder.On("GetTip").Return(0.0)
	mockOrder.On("GetFees").Return(0.0)
	mockOrder.On("GetItems").Return([]providers.OrderItem{})
	mockOrder.On("GetProviderName").Return("TestProvider")
	mockOrder.On("GetRawData").Return(nil)

	orders := []providers.Order{mockOrder}
	mockProvider.On("FetchOrders", mock.Anything, mock.Anything).Return(orders, nil)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	orchestrator := NewOrchestrator(mockProvider, nil, nil, logger)

	opts := Options{
		DryRun:       true,
		LookbackDays: 7,
		MaxOrders:    0,
		Force:        false,
	}

	// Act
	result, err := orchestrator.Run(context.Background(), opts)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// With current stub implementation, we expect 0 processed
	// This test will be updated once we implement the full logic
	mockProvider.AssertExpectations(t)
}

// TestOptions_Validation tests that options are validated
func TestOptions_Validation(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name: "valid options",
			opts: Options{
				DryRun:       false,
				LookbackDays: 7,
				MaxOrders:    10,
			},
			wantErr: false,
		},
		{
			name: "valid dry run",
			opts: Options{
				DryRun:       true,
				LookbackDays: 30,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For now, just verify the Options struct can be created
			assert.NotNil(t, tt.opts)
		})
	}
}

// TestResult_Initialization tests that Result is properly initialized
func TestResult_Initialization(t *testing.T) {
	result := &Result{
		ProcessedCount: 5,
		SkippedCount:   2,
		ErrorCount:     1,
		Errors:         []error{errors.New("test error")},
	}

	assert.Equal(t, 5, result.ProcessedCount)
	assert.Equal(t, 2, result.SkippedCount)
	assert.Equal(t, 1, result.ErrorCount)
	assert.Len(t, result.Errors, 1)
}

// TestNewOrchestrator tests orchestrator creation
func TestNewOrchestrator(t *testing.T) {
	mockProvider := new(MockProvider)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	orchestrator := NewOrchestrator(mockProvider, nil, nil, logger)

	assert.NotNil(t, orchestrator)
	assert.Equal(t, mockProvider, orchestrator.provider)
	assert.Nil(t, orchestrator.clients)
	assert.Nil(t, orchestrator.storage)
	assert.Equal(t, logger, orchestrator.logger)
}

// TestOrchestrator_MatchesTransactions will test transaction matching
// This will be implemented once we have the full orchestrator logic
func TestOrchestrator_MatchesTransactions(t *testing.T) {
	t.Skip("Will be implemented after orchestrator logic is complete")

	// TODO: Test that:
	// - Orders are matched to correct transactions
	// - Date tolerance works correctly
	// - Amount tolerance works correctly
	// - Already matched transactions are not reused
}

// TestOrchestrator_CreatesCorrectSplits will test split creation
// This will be implemented once we have the full orchestrator logic
func TestOrchestrator_CreatesCorrectSplits(t *testing.T) {
	t.Skip("Will be implemented after orchestrator logic is complete")

	// TODO: Test that:
	// - Items are categorized correctly
	// - Splits sum to transaction total
	// - Tax is distributed proportionally
	// - Notes contain item details
}

// TestOrchestrator_HandlesAlreadyProcessed will test skip logic
func TestOrchestrator_HandlesAlreadyProcessed(t *testing.T) {
	t.Skip("Will be implemented after orchestrator logic is complete")

	// TODO: Test that:
	// - Already processed orders are skipped (unless force=true)
	// - Force flag causes reprocessing
	// - Skipped count is incremented correctly
}

// TestOrchestrator_MaxOrders will test max orders limit
func TestOrchestrator_MaxOrders(t *testing.T) {
	t.Skip("Will be implemented after orchestrator logic is complete")

	// TODO: Test that:
	// - Only MaxOrders orders are processed when set
	// - All orders are processed when MaxOrders=0
}

// Integration test placeholder - will need real dependencies
func TestOrchestrator_Integration(t *testing.T) {
	t.Skip("Integration test - requires real clients and storage")

	// TODO: Full end-to-end test with:
	// - Real storage (test database)
	// - Mock HTTP clients for APIs
	// - Full flow from fetch to split
}

// MockMonarchClient is a mock for testing
type MockMonarchClient struct {
	mock.Mock
}

// MockTransactionsService is a mock for testing
type MockTransactionsService struct {
	mock.Mock
}

// MockTransactionQuery is a mock for testing
type MockTransactionQuery struct {
	mock.Mock
}

func (m *MockTransactionQuery) Between(start, end time.Time) *MockTransactionQuery {
	m.Called(start, end)
	return m
}

func (m *MockTransactionQuery) Limit(limit int) *MockTransactionQuery {
	m.Called(limit)
	return m
}

func (m *MockTransactionQuery) Execute(ctx context.Context) (*monarch.TransactionList, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*monarch.TransactionList), args.Error(1)
}

// TestOrchestrator_fetchOrders tests the order fetching functionality
func TestOrchestrator_fetchOrders(t *testing.T) {
	tests := []struct {
		name            string
		opts            Options
		mockOrders      []providers.Order
		mockErr         error
		expectErr       bool
		expectOrders    int
		expectStartDays int
	}{
		{
			name: "successful fetch with default lookback",
			opts: Options{
				LookbackDays: 7,
				MaxOrders:    10,
			},
			mockOrders: []providers.Order{
				&MockOrder{},
			},
			mockErr:         nil,
			expectErr:       false,
			expectOrders:    1,
			expectStartDays: 7,
		},
		{
			name: "successful fetch with 30 day lookback",
			opts: Options{
				LookbackDays: 30,
				MaxOrders:    0,
			},
			mockOrders: []providers.Order{
				&MockOrder{},
				&MockOrder{},
			},
			mockErr:         nil,
			expectErr:       false,
			expectOrders:    2,
			expectStartDays: 30,
		},
		{
			name: "fetch returns error",
			opts: Options{
				LookbackDays: 7,
			},
			mockOrders:   nil,
			mockErr:      errors.New("provider error"),
			expectErr:    true,
			expectOrders: 0,
		},
		{
			name: "fetch returns empty orders",
			opts: Options{
				LookbackDays: 7,
			},
			mockOrders:   []providers.Order{},
			mockErr:      nil,
			expectErr:    false,
			expectOrders: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockProvider := new(MockProvider)
			mockProvider.On("FetchOrders", mock.Anything, mock.Anything).Return(tt.mockOrders, tt.mockErr)

			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			orchestrator := NewOrchestrator(mockProvider, nil, nil, logger)

			// Act
			orders, err := orchestrator.fetchOrders(context.Background(), tt.opts)

			// Assert
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, orders)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, orders)
				assert.Len(t, orders, tt.expectOrders)
			}
			mockProvider.AssertExpectations(t)
		})
	}
}

func TestOrchestrator_fetchOrders_LogsProviderFetch(t *testing.T) {
	mockProvider := new(MockProvider)
	mockProvider.On("DisplayName").Return("Costco")
	mockProvider.On("FetchOrders", mock.Anything, mock.Anything).Return([]providers.Order{
		&mockSimpleOrder{
			id:           "receipt-1",
			date:         time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
			total:        12.34,
			subtotal:     11.00,
			tax:          1.34,
			providerName: "Costco",
		},
	}, nil)

	store := storage.NewMockRepository()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	orchestrator := NewOrchestrator(mockProvider, nil, store, logger)
	orchestrator.runID = 77

	orders, err := orchestrator.fetchOrders(context.Background(), Options{LookbackDays: 14})
	require.NoError(t, err)
	require.Len(t, orders, 1)

	logs, err := store.GetProviderFetchesByRunID(77)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "Costco", logs[0].Provider)
	assert.Equal(t, "orders", logs[0].FetchType)
	assert.Equal(t, 1, logs[0].OrderCount)
	assert.Contains(t, logs[0].ResponseJSON, "receipt-1")
}
