package matcher

import (
	"fmt"
	"math"
	"time"

	"github.com/eshaffer321/itemize/internal/adapters/providers"
	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
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
	return m.FindMultipleMatchesWithDates(order, transactions, usedTransactionIDs, amounts, nil)
}

// FindMultipleMatchesWithDates is FindMultipleMatches with an optional charge
// date per amount (chargeDates[i] pairs with amounts[i]; nil or a zero time
// means unknown). Multi-shipment orders are charged as each shipment ships,
// which can be more than DateTolerance days after the order date, so a
// transaction is eligible when it falls within DateTolerance of either the
// order date or that charge's own date. A known charge date is also used for
// scoring.
func (m *Matcher) FindMultipleMatchesWithDates(
	order providers.Order,
	transactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
	amounts []float64,
	chargeDates []time.Time,
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
		var chargeDate time.Time
		if i < len(chargeDates) {
			chargeDate = chargeDates[i]
		}

		match := m.findBestMatchForAmount(
			amount,
			orderDate,
			chargeDate,
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

	// Validate sum of matched transactions equals sum of requested amounts
	// Note: We validate against requested amounts, NOT order.GetTotal(), because
	// orders may be partially paid with gift cards which don't appear in bank statements
	if result.AllFound {
		if err := m.validateMultiMatchSum(result); err != nil {
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
	chargeDate time.Time,
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

		// Calculate date difference (in days). Eligible if near the order date
		// or near this charge's own date; score by the charge date when known.
		tolerance := float64(m.config.DateTolerance)
		dateDiff := math.Abs(tx.Date.Time.Sub(orderDate).Hours() / 24)
		if !chargeDate.IsZero() {
			chargeDiff := math.Abs(tx.Date.Time.Sub(chargeDate).Hours() / 24)
			if dateDiff > tolerance && chargeDiff > tolerance {
				continue
			}
			dateDiff = chargeDiff
		} else if dateDiff > tolerance {
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

// validateMultiMatchSum ensures matched transactions sum to requested amounts
// Uses tolerance-based comparison to handle floating-point arithmetic
// Note: We validate against sum of requested amounts (result.Amounts), NOT order total,
// because orders may be partially paid with gift cards which don't appear in bank statements
func (m *Matcher) validateMultiMatchSum(result *MultiMatchResult) error {
	if len(result.Matches) == 0 {
		return fmt.Errorf("no matches to validate")
	}

	// Sum the matched transaction amounts
	matchedSum := 0.0
	for i, match := range result.Matches {
		if match == nil {
			return fmt.Errorf("cannot validate sum with nil match at index %d", i)
		}
		matchedSum += math.Abs(match.Transaction.Amount)
	}

	// Sum the requested amounts (what we were looking for)
	requestedSum := 0.0
	for _, amount := range result.Amounts {
		requestedSum += amount
	}

	// Compare with tolerance
	diff := math.Abs(matchedSum - requestedSum)

	if diff > m.config.AmountTolerance {
		return fmt.Errorf(
			"matched sum $%.2f does not match requested amounts $%.2f (diff: $%.2f, tolerance: $%.2f)",
			matchedSum, requestedSum, diff, m.config.AmountTolerance,
		)
	}

	return nil
}
