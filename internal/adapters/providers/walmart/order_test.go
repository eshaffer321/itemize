package walmart

import (
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	walmartclient "github.com/eshaffer321/walmart-client-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrder_ImplementsInterface verifies Order implements providers.Order
func TestOrder_ImplementsInterface(t *testing.T) {
	var _ providers.Order = (*Order)(nil)
}

// TestOrder_GetID tests order ID retrieval
func TestOrder_GetID(t *testing.T) {
	walmartOrder := &walmartclient.Order{
		ID:        "walmart-order-123",
		DisplayID: "WM123456",
	}

	order := &Order{walmartOrder: walmartOrder}

	assert.Equal(t, "walmart-order-123", order.GetID())
}

// TestOrder_GetDate tests date parsing
func TestOrder_GetDate(t *testing.T) {
	tests := []struct {
		name        string
		orderDate   string
		expectValid bool
	}{
		{
			name:        "valid ISO date",
			orderDate:   "2025-10-13T10:30:00Z",
			expectValid: true,
		},
		{
			name:        "valid date without timezone",
			orderDate:   "2025-10-13",
			expectValid: true,
		},
		{
			name:        "Walmart format with milliseconds and timezone",
			orderDate:   "2025-10-09T13:55:58.000-0600",
			expectValid: true,
		},
		{
			name:        "Walmart format without milliseconds",
			orderDate:   "2025-10-09T13:55:58-0600",
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			walmartOrder := &walmartclient.Order{
				ID:        "test-order",
				OrderDate: tt.orderDate,
			}

			order := &Order{walmartOrder: walmartOrder}
			date := order.GetDate()

			if tt.expectValid {
				assert.False(t, date.IsZero())
				assert.Equal(t, 2025, date.Year())
				assert.Equal(t, time.October, date.Month())
				// Day varies by test case
				if tt.orderDate == "2025-10-09T13:55:58.000-0600" || tt.orderDate == "2025-10-09T13:55:58-0600" {
					assert.Equal(t, 9, date.Day())
				} else {
					assert.Equal(t, 13, date.Day())
				}
			}
		})
	}
}

// TestOrder_GetTotal tests total amount calculation
func TestOrder_GetTotal(t *testing.T) {
	tests := []struct {
		name          string
		grandTotal    *walmartclient.PriceLineItem
		driverTip     *walmartclient.PriceLineItem
		expectedTotal float64
	}{
		{
			name: "order without tip",
			grandTotal: &walmartclient.PriceLineItem{
				Value: 150.00,
			},
			driverTip:     nil,
			expectedTotal: 150.00,
		},
		{
			name: "order with tip",
			grandTotal: &walmartclient.PriceLineItem{
				Value: 150.00,
			},
			driverTip: &walmartclient.PriceLineItem{
				Value: 15.00,
			},
			expectedTotal: 165.00,
		},
		{
			name:          "order with no price details",
			grandTotal:    nil,
			driverTip:     nil,
			expectedTotal: 0.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			walmartOrder := &walmartclient.Order{
				ID: "test-order",
			}
			if tt.grandTotal != nil || tt.driverTip != nil {
				walmartOrder.PriceDetails = &walmartclient.OrderPriceDetails{
					GrandTotal: tt.grandTotal,
					DriverTip:  tt.driverTip,
				}
			}

			order := &Order{walmartOrder: walmartOrder}

			assert.Equal(t, tt.expectedTotal, order.GetTotal())
		})
	}
}

// TestOrder_GetSubtotal tests subtotal calculation
func TestOrder_GetSubtotal(t *testing.T) {
	walmartOrder := &walmartclient.Order{
		ID: "test-order",
		PriceDetails: &walmartclient.OrderPriceDetails{
			SubTotal: &walmartclient.PriceLineItem{
				Value: 135.50,
			},
		},
	}

	order := &Order{walmartOrder: walmartOrder}

	assert.Equal(t, 135.50, order.GetSubtotal())
}

// TestOrder_GetTax tests tax calculation
func TestOrder_GetTax(t *testing.T) {
	walmartOrder := &walmartclient.Order{
		ID: "test-order",
		PriceDetails: &walmartclient.OrderPriceDetails{
			TaxTotal: &walmartclient.PriceLineItem{
				Value: 14.50,
			},
		},
	}

	order := &Order{walmartOrder: walmartOrder}

	assert.Equal(t, 14.50, order.GetTax())
}

// TestOrder_GetTip tests tip calculation
func TestOrder_GetTip(t *testing.T) {
	walmartOrder := &walmartclient.Order{
		ID: "test-order",
		PriceDetails: &walmartclient.OrderPriceDetails{
			DriverTip: &walmartclient.PriceLineItem{
				Value: 10.00,
			},
		},
	}

	order := &Order{walmartOrder: walmartOrder}

	assert.Equal(t, 10.00, order.GetTip())
}

// TestOrder_GetFees tests fee calculation
func TestOrder_GetFees(t *testing.T) {
	walmartOrder := &walmartclient.Order{
		ID: "test-order",
		PriceDetails: &walmartclient.OrderPriceDetails{
			Fees: []walmartclient.PriceLineItem{
				{Label: "Delivery Fee", Value: 5.99},
				{Label: "Service Fee", Value: 2.00},
			},
		},
	}

	order := &Order{walmartOrder: walmartOrder}

	assert.Equal(t, 7.99, order.GetFees())
}

// TestOrder_GetItems tests item retrieval
func TestOrder_GetItems(t *testing.T) {
	walmartOrder := &walmartclient.Order{
		ID: "test-order",
		Groups: []walmartclient.OrderGroup{
			{
				Items: []walmartclient.OrderItem{
					{
						Quantity: 2,
						ProductInfo: &walmartclient.ProductInfo{
							Name: "Test Product",
						},
						PriceInfo: &walmartclient.ItemPrice{
							LinePrice: &walmartclient.Price{
								Value: 10.00,
							},
							UnitPrice: &walmartclient.Price{
								Value: 5.00,
							},
						},
					},
				},
			},
		},
	}

	order := &Order{walmartOrder: walmartOrder}
	items := order.GetItems()

	require.Len(t, items, 1)
	assert.Equal(t, "Test Product", items[0].GetName())
	assert.Equal(t, 10.00, items[0].GetPrice())
	assert.Equal(t, 2.0, items[0].GetQuantity())
	assert.Equal(t, 5.00, items[0].GetUnitPrice())
}

// TestOrder_GetProviderName tests provider name
func TestOrder_GetProviderName(t *testing.T) {
	order := &Order{walmartOrder: &walmartclient.Order{ID: "test"}}
	assert.Equal(t, "walmart", order.GetProviderName())
}

// TestOrder_GetRawData tests raw data access
func TestOrder_GetRawData(t *testing.T) {
	walmartOrder := &walmartclient.Order{ID: "test-order"}
	order := &Order{walmartOrder: walmartOrder}

	rawData := order.GetRawData()
	assert.Equal(t, walmartOrder, rawData)
}
