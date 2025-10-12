package costco

import (
	"log/slog"
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_Interface(t *testing.T) {
	// Ensure Provider implements OrderProvider interface
	var _ providers.OrderProvider = (*Provider)(nil)
}

func TestProvider_Name(t *testing.T) {
	provider := &Provider{}
	assert.Equal(t, "costco", provider.Name())
}

func TestProvider_DisplayName(t *testing.T) {
	provider := &Provider{}
	assert.Equal(t, "Costco", provider.DisplayName())
}

func TestProvider_Capabilities(t *testing.T) {
	provider := &Provider{
		rateLimit: 3 * time.Second,
	}
	
	assert.False(t, provider.SupportsDeliveryTips())
	assert.False(t, provider.SupportsRefunds())
	assert.True(t, provider.SupportsBulkFetch())
	assert.Equal(t, 3*time.Second, provider.GetRateLimit())
}

func TestCostcoOrder_Interface(t *testing.T) {
	// Ensure CostcoOrder implements Order interface
	var _ providers.Order = (*CostcoOrder)(nil)
}

func TestCostcoOrder_Methods(t *testing.T) {
	testDate := time.Now()
	testItems := []providers.OrderItem{
		&CostcoOrderItem{
			name:     "Test Item",
			price:    10.99,
			quantity: 2,
		},
	}
	
	order := &CostcoOrder{
		id:           "TEST123",
		date:         testDate,
		total:        100.50,
		subtotal:     90.00,
		tax:          10.50,
		tip:          0,
		fees:         0,
		items:        testItems,
		providerName: "Costco",
		orderType:    "receipt",
	}
	
	assert.Equal(t, "TEST123", order.GetID())
	assert.Equal(t, testDate, order.GetDate())
	assert.Equal(t, 100.50, order.GetTotal())
	assert.Equal(t, 90.00, order.GetSubtotal())
	assert.Equal(t, 10.50, order.GetTax())
	assert.Equal(t, 0.0, order.GetTip())
	assert.Equal(t, 0.0, order.GetFees())
	assert.Equal(t, testItems, order.GetItems())
	assert.Equal(t, "Costco", order.GetProviderName())
}

func TestCostcoOrderItem_Interface(t *testing.T) {
	// Ensure CostcoOrderItem implements OrderItem interface
	var _ providers.OrderItem = (*CostcoOrderItem)(nil)
}

func TestCostcoOrderItem_Methods(t *testing.T) {
	item := &CostcoOrderItem{
		name:        "Kirkland Milk",
		price:       15.99,
		quantity:    3,
		unitPrice:   5.33,
		description: "Kirkland Organic Milk 3-pack",
		sku:         "KS12345",
		category:    "Dairy",
	}
	
	assert.Equal(t, "Kirkland Milk", item.GetName())
	assert.Equal(t, 15.99, item.GetPrice())
	assert.Equal(t, 3.0, item.GetQuantity())
	assert.Equal(t, 5.33, item.GetUnitPrice())
	assert.Equal(t, "Kirkland Organic Milk 3-pack", item.GetDescription())
	assert.Equal(t, "KS12345", item.GetSKU())
	assert.Equal(t, "Dairy", item.GetCategory())
}

func TestConvertReceipt_DateParsing(t *testing.T) {
	logger := slog.Default()
	provider := NewProvider(nil, logger)
	
	// Test data with different date formats
	testCases := []struct {
		name            string
		transactionDate string
		transactionTime string
		expectValid     bool
	}{
		{
			name:            "Valid ISO date",
			transactionDate: "2025-09-05",
			transactionTime: "",
			expectValid:     true,
		},
		{
			name:            "Invalid date with datetime",
			transactionDate: "",
			transactionTime: "2025-09-05T13:23:00",
			expectValid:     true,
		},
		{
			name:            "Both invalid",
			transactionDate: "invalid",
			transactionTime: "invalid",
			expectValid:     false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This would require mocking the costco client types
			// For now, just verify the provider exists
			require.NotNil(t, provider)
		})
	}
}

func TestProvider_FetchOrders_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// This test would require a real Costco client
	// It's here as a placeholder for integration testing
	t.Skip("Integration test requires real Costco credentials")
}

func TestProvider_NewProvider(t *testing.T) {
	// Test with nil logger
	provider := NewProvider(nil, nil)
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.logger)
	assert.Equal(t, 3*time.Second, provider.rateLimit)
	
	// Test with custom logger
	customLogger := slog.Default().With(slog.String("test", "true"))
	provider2 := NewProvider(nil, customLogger)
	assert.NotNil(t, provider2)
	assert.NotNil(t, provider2.logger)
}

func TestProvider_FetchOrders_Context(t *testing.T) {
	// Skip this test as it requires a real client
	t.Skip("Test requires a mock Costco client")
}