package matcher

import (
	"fmt"
	"math"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
)

// MultiMatchResult contains results from multi-transaction matching
type MultiMatchResult struct {
	Matches  []*MatchResult // One per charge amount (same order as amounts)
	Amounts  []float64      // The amounts that were matched
	AllFound bool           // True if found match for every amount
}

// FindMultipleMatches finds transactions matching each charge amount
// Returns matches for each amount in the same order as amounts slice
// Prevents duplicate matches within the same operation
func (m *Matcher) FindMultipleMatches(
	order providers.Order,
	transactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
	amounts []float64,
) (*MultiMatchResult, error) {
	if len(amounts) == 0 {
		return nil, fmt.Errorf("no amounts provided")
	}

	result := &MultiMatchResult{
		Matches:  make([]*MatchResult, 0, len(amounts)),
		Amounts:  amounts,
		AllFound: true,
	}

	// Track matches in this operation to prevent duplicates
	matchedThisRound := make(map[string]bool)

	orderDate := order.GetDate()

	// Find best match for each charge amount
	for i, amount := range amounts {
		if amount <= 0 {
			return nil, fmt.Errorf("invalid amount at index %d: %.2f (must be positive)", i, amount)
		}

		// Find best matching transaction for this amount
		match := m.findBestMatchForAmount(
			amount,
			orderDate,
			transactions,
			usedTransactionIDs,
			matchedThisRound,
		)

		if match != nil {
			// Mark as matched in this round
			matchedThisRound[match.Transaction.ID] = true
			result.Matches = append(result.Matches, match)
		} else {
			// Didn't find match for this amount
			result.AllFound = false
			// Still append nil to maintain index alignment
			result.Matches = append(result.Matches, nil)
		}
	}

	// Validate sum matches order total (if all found)
	if result.AllFound {
		if err := m.validateMultiMatchSum(result, order.GetTotal()); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// findBestMatchForAmount finds the best matching transaction for a specific amount
// Reuses core matching logic from FindMatch but for a single amount
func (m *Matcher) findBestMatchForAmount(
	amount float64,
	orderDate time.Time,
	transactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
	matchedThisRound map[string]bool,
) *MatchResult {
	var bestMatch *monarch.Transaction
	var bestScore float64 = 999999 // Lower is better

	for _, tx := range transactions {
		// Skip if already used globally
		if usedTransactionIDs[tx.ID] {
			continue
		}

		// Skip if matched in this operation
		if matchedThisRound[tx.ID] {
			continue
		}

		// Calculate date difference (in days)
		dateDiff := math.Abs(tx.Date.Time.Sub(orderDate).Hours() / 24)
		if dateDiff > float64(m.config.DateTolerance) {
			continue
		}

		// Match against negative transactions (purchases)
		// Multi-delivery charges are always purchases (positive amounts split into negative transactions)
		if tx.Amount > 0 {
			continue // Skip positive transactions
		}

		txAmount := -tx.Amount // Make positive for comparison

		// Calculate amount difference
		amountDiff := math.Abs(amount - txAmount)

		// Require exact match within tolerance
		const epsilon = 0.0000001
		if amountDiff > m.config.AmountTolerance+epsilon {
			continue
		}

		// Score based on date closeness (amount is already exact)
		score := dateDiff

		if score < bestScore {
			bestMatch = tx
			bestScore = score
		}
	}

	if bestMatch == nil {
		return nil
	}

	// Calculate final result
	result := &MatchResult{
		Transaction: bestMatch,
		DateDiff:    bestScore,
		AmountDiff:  math.Abs(amount - math.Abs(bestMatch.Amount)),
		Confidence:  1.0,
	}

	return result
}

// validateMultiMatchSum ensures matched transactions sum to order total
// Uses tolerance-based comparison to handle floating-point arithmetic
func (m *Matcher) validateMultiMatchSum(result *MultiMatchResult, orderTotal float64) error {
	if len(result.Matches) == 0 {
		return fmt.Errorf("no matches to validate")
	}

	// Sum the matched transaction amounts
	sum := 0.0
	for i, match := range result.Matches {
		if match == nil {
			return fmt.Errorf("cannot validate sum with nil match at index %d", i)
		}
		sum += math.Abs(match.Transaction.Amount)
	}

	// Compare with tolerance
	orderAmount := math.Abs(orderTotal)
	diff := math.Abs(sum - orderAmount)

	if diff > m.config.AmountTolerance {
		return fmt.Errorf(
			"charge sum $%.2f does not match order total $%.2f (diff: $%.2f, tolerance: $%.2f)",
			sum, orderAmount, diff, m.config.AmountTolerance,
		)
	}

	return nil
}
