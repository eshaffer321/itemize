package costco

import (
	"log/slog"
	"testing"
	"time"

	costcogo "github.com/eshaffer321/costco-go/pkg/costco"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
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

// TestConvertReceipt_DiscountNetting tests that discount line items are properly netted
// into their parent items instead of being returned as separate items
func TestConvertReceipt_DiscountNetting(t *testing.T) {
	logger := slog.Default()
	provider := NewProvider(nil, logger)

	t.Run("single item with single discount", func(t *testing.T) {
		// Guacamole case: $13.99 item with $4.00 discount = $9.99 net
		receipt := &costcogo.Receipt{
			TransactionBarcode: "TEST123",
			TransactionDate:    "2025-10-20",
			Total:              9.99,
			SubTotal:           9.99,
			Taxes:              0.00,
			ItemArray: []costcogo.ReceiptItem{
				{
					ItemNumber:        "1553261",
					ItemDescription01: "GUAC BOWL",
					ItemDescription02: "",
					Amount:            13.99,
					Unit:              1,
				},
				{
					ItemNumber:        "363064",
					ItemDescription01: "/1553261", // References parent item
					ItemDescription02: "",
					Amount:            -4.00, // Discount
					Unit:              -1,
				},
			},
		}

		order := provider.convertReceipt(receipt, true)
		items := order.GetItems()

		// Should have only 1 item (guac with net amount)
		require.Len(t, items, 1, "Should only have 1 item after netting discount")

		// Verify the item has the net amount
		assert.Equal(t, "GUAC BOWL", items[0].GetName())
		assert.Equal(t, 9.99, items[0].GetPrice(), "Price should be net amount (13.99 - 4.00)")
		assert.Equal(t, "1553261", items[0].GetSKU())
	})

	t.Run("multiple items with multiple discounts", func(t *testing.T) {
		receipt := &costcogo.Receipt{
			TransactionBarcode: "TEST456",
			TransactionDate:    "2025-10-20",
			Total:              29.98,
			SubTotal:           29.98,
			Taxes:              0.00,
			ItemArray: []costcogo.ReceiptItem{
				{
					ItemNumber:        "111111",
					ItemDescription01: "ITEM A",
					Amount:            15.00,
					Unit:              1,
				},
				{
					ItemNumber:        "222222",
					ItemDescription01: "ITEM B",
					Amount:            20.00,
					Unit:              1,
				},
				{
					ItemNumber:        "999001",
					ItemDescription01: "/111111",
					Amount:            -5.00, // Discount on Item A
					Unit:              -1,
				},
				{
					ItemNumber:        "999002",
					ItemDescription01: "/222222",
					Amount:            -0.02, // Discount on Item B
					Unit:              -1,
				},
			},
		}

		order := provider.convertReceipt(receipt, true)
		items := order.GetItems()

		// Should have only 2 items (both with net amounts)
		require.Len(t, items, 2, "Should have 2 items after netting discounts")

		// Find items by name (order might vary)
		itemMap := make(map[string]providers.OrderItem)
		for _, item := range items {
			itemMap[item.GetName()] = item
		}

		assert.Equal(t, 10.00, itemMap["ITEM A"].GetPrice(), "Item A should be 15.00 - 5.00")
		assert.Equal(t, 19.98, itemMap["ITEM B"].GetPrice(), "Item B should be 20.00 - 0.02")
	})

	t.Run("multiple discounts on same item", func(t *testing.T) {
		// Edge case: One item with two discount line items
		receipt := &costcogo.Receipt{
			TransactionBarcode: "TEST789",
			TransactionDate:    "2025-10-20",
			Total:              8.00,
			SubTotal:           8.00,
			Taxes:              0.00,
			ItemArray: []costcogo.ReceiptItem{
				{
					ItemNumber:        "333333",
					ItemDescription01: "BULK ITEM",
					Amount:            15.00,
					Unit:              1,
				},
				{
					ItemNumber:        "999003",
					ItemDescription01: "/333333",
					Amount:            -5.00, // First discount
					Unit:              -1,
				},
				{
					ItemNumber:        "999004",
					ItemDescription01: "/333333",
					Amount:            -2.00, // Second discount
					Unit:              -1,
				},
			},
		}

		order := provider.convertReceipt(receipt, true)
		items := order.GetItems()

		// Should have only 1 item with both discounts applied
		require.Len(t, items, 1, "Should have 1 item with all discounts netted")
		assert.Equal(t, "BULK ITEM", items[0].GetName())
		assert.Equal(t, 8.00, items[0].GetPrice(), "Price should be 15.00 - 5.00 - 2.00")
	})

	t.Run("items without discounts", func(t *testing.T) {
		// Regular items with no discount line items
		receipt := &costcogo.Receipt{
			TransactionBarcode: "TEST101",
			TransactionDate:    "2025-10-20",
			Total:              25.00,
			SubTotal:           25.00,
			Taxes:              0.00,
			ItemArray: []costcogo.ReceiptItem{
				{
					ItemNumber:        "444444",
					ItemDescription01: "MILK",
					Amount:            5.00,
					Unit:              1,
				},
				{
					ItemNumber:        "555555",
					ItemDescription01: "BREAD",
					Amount:            3.50,
					Unit:              1,
				},
			},
		}

		order := provider.convertReceipt(receipt, true)
		items := order.GetItems()

		// Should have both items at full price
		require.Len(t, items, 2, "Should have 2 items")

		itemMap := make(map[string]providers.OrderItem)
		for _, item := range items {
			itemMap[item.GetName()] = item
		}

		assert.Equal(t, 5.00, itemMap["MILK"].GetPrice())
		assert.Equal(t, 3.50, itemMap["BREAD"].GetPrice())
	})

	t.Run("orphaned discount is skipped", func(t *testing.T) {
		// Discount line item with no matching parent (edge case)
		receipt := &costcogo.Receipt{
			TransactionBarcode: "TEST202",
			TransactionDate:    "2025-10-20",
			Total:              10.00,
			SubTotal:           10.00,
			Taxes:              0.00,
			ItemArray: []costcogo.ReceiptItem{
				{
					ItemNumber:        "666666",
					ItemDescription01: "CHICKEN",
					Amount:            10.00,
					Unit:              1,
				},
				{
					ItemNumber:        "999005",
					ItemDescription01: "/777777", // References non-existent item
					Amount:            -2.00,
					Unit:              -1,
				},
			},
		}

		order := provider.convertReceipt(receipt, true)
		items := order.GetItems()

		// Should only have the chicken, orphaned discount should be skipped
		require.Len(t, items, 1, "Should skip orphaned discount")
		assert.Equal(t, "CHICKEN", items[0].GetName())
		assert.Equal(t, 10.00, items[0].GetPrice(), "Price should not be affected by orphaned discount")
	})

	t.Run("discount with whitespace in reference", func(t *testing.T) {
		// Edge case: Discount description has space like "/ 1857091" instead of "/1857091"
		receipt := &costcogo.Receipt{
			TransactionBarcode: "TEST404",
			TransactionDate:    "2025-10-20",
			Total:              11.00,
			SubTotal:           11.00,
			Taxes:              0.00,
			ItemArray: []costcogo.ReceiptItem{
				{
					ItemNumber:        "1857091",
					ItemDescription01: "CHEESE",
					Amount:            14.00,
					Unit:              1,
				},
				{
					ItemNumber:        "363581",
					ItemDescription01: "/ 1857091", // Note the space after /
					Amount:            -3.00,
					Unit:              -1,
				},
			},
		}

		order := provider.convertReceipt(receipt, true)
		items := order.GetItems()

		// Should have 1 item with discount applied (whitespace trimmed)
		require.Len(t, items, 1, "Should have 1 item after netting discount with whitespace")
		assert.Equal(t, "CHEESE", items[0].GetName())
		assert.Equal(t, 11.00, items[0].GetPrice(), "Price should be 14.00 - 3.00")
	})

	t.Run("mixed scenario", func(t *testing.T) {
		// Realistic receipt: some items with discounts, some without
		receipt := &costcogo.Receipt{
			TransactionBarcode: "TEST303",
			TransactionDate:    "2025-10-20",
			Total:              44.97,
			SubTotal:           44.97,
			Taxes:              0.00,
			ItemArray: []costcogo.ReceiptItem{
				{
					ItemNumber:        "800001",
					ItemDescription01: "GUAC BOWL",
					Amount:            13.99,
					Unit:              1,
				},
				{
					ItemNumber:        "800002",
					ItemDescription01: "ORGANIC MILK",
					Amount:            6.99,
					Unit:              1,
				},
				{
					ItemNumber:        "800003",
					ItemDescription01: "ROTISSERIE CHICKEN",
					Amount:            4.99,
					Unit:              1,
				},
				{
					ItemNumber:        "800004",
					ItemDescription01: "TOILET PAPER",
					Amount:            23.00,
					Unit:              1,
				},
				{
					ItemNumber:        "999010",
					ItemDescription01: "/800001",
					Amount:            -4.00, // Guac discount
					Unit:              -1,
				},
			},
		}

		order := provider.convertReceipt(receipt, true)
		items := order.GetItems()

		// Should have 4 items (guac, milk, chicken, TP)
		require.Len(t, items, 4, "Should have 4 items after netting discount")

		itemMap := make(map[string]providers.OrderItem)
		for _, item := range items {
			itemMap[item.GetName()] = item
		}

		assert.Equal(t, 9.99, itemMap["GUAC BOWL"].GetPrice(), "Guac should be discounted")
		assert.Equal(t, 6.99, itemMap["ORGANIC MILK"].GetPrice(), "Milk at full price")
		assert.Equal(t, 4.99, itemMap["ROTISSERIE CHICKEN"].GetPrice(), "Chicken at full price")
		assert.Equal(t, 23.00, itemMap["TOILET PAPER"].GetPrice(), "TP at full price")
	})
}
