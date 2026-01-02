package amazon

import (
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/stretchr/testify/assert"
)

// TestProvider_ImplementsInterface verifies the provider implements the interface
func TestProvider_ImplementsInterface(t *testing.T) {
	var _ providers.OrderProvider = (*Provider)(nil)
}

// TestProvider_Name tests the provider identification methods
func TestProvider_Name(t *testing.T) {
	provider := NewProvider(nil, nil)
	assert.Equal(t, "amazon", provider.Name())
	assert.Equal(t, "Amazon", provider.DisplayName())
}

// TestProvider_SupportsFeatures tests capability flags
func TestProvider_SupportsFeatures(t *testing.T) {
	provider := NewProvider(nil, nil)
	assert.False(t, provider.SupportsDeliveryTips(), "Amazon doesn't support delivery tips")
	assert.True(t, provider.SupportsRefunds(), "Amazon supports refunds")
	assert.True(t, provider.SupportsBulkFetch(), "Amazon supports bulk fetch")
}

// TestProvider_GetRateLimit tests rate limit configuration
func TestProvider_GetRateLimit(t *testing.T) {
	provider := NewProvider(nil, nil)
	assert.Equal(t, 1*time.Second, provider.GetRateLimit())
}

// TestProvider_MerchantSearchTerms tests merchant search terms
func TestProvider_MerchantSearchTerms(t *testing.T) {
	provider := NewProvider(nil, nil)
	terms := provider.MerchantSearchTerms()

	assert.Contains(t, terms, "Amazon")
	assert.Contains(t, terms, "AMZN")
	assert.Contains(t, terms, "AMZN Mktp US")
	assert.Contains(t, terms, "Whole Foods")
}

// TestProvider_WithConfig tests provider creation with config
func TestProvider_WithConfig(t *testing.T) {
	cfg := &ProviderConfig{
		Profile:  "wife",
		Headless: true,
	}

	provider := NewProvider(nil, cfg)
	assert.Equal(t, "wife", provider.profile)
	assert.True(t, provider.headless)
}

// TestProvider_BuildCLIArgs tests CLI argument building
func TestProvider_BuildCLIArgs(t *testing.T) {
	tests := []struct {
		name     string
		provider *Provider
		opts     providers.FetchOptions
		expected []string
	}{
		{
			name:     "default - no dates",
			provider: NewProvider(nil, nil),
			opts:     providers.FetchOptions{},
			expected: []string{"--days", "14", "--stdout"},
		},
		{
			name:     "with date range",
			provider: NewProvider(nil, nil),
			opts: providers.FetchOptions{
				StartDate: time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 11, 30, 0, 0, 0, 0, time.UTC),
			},
			expected: []string{"--since", "2024-11-01", "--until", "2024-11-30", "--stdout"},
		},
		{
			name:     "with profile",
			provider: NewProvider(nil, &ProviderConfig{Profile: "wife"}),
			opts:     providers.FetchOptions{},
			expected: []string{"--days", "14", "--profile", "wife", "--stdout"},
		},
		{
			name:     "with headless",
			provider: NewProvider(nil, &ProviderConfig{Headless: true}),
			opts:     providers.FetchOptions{},
			expected: []string{"--days", "14", "--headless", "--stdout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.provider.buildCLIArgs(tt.opts)
			assert.Equal(t, tt.expected, args)
		})
	}
}

// TestOrder_Interface verifies Order implements providers.Order
func TestOrder_Interface(t *testing.T) {
	var _ providers.Order = (*Order)(nil)
}

// TestOrderItem_Interface verifies OrderItem implements providers.OrderItem
func TestOrderItem_Interface(t *testing.T) {
	var _ providers.OrderItem = (*OrderItem)(nil)
}

