// Package matcher provides transaction matching logic for syncing
// provider orders with Monarch transactions.
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
	"errors"
	"math"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
)

// ErrAmbiguousMatch indicates that two or more transactions are equally good
// matches and choosing one would require guessing.
var ErrAmbiguousMatch = errors.New("ambiguous transaction match")

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

// FindUniqueMatch returns a match only when one candidate is strictly better
// than every other eligible transaction by amount difference, then date.
func (m *Matcher) FindUniqueMatch(
	order providers.Order,
	transactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
) (*MatchResult, error) {
	orderAmount := order.GetTotal()
	orderDate := order.GetDate()
	isReturn := orderAmount < 0
	orderAmount = math.Abs(orderAmount)

	var best *monarch.Transaction
	bestAmountDiff := math.Inf(1)
	bestDateDiff := math.Inf(1)
	bestCount := 0
	const epsilon = 0.0000001

	for _, tx := range transactions {
		if tx == nil || usedTransactionIDs[tx.ID] {
			continue
		}
		if isReturn && tx.Amount < 0 {
			continue
		}
		if !isReturn && tx.Amount > 0 {
			continue
		}

		amountDiff := math.Abs(orderAmount - math.Abs(tx.Amount))
		if amountDiff > m.config.AmountTolerance+epsilon {
			continue
		}
		dateDiff := math.Abs(tx.Date.Time.Sub(orderDate).Hours() / 24)
		if dateDiff > float64(m.config.DateTolerance) {
			continue
		}

		betterAmount := amountDiff < bestAmountDiff-epsilon
		equalAmount := math.Abs(amountDiff-bestAmountDiff) <= epsilon
		betterDate := dateDiff < bestDateDiff-epsilon
		equalDate := math.Abs(dateDiff-bestDateDiff) <= epsilon
		switch {
		case betterAmount || (equalAmount && betterDate):
			best = tx
			bestAmountDiff = amountDiff
			bestDateDiff = dateDiff
			bestCount = 1
		case equalAmount && equalDate:
			bestCount++
		}
	}

	if best == nil {
		return nil, nil
	}
	if bestCount > 1 {
		return nil, ErrAmbiguousMatch
	}
	return &MatchResult{
		Transaction: best,
		DateDiff:    bestDateDiff,
		AmountDiff:  bestAmountDiff,
		Confidence:  1.0,
	}, nil
}
