// Package matcher provides transaction matching logic for syncing
// provider orders with Monarch Money transactions.
//
// The matcher uses strict matching criteria:
//   - Amount must match within 1 cent (configurable)
//   - Date must be within tolerance (default 5 days)
//   - Transaction must not be already used
//
// Example usage:
//
//	config := matcher.DefaultConfig()
//	m := matcher.NewMatcher(config)
//	result, err := m.FindMatch(order, transactions, usedIDs)
//	if result != nil {
//		// Found a match!
//		transaction := result.Transaction
//	}
package matcher

import (
	"math"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
)

// Matcher matches orders with Monarch transactions
type Matcher struct {
	config Config
}

// NewMatcher creates a new matcher with the given config
func NewMatcher(config Config) *Matcher {
	return &Matcher{
		config: config,
	}
}

// FindMatch finds the best matching transaction for an order
// Returns nil if no suitable match found
func (m *Matcher) FindMatch(
	order providers.Order,
	transactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
) (*MatchResult, error) {

	var bestMatch *monarch.Transaction
	var bestScore float64 = 999999 // Lower is better

	orderAmount := order.GetTotal()
	orderDate := order.GetDate()

	// Handle returns (negative amounts)
	isReturn := orderAmount < 0
	if isReturn {
		orderAmount = -orderAmount // Make positive for comparison
	}

	for _, tx := range transactions {
		// Skip if already used
		if usedTransactionIDs[tx.ID] {
			continue
		}

		// Calculate date difference (in days)
		dateDiff := math.Abs(tx.Date.Time.Sub(orderDate).Hours() / 24)
		if dateDiff > float64(m.config.DateTolerance) {
			continue
		}

		// For returns, match positive Monarch transactions
		// For purchases, match negative Monarch transactions
		txAmount := tx.Amount
		if isReturn {
			if txAmount < 0 {
				continue // Skip negative transactions for returns
			}
		} else {
			if txAmount > 0 {
				continue // Skip positive transactions for purchases
			}
			txAmount = -txAmount // Make positive for comparison
		}

		// Calculate amount difference
		amountDiff := math.Abs(orderAmount - txAmount)

		// Require exact match within tolerance
		// Add small epsilon to handle floating point precision issues
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
		return nil, nil
	}

	// Calculate final result
	result := &MatchResult{
		Transaction: bestMatch,
		DateDiff:    bestScore,
		AmountDiff:  math.Abs(orderAmount - math.Abs(bestMatch.Amount)),
		Confidence:  1.0, // For now, all matches are high confidence
	}

	return result, nil
}