// TestOrder_Methods tests Order adapter methods
func TestOrder_Methods(t *testing.T) {
	testDate := time.Date(2024, 11, 21, 0, 0, 0, 0, time.UTC)
	parsedOrder := &ParsedOrder{
		ID:       "114-9733092-9360267",
		Date:     testDate,
		Total:    44.91,
		Subtotal: 42.37,
		Tax:      2.54,
		Shipping: 0,
		Items: []*ParsedOrderItem{
			{
				Name:     "Test Item 1",
				Price:    14.99,
				Quantity: 1,
			},
			{
				Name:     "Test Item 2",
				Price:    27.38,
				Quantity: 2,
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	assert.Equal(t, "114-9733092-9360267", order.GetID())
	assert.Equal(t, testDate, order.GetDate())
	assert.Equal(t, 44.91, order.GetTotal())
	assert.Equal(t, 42.37, order.GetSubtotal())
	assert.Equal(t, 2.54, order.GetTax())
	assert.Equal(t, 0.0, order.GetTip())
	assert.Equal(t, 0.0, order.GetFees())
	assert.Equal(t, "Amazon", order.GetProviderName())
	assert.Len(t, order.GetItems(), 2)
	assert.Equal(t, parsedOrder, order.GetRawData())
}

// TestOrderItem_Methods tests OrderItem adapter methods
func TestOrderItem_Methods(t *testing.T) {
	parsedItem := &ParsedOrderItem{
		Name:     "Test Product",
		Price:    29.99,
		Quantity: 2,
	}

	item := &OrderItem{parsedItem: parsedItem}

	assert.Equal(t, "Test Product", item.GetName())
	assert.Equal(t, 29.99, item.GetPrice())
	assert.Equal(t, 2.0, item.GetQuantity())
	assert.Equal(t, 14.995, item.GetUnitPrice()) // 29.99 / 2
	assert.Equal(t, "", item.GetSKU())           // Not available from CLI
	assert.Equal(t, "Test Product", item.GetDescription())
	assert.Equal(t, "", item.GetCategory()) // Not available from CLI
}

// TestNewOrder_EmptyItems tests handling of orders with no items
func TestNewOrder_EmptyItems(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:    "114-0000000-0000000",
		Total: 0,
		Items: nil,
	}

	order := NewOrder(parsedOrder, nil)
	assert.Len(t, order.GetItems(), 0)
}

// TestNewProvider_NilLogger tests provider creation with nil logger
func TestNewProvider_NilLogger(t *testing.T) {
	provider := NewProvider(nil, nil)
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.logger)
}

// TestOrder_GetFinalCharges_NoTransactions tests GetFinalCharges returns ErrPaymentPending without transactions
func TestOrder_GetFinalCharges_NoTransactions(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:           "114-0000000-0000000",
		Total:        50.00,
		Transactions: nil,
	}

	order := NewOrder(parsedOrder, nil)
	charges, err := order.GetFinalCharges()

	assert.Error(t, err)
	assert.Nil(t, charges)
	assert.ErrorIs(t, err, ErrPaymentPending, "Should return ErrPaymentPending for orders without transactions")
}

// TestOrder_IsMultiDelivery_NoTransactions tests IsMultiDelivery returns ErrPaymentPending without transactions
func TestOrder_IsMultiDelivery_NoTransactions(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:           "114-0000000-0000000",
		Total:        50.00,
		Transactions: nil,
	}

	order := NewOrder(parsedOrder, nil)
	isMulti, err := order.IsMultiDelivery()

	assert.Error(t, err)
	assert.False(t, isMulti)
	assert.ErrorIs(t, err, ErrPaymentPending, "Should return ErrPaymentPending for orders without transactions")
}

// TestCalculateLookbackDays tests lookback days calculation
func TestCalculateLookbackDays(t *testing.T) {
	tests := []struct {
		name      string
		startDate time.Time
		endDate   time.Time
		expected  int
	}{
		{
			name:      "zero dates",
			startDate: time.Time{},
			endDate:   time.Time{},
			expected:  14,
		},
		{
			name:      "7 days apart",
			startDate: time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
			endDate:   time.Date(2024, 11, 8, 0, 0, 0, 0, time.UTC),
			expected:  7,
		},
		{
			name:      "same day",
			startDate: time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
			endDate:   time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
			expected:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateLookbackDays(tt.startDate, tt.endDate)
			assert.Equal(t, tt.expected, result)
		})
	}
}
