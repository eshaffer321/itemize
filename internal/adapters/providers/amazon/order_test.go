package amazon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrder_GetFinalCharges_FilterNonBankTransactions(t *testing.T) {
	// Create an order with transactions
	parsedOrder := &ParsedOrder{
		ID:    "112-4559127-2161020",
		Date:  time.Now(),
		Total: 103.27,
		Transactions: []*ParsedTransaction{
			{
				Amount:      52.55,
				Description: "Prime Visa ****1211",
				Last4:       "1211",
				Type:        "charge",
				Date:        time.Now(),
			},
			{
				Amount:      50.72,
				Description: "Prime Visa ****1211",
				Last4:       "1211",
				Type:        "charge",
				Date:        time.Now(),
			},
			{
				Amount:      8.03,
				Description: "Amazon Visa points",
				Last4:       "", // No card digits = not a bank transaction
				Type:        "charge",
				Date:        time.Now(),
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	charges, err := order.GetFinalCharges()
	assert.NoError(t, err)

	// Should only return the two bank charges, not the points
	assert.Len(t, charges, 2, "Should filter out Amazon Visa points")
	assert.Contains(t, charges, 52.55)
	assert.Contains(t, charges, 50.72)
	assert.NotContains(t, charges, 8.03, "Should not include points transaction")
}

func TestOrder_GetFinalCharges_OnlyGiftCard(t *testing.T) {
	// Test order paid entirely with gift card
	parsedOrder := &ParsedOrder{
		ID:    "test-gift-card-order",
		Date:  time.Now(),
		Total: 50.00,
		Transactions: []*ParsedTransaction{
			{
				Amount:      50.00,
				Description: "Gift Card",
				Last4:       "", // No card digits
				Type:        "charge",
				Date:        time.Now(),
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	charges, err := order.GetFinalCharges()
	assert.Error(t, err, "Should return error when no bank charges found")
	assert.Nil(t, charges)
	assert.Contains(t, err.Error(), "paid entirely with gift cards/points")
}

func TestOrder_GetFinalCharges_NoTransactions_ReturnsPending(t *testing.T) {
	// Test order with no transactions (not yet shipped/charged)
	parsedOrder := &ParsedOrder{
		ID:           "test-pending-order",
		Date:         time.Now(),
		Total:        50.00,
		Transactions: []*ParsedTransaction{}, // Empty - not charged yet
	}

	order := NewOrder(parsedOrder, nil)

	charges, err := order.GetFinalCharges()
	assert.Error(t, err, "Should return error when no transactions")
	assert.Nil(t, charges)
	assert.ErrorIs(t, err, ErrPaymentPending, "Should return ErrPaymentPending for orders not yet charged")
}

func TestOrder_GetFinalCharges_NilTransactions_ReturnsPending(t *testing.T) {
	// Test order with nil transactions slice
	parsedOrder := &ParsedOrder{
		ID:           "test-pending-order-nil",
		Date:         time.Now(),
		Total:        50.00,
		Transactions: nil, // Nil - not charged yet
	}

	order := NewOrder(parsedOrder, nil)

	charges, err := order.GetFinalCharges()
	assert.Error(t, err, "Should return error when no transactions")
	assert.Nil(t, charges)
	assert.ErrorIs(t, err, ErrPaymentPending, "Should return ErrPaymentPending for orders not yet charged")
}

func TestOrder_GetFinalCharges_SkipsRefunds(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:    "test-with-refund",
		Date:  time.Now(),
		Total: 50.00,
		Transactions: []*ParsedTransaction{
			{
				Amount:      50.00,
				Description: "Prime Visa ****1211",
				Last4:       "1211",
				Type:        "charge",
				Date:        time.Now(),
			},
			{
				Amount:      25.00,
				Description: "Prime Visa ****1211",
				Last4:       "1211",
				Type:        "refund", // Should be skipped
				Date:        time.Now(),
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	charges, err := order.GetFinalCharges()
	assert.NoError(t, err)
	assert.Len(t, charges, 1, "Should skip refund transactions")
	assert.Equal(t, 50.00, charges[0])
}

func TestOrder_IsMultiDelivery_WithFilteredPoints(t *testing.T) {
	// Test that multi-delivery detection correctly filters points
	parsedOrder := &ParsedOrder{
		ID:    "112-4559127-2161020",
		Date:  time.Now(),
		Total: 103.27,
		Transactions: []*ParsedTransaction{
			{
				Amount: 52.55,
				Last4:  "1211",
				Type:   "charge",
			},
			{
				Amount: 50.72,
				Last4:  "1211",
				Type:   "charge",
			},
			{
				Amount: 8.03,
				Last4:  "", // Points - not bank
				Type:   "charge",
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	isMulti, err := order.IsMultiDelivery()
	assert.NoError(t, err)
	assert.True(t, isMulti, "Should detect 2 bank charges as multi-delivery")
}

func TestOrder_IsMultiDelivery_SingleCharge(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:    "test-single-order",
		Date:  time.Now(),
		Total: 50.00,
		Transactions: []*ParsedTransaction{
			{
				Amount: 50.00,
				Last4:  "1211",
				Type:   "charge",
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	isMulti, err := order.IsMultiDelivery()
	assert.NoError(t, err)
	assert.False(t, isMulti, "Single charge should not be multi-delivery")
}

func TestOrder_GetNonBankAmount_WithPoints(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:    "112-4559127-2161020",
		Date:  time.Now(),
		Total: 111.30,
		Transactions: []*ParsedTransaction{
			{
				Amount: 52.55,
				Last4:  "1211",
				Type:   "charge",
			},
			{
				Amount: 50.72,
				Last4:  "1211",
				Type:   "charge",
			},
			{
				Amount:      8.03,
				Description: "Amazon Visa points",
				Last4:       "", // No card digits = non-bank
				Type:        "charge",
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	nonBankAmount, err := order.GetNonBankAmount()
	assert.NoError(t, err)
	assert.Equal(t, 8.03, nonBankAmount, "Should return points amount")
}

func TestOrder_GetNonBankAmount_NoNonBankPayments(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:    "test-no-points",
		Date:  time.Now(),
		Total: 100.00,
		Transactions: []*ParsedTransaction{
			{
				Amount: 100.00,
				Last4:  "1211",
				Type:   "charge",
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	nonBankAmount, err := order.GetNonBankAmount()
	assert.NoError(t, err)
	assert.Equal(t, 0.0, nonBankAmount, "Should return 0 when no non-bank payments")
}

func TestOrder_GetNonBankAmount_GiftCardAndPoints(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:    "test-gift-and-points",
		Date:  time.Now(),
		Total: 150.00,
		Transactions: []*ParsedTransaction{
			{
				Amount: 100.00,
				Last4:  "1211",
				Type:   "charge",
			},
			{
				Amount:      25.00,
				Description: "Gift Card",
				Last4:       "",
				Type:        "charge",
			},
			{
				Amount:      25.00,
				Description: "Amazon Visa points",
				Last4:       "",
				Type:        "charge",
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	nonBankAmount, err := order.GetNonBankAmount()
	assert.NoError(t, err)
	assert.Equal(t, 50.00, nonBankAmount, "Should sum gift card and points")
}

func TestOrder_GetNonBankAmount_SkipsRefunds(t *testing.T) {
	parsedOrder := &ParsedOrder{
		ID:    "test-with-refund",
		Date:  time.Now(),
		Total: 75.00,
		Transactions: []*ParsedTransaction{
			{
				Amount: 100.00,
				Last4:  "1211",
				Type:   "charge",
			},
			{
				Amount:      25.00,
				Description: "Gift Card",
				Last4:       "",
				Type:        "charge",
			},
			{
				Amount:      -50.00, // Refund - should be skipped
				Description: "Gift Card",
				Last4:       "",
				Type:        "refund",
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	nonBankAmount, err := order.GetNonBankAmount()
	assert.NoError(t, err)
	assert.Equal(t, 25.00, nonBankAmount, "Should skip refunds")
}

func TestOrder_GetTransactionDates(t *testing.T) {
	date1 := time.Date(2024, 11, 21, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 11, 22, 0, 0, 0, 0, time.UTC)

	parsedOrder := &ParsedOrder{
		ID:    "test-dates",
		Date:  date1,
		Total: 100.00,
		Transactions: []*ParsedTransaction{
			{
				Amount: 50.00,
				Last4:  "1211",
				Type:   "charge",
				Date:   date1,
			},
			{
				Amount: 50.00,
				Last4:  "1211",
				Type:   "charge",
				Date:   date2,
			},
			{
				Amount: 25.00,
				Last4:  "1211",
				Type:   "refund", // Refund - should be excluded
				Date:   date2,
			},
		},
	}

	order := NewOrder(parsedOrder, nil)

	dates := order.GetTransactionDates()
	assert.Len(t, dates, 2)
	assert.Contains(t, dates, date1)
	assert.Contains(t, dates, date2)
}
