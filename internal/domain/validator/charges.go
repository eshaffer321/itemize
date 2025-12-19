// Package validator provides validation logic for order processing.
//
// The charges validator ensures that bank charges sum correctly before
// processing an order. This prevents processing incomplete orders where
// not all charges have posted yet.
package validator

import (
	"fmt"
	"math"
)

// ChargeValidation contains the result of validating order charges.
type ChargeValidation struct {
	// Valid is true if the charges sum correctly
	Valid bool

	// BankChargesSum is the sum of all bank charges
	BankChargesSum float64

	// ExpectedSum is what the bank charges should sum to
	ExpectedSum float64

	// Difference is the gap between actual and expected
	Difference float64

	// Reason explains why validation failed (empty if valid)
	Reason string
}

// ValidateCharges checks that bank charges sum to the expected amount.
//
// For Amazon orders:
//   - orderTotal is the grand total from the order
//   - bankCharges are the actual card charges (excluding points/gift cards)
//   - nonBankAmount is the sum of points, gift cards, etc.
//
// The validation passes if:
//
//	sum(bankCharges) ≈ orderTotal - nonBankAmount
//
// A tolerance of 2 cents is allowed for rounding differences.
func ValidateCharges(bankCharges []float64, orderTotal, nonBankAmount float64) *ChargeValidation {
	// Sum bank charges
	var bankSum float64
	for _, charge := range bankCharges {
		bankSum += charge
	}
	bankSum = roundToCents(bankSum)

	// Calculate expected sum
	expectedSum := roundToCents(orderTotal - nonBankAmount)

	// Calculate difference
	diff := roundToCents(bankSum - expectedSum)

	// Allow 2 cent tolerance for rounding
	const tolerance = 0.02

	if math.Abs(diff) <= tolerance {
		return &ChargeValidation{
			Valid:          true,
			BankChargesSum: bankSum,
			ExpectedSum:    expectedSum,
			Difference:     diff,
			Reason:         "",
		}
	}

	// Validation failed - determine reason
	var reason string
	if diff < 0 {
		reason = fmt.Sprintf("bank charges ($%.2f) are less than expected ($%.2f) - missing $%.2f, likely a charge hasn't posted yet",
			bankSum, expectedSum, -diff)
	} else {
		reason = fmt.Sprintf("bank charges ($%.2f) exceed expected ($%.2f) by $%.2f - possible duplicate or extra charge",
			bankSum, expectedSum, diff)
	}

	return &ChargeValidation{
		Valid:          false,
		BankChargesSum: bankSum,
		ExpectedSum:    expectedSum,
		Difference:     diff,
		Reason:         reason,
	}
}

// ValidateChargesSimple is a convenience function for orders without non-bank payments.
// It validates that bank charges sum to the order total.
func ValidateChargesSimple(bankCharges []float64, orderTotal float64) *ChargeValidation {
	return ValidateCharges(bankCharges, orderTotal, 0)
}

// roundToCents rounds a float to 2 decimal places.
func roundToCents(amount float64) float64 {
	return math.Round(amount*100) / 100
}
