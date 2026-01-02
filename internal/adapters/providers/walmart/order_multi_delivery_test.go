package walmart

import (
	"testing"

	walmartclient "github.com/eshaffer321/walmart-client-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrder_GetFinalCharges tests retrieving final charges from order ledger
func TestOrder_GetFinalCharges(t *testing.T) {
	t.Run("single delivery order", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST123"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "TEST123",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						CardType:     "VISA",
						LastFour:     "1234",
						FinalCharges: []float64{126.98},
						TotalCharged: 126.98,
					},
				},
			},
		}

		charges, err := order.GetFinalCharges()
		require.NoError(t, err)
		assert.Len(t, charges, 1)
		assert.Equal(t, 126.98, charges[0])
	})

	t.Run("multi-delivery order", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST456"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "TEST456",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						CardType:     "VISA",
						LastFour:     "1234",
						FinalCharges: []float64{118.67, 8.31},
						TotalCharged: 126.98,
					},
				},
			},
		}

		charges, err := order.GetFinalCharges()
		require.NoError(t, err)
		assert.Len(t, charges, 2)
		assert.Equal(t, 118.67, charges[0])
		assert.Equal(t, 8.31, charges[1])
	})

	t.Run("uses cache on subsequent calls", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST789"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "TEST789",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{100.00},
						TotalCharged: 100.00,
					},
				},
			},
		}

		// First call
		charges1, err := order.GetFinalCharges()
		require.NoError(t, err)

		// Second call should use cache
		charges2, err := order.GetFinalCharges()
		require.NoError(t, err)

		assert.Equal(t, charges1, charges2)
	})

	t.Run("error - order not yet charged (payment pending)", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST000"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID:        "TEST000",
				PaymentMethods: []walmartclient.PaymentMethodCharges{},
			},
		}

		_, err := order.GetFinalCharges()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "payment pending")
	})

	t.Run("error - client not available", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST001"},
			client:       nil,
			ledgerCache:  nil,
		}

		_, err := order.GetFinalCharges()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client not available")
	})

	t.Run("error - empty final charges", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST002"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "TEST002",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{},
						TotalCharged: 0,
					},
				},
			},
		}

		_, err := order.GetFinalCharges()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no positive charges found")
	})

	t.Run("filters negative charge amounts (refunds)", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST003"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "TEST003",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{100.00, -50.00},
						TotalCharged: 50.00,
					},
				},
			},
		}

		charges, err := order.GetFinalCharges()
		require.NoError(t, err)
		assert.Len(t, charges, 1, "should filter out negative charges")
		assert.Equal(t, 100.00, charges[0], "should return only positive charge")
	})

	t.Run("filters zero charge amounts", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST004"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "TEST004",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{100.00, 0.00},
						TotalCharged: 100.00,
					},
				},
			},
		}

		charges, err := order.GetFinalCharges()
		require.NoError(t, err)
		assert.Len(t, charges, 1, "should filter out zero charges")
		assert.Equal(t, 100.00, charges[0], "should return only positive charge")
	})

	t.Run("real world - order with refund and gift card", func(t *testing.T) {
		// Based on actual order 200013800734485:
		// - VISA charge: $95.90
		// - Refund: -$4.63
		// - Gift card: $5.00
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "200013800734485"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "200013800734485",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{-4.63, 95.9},
						TotalCharged: 91.27,
					},
					{
						PaymentType:  "GIFTCARD",
						FinalCharges: []float64{5.0},
						TotalCharged: 5.0,
					},
				},
			},
		}

		charges, err := order.GetFinalCharges()
		require.NoError(t, err)
		assert.Len(t, charges, 1, "should only return VISA charge (gift card doesn't appear in bank)")
		assert.Equal(t, 95.9, charges[0], "should return the VISA charge, not the refund")
	})

	t.Run("error - all charges are negative (full refund)", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TEST005"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "TEST005",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{-100.00, -50.00},
						TotalCharged: -150.00,
					},
				},
			},
		}

		_, err := order.GetFinalCharges()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no positive charges found")
	})

	t.Run("filters gift card payments (only returns credit card)", func(t *testing.T) {
		// Order 200014031633755 - gift card $1.50, VISA $64.36
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "200014031633755"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "200014031633755",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "GIFTCARD",
						CardType:     "WMTRC",
						FinalCharges: []float64{1.5},
						TotalCharged: 1.5,
					},
					{
						PaymentType:  "CREDITCARD",
						CardType:     "VISA",
						FinalCharges: []float64{64.36},
						TotalCharged: 64.36,
					},
				},
			},
		}

		charges, err := order.GetFinalCharges()
		require.NoError(t, err)
		assert.Len(t, charges, 1, "should only return credit card charge, not gift card")
		assert.Equal(t, 64.36, charges[0], "should return VISA charge, not gift card")
	})
}

// TestOrder_IsMultiDelivery tests multi-delivery detection
func TestOrder_IsMultiDelivery(t *testing.T) {
	t.Run("single delivery returns false", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "SINGLE"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "SINGLE",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{126.98},
						TotalCharged: 126.98,
					},
				},
			},
		}

		isMulti, err := order.IsMultiDelivery()
		require.NoError(t, err)
		assert.False(t, isMulti)
	})

	t.Run("multi-delivery returns true", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "MULTI"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "MULTI",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{118.67, 8.31},
						TotalCharged: 126.98,
					},
				},
			},
		}

		isMulti, err := order.IsMultiDelivery()
		require.NoError(t, err)
		assert.True(t, isMulti)
	})

	t.Run("three deliveries returns true", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "TRIPLE"},
			ledgerCache: &walmartclient.OrderLedger{
				OrderID: "TRIPLE",
				PaymentMethods: []walmartclient.PaymentMethodCharges{
					{
						PaymentType:  "CREDITCARD",
						FinalCharges: []float64{50.00, 30.00, 20.00},
						TotalCharged: 100.00,
					},
				},
			},
		}

		isMulti, err := order.IsMultiDelivery()
		require.NoError(t, err)
		assert.True(t, isMulti)
	})

	t.Run("propagates GetFinalCharges error", func(t *testing.T) {
		order := &Order{
			walmartOrder: &walmartclient.Order{ID: "ERROR"},
			client:       nil,
			ledgerCache:  nil,
		}

		_, err := order.IsMultiDelivery()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client not available")
	})
}
