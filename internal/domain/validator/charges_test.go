package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCharges_ValidWithPoints(t *testing.T) {
	// Real order 112-4559127-2161020
	// Order total: $103.27
	// Bank charges: $52.55 + $50.72 = $103.27
	// Points used: $8.03 (but order total already reflects this)
	bankCharges := []float64{52.55, 50.72}
	orderTotal := 103.27
	nonBankAmount := 0.0 // Points already deducted from order total

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.True(t, result.Valid)
	assert.Equal(t, 103.27, result.BankChargesSum)
	assert.Equal(t, 103.27, result.ExpectedSum)
	assert.Empty(t, result.Reason)
}

func TestValidateCharges_ValidWithNonBankAmount(t *testing.T) {
	// Order where total includes gift card
	// Grand total before gift card: $150.00
	// Gift card: $50.00
	// Bank charges should be: $100.00
	bankCharges := []float64{100.00}
	orderTotal := 150.00
	nonBankAmount := 50.00

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.True(t, result.Valid)
	assert.Equal(t, 100.00, result.BankChargesSum)
	assert.Equal(t, 100.00, result.ExpectedSum)
}

func TestValidateCharges_ValidMultipleCharges(t *testing.T) {
	// Three shipments
	bankCharges := []float64{33.33, 33.33, 33.34}
	orderTotal := 100.00
	nonBankAmount := 0.0

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.True(t, result.Valid)
	assert.Equal(t, 100.00, result.BankChargesSum)
}

func TestValidateCharges_ValidWithinTolerance(t *testing.T) {
	// Off by 1 cent - should still be valid
	bankCharges := []float64{99.99}
	orderTotal := 100.00
	nonBankAmount := 0.0

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.True(t, result.Valid, "1 cent difference should be within tolerance")
}

func TestValidateCharges_ValidExactlyAtTolerance(t *testing.T) {
	// Off by exactly 2 cents - should still be valid
	bankCharges := []float64{99.98}
	orderTotal := 100.00
	nonBankAmount := 0.0

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.True(t, result.Valid, "2 cent difference should be within tolerance")
}

func TestValidateCharges_InvalidMissingCharge(t *testing.T) {
	// Missing one shipment charge
	// Expected: $52.55 + $50.72 = $103.27
	// Actual: only $52.55
	bankCharges := []float64{52.55}
	orderTotal := 103.27
	nonBankAmount := 0.0

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.False(t, result.Valid)
	assert.Equal(t, 52.55, result.BankChargesSum)
	assert.Equal(t, 103.27, result.ExpectedSum)
	assert.InDelta(t, -50.72, result.Difference, 0.01)
	assert.Contains(t, result.Reason, "less than expected")
	assert.Contains(t, result.Reason, "hasn't posted")
}

func TestValidateCharges_InvalidExtraCharge(t *testing.T) {
	// Extra charge (maybe duplicate)
	bankCharges := []float64{50.00, 50.00, 50.00}
	orderTotal := 100.00
	nonBankAmount := 0.0

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.False(t, result.Valid)
	assert.Equal(t, 150.00, result.BankChargesSum)
	assert.Equal(t, 100.00, result.ExpectedSum)
	assert.InDelta(t, 50.00, result.Difference, 0.01)
	assert.Contains(t, result.Reason, "exceed expected")
}

func TestValidateCharges_InvalidJustOutsideTolerance(t *testing.T) {
	// Off by 3 cents - just outside tolerance
	bankCharges := []float64{99.97}
	orderTotal := 100.00
	nonBankAmount := 0.0

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.False(t, result.Valid, "3 cent difference should exceed tolerance")
}

func TestValidateCharges_EmptyCharges(t *testing.T) {
	// No charges at all
	bankCharges := []float64{}
	orderTotal := 100.00
	nonBankAmount := 0.0

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.False(t, result.Valid)
	assert.Equal(t, 0.0, result.BankChargesSum)
	assert.Contains(t, result.Reason, "less than expected")
}

func TestValidateCharges_ZeroOrderTotal(t *testing.T) {
	// Free order (100% covered by gift card)
	bankCharges := []float64{}
	orderTotal := 0.0
	nonBankAmount := 0.0

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.True(t, result.Valid)
	assert.Equal(t, 0.0, result.BankChargesSum)
	assert.Equal(t, 0.0, result.ExpectedSum)
}

func TestValidateCharges_FullyPaidByGiftCard(t *testing.T) {
	// Order fully paid by gift card
	bankCharges := []float64{}
	orderTotal := 50.00
	nonBankAmount := 50.00 // Gift card covers everything

	result := ValidateCharges(bankCharges, orderTotal, nonBankAmount)

	assert.True(t, result.Valid)
	assert.Equal(t, 0.0, result.BankChargesSum)
	assert.Equal(t, 0.0, result.ExpectedSum)
}

func TestValidateChargesSimple(t *testing.T) {
	// Test the convenience function
	bankCharges := []float64{50.00, 50.00}
	orderTotal := 100.00

	result := ValidateChargesSimple(bankCharges, orderTotal)

	assert.True(t, result.Valid)
	assert.Equal(t, 100.00, result.BankChargesSum)
}

func TestValidateCharges_RoundingEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		charges     []float64
		orderTotal  float64
		nonBank     float64
		expectValid bool
	}{
		{
			name:        "floating point precision issue",
			charges:     []float64{33.33, 33.33, 33.34},
			orderTotal:  100.00,
			nonBank:     0.0,
			expectValid: true,
		},
		{
			name:        "many small charges",
			charges:     []float64{10.00, 10.00, 10.00, 10.00, 10.00, 10.00, 10.00, 10.00, 10.00, 10.00},
			orderTotal:  100.00,
			nonBank:     0.0,
			expectValid: true,
		},
		{
			name:        "cents only",
			charges:     []float64{0.50, 0.50},
			orderTotal:  1.00,
			nonBank:     0.0,
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateCharges(tt.charges, tt.orderTotal, tt.nonBank)
			assert.Equal(t, tt.expectValid, result.Valid, "unexpected validation result for %s", tt.name)
		})
	}
}

func TestValidateCharges_ReasonMessages(t *testing.T) {
	t.Run("missing charge has helpful message", func(t *testing.T) {
		result := ValidateCharges([]float64{50.00}, 100.00, 0.0)
		assert.Contains(t, result.Reason, "$50.00")
		assert.Contains(t, result.Reason, "$100.00")
		assert.Contains(t, result.Reason, "missing")
	})

	t.Run("extra charge has helpful message", func(t *testing.T) {
		result := ValidateCharges([]float64{150.00}, 100.00, 0.0)
		assert.Contains(t, result.Reason, "exceed")
		assert.Contains(t, result.Reason, "$50.00")
	})
}
