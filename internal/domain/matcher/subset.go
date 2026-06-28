package matcher

import (
	"fmt"
	"math"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/itemize/internal/adapters/providers"
)

// subsetDateTolerance is wider than the normal match window because multi-shipment
// charges can post several days after the order date.
const subsetDateTolerance = 10

// FindSubsetByTotal finds a subset of Monarch transactions whose absolute amounts
// sum to the order total. Used as a fallback when the Amazon scraper cannot
// discover all bank charges from the order's transaction page (e.g. when
// subsequent shipment charges post after the scraper visited the order page).
//
// Only negative (purchase) transactions are considered; refunds/credits are
// excluded. Returns the matched transactions or an error if no valid subset
// is found.
func (m *Matcher) FindSubsetByTotal(
	order providers.Order,
	monarchTxns []*monarch.Transaction,
	usedTxnIDs map[string]bool,
) ([]*monarch.Transaction, error) {
	target := order.GetTotal()
	if target <= 0 {
		return nil, fmt.Errorf("order total must be positive")
	}

	orderDate := order.GetDate()

	// Collect purchase candidates within the date window
	var candidates []*monarch.Transaction
	for _, txn := range monarchTxns {
		if usedTxnIDs[txn.ID] {
			continue
		}
		if txn.Amount >= 0 { // skip refunds/credits
			continue
		}
		days := math.Abs(txn.Date.Time.Sub(orderDate).Hours() / 24)
		if days > subsetDateTolerance {
			continue
		}
		candidates = append(candidates, txn)
	}

	// Brute-force subset search — n is always small (typically 1–5)
	matches := subsetSummingTo(candidates, target, m.config.AmountTolerance)
	if matches == nil {
		return nil, fmt.Errorf("no combination of Monarch transactions sums to order total $%.2f", target)
	}
	return matches, nil
}

// subsetSummingTo returns the smallest subset of txns whose absolute amounts
// sum to target within tolerance, or nil if none exists.
func subsetSummingTo(txns []*monarch.Transaction, target, tolerance float64) []*monarch.Transaction {
	n := len(txns)
	if n > 20 {
		n = 20 // guard; 2^20 is ~1M — still fast, but cap for safety
	}

	// Try subsets in increasing size order so we prefer fewer transactions
	for size := 1; size <= n; size++ {
		result := subsetOfSize(txns[:n], target, tolerance, 0, size, nil)
		if result != nil {
			return result
		}
	}
	return nil
}

// subsetOfSize is a recursive backtracking search for a subset of exactly `size`
// transactions summing to target.
func subsetOfSize(
	txns []*monarch.Transaction,
	target, tolerance float64,
	start, remaining int,
	current []*monarch.Transaction,
) []*monarch.Transaction {
	if remaining == 0 {
		sum := 0.0
		for _, t := range current {
			sum += math.Abs(t.Amount)
		}
		if math.Abs(sum-target) <= tolerance {
			result := make([]*monarch.Transaction, len(current))
			copy(result, current)
			return result
		}
		return nil
	}

	for i := start; i <= len(txns)-remaining; i++ {
		found := subsetOfSize(txns, target, tolerance, i+1, remaining-1,
			append(current, txns[i]))
		if found != nil {
			return found
		}
	}
	return nil
}
