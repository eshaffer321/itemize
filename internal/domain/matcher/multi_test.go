package matcher

import (
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMatcher_FindMultipleMatches tests multi-transaction matching
func TestMatcher_FindMultipleMatches(t *testing.T) {
	config := Config{
		AmountTolerance: 0.01,
		DateTolerance:   5,
	}
	matcher := NewMatcher(config)

	baseDate := time.Date(2025, 10, 16, 0, 0, 0, 0, time.UTC)

	t.Run("finds two matching transactions", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER123",
			date:  baseDate,
			total: 126.98,
		}

		transactions := []*monarch.Transaction{
			{
				ID:     "txn1",
				Amount: -118.67,
				Date:   monarch.Date{Time: baseDate},
			},
			{
				ID:     "txn2",
				Amount: -8.31,
				Date:   monarch.Date{Time: baseDate.AddDate(0, 0, 1)}, // Next day
			},
			{
				ID:     "txn3",
				Amount: -50.00, // Different amount
				Date:   monarch.Date{Time: baseDate},
			},
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{118.67, 8.31}

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.True(t, result.AllFound, "Should find all matches")
		assert.Len(t, result.Matches, 2, "Should have 2 matches")
		assert.Equal(t, "txn1", result.Matches[0].Transaction.ID)
		assert.Equal(t, "txn2", result.Matches[1].Transaction.ID)
		assert.Equal(t, amounts, result.Amounts)
	})

	t.Run("finds three matching transactions", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER456",
			date:  baseDate,
			total: 100.00,
		}

		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -50.00, Date: monarch.Date{Time: baseDate}},
			{ID: "txn2", Amount: -30.00, Date: monarch.Date{Time: baseDate}},
			{ID: "txn3", Amount: -20.00, Date: monarch.Date{Time: baseDate}},
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{50.00, 30.00, 20.00}

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.True(t, result.AllFound)
		assert.Len(t, result.Matches, 3)
	})

	t.Run("partial match - only finds some transactions", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER789",
			date:  baseDate,
			total: 126.98,
		}

		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.67, Date: monarch.Date{Time: baseDate}},
			// Missing txn for $8.31
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{118.67, 8.31}

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.False(t, result.AllFound, "Should not find all matches")
		assert.Len(t, result.Matches, 2, "Should still return 2 slots")
		assert.NotNil(t, result.Matches[0], "First match should be found")
		assert.Nil(t, result.Matches[1], "Second match should be nil")
	})

	t.Run("prevents duplicate matches within operation", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER999",
			date:  baseDate,
			total: 200.00,
		}

		// Two charges of same amount, but only one matching transaction
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -100.00, Date: monarch.Date{Time: baseDate}},
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{100.00, 100.00} // Same amount twice

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.False(t, result.AllFound, "Should not match same txn twice")
		assert.Len(t, result.Matches, 2)
		// First charge gets the match
		assert.NotNil(t, result.Matches[0])
		assert.Equal(t, "txn1", result.Matches[0].Transaction.ID)
		// Second charge doesn't (already matched)
		assert.Nil(t, result.Matches[1])
	})

	t.Run("respects globally used transaction IDs", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER111",
			date:  baseDate,
			total: 100.00,
		}

		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -100.00, Date: monarch.Date{Time: baseDate}},
		}

		usedIDs := map[string]bool{
			"txn1": true, // Already used
		}
		amounts := []float64{100.00}

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.False(t, result.AllFound)
		assert.Nil(t, result.Matches[0], "Should skip used transaction")
	})

	t.Run("matches within amount tolerance", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER222",
			date:  baseDate,
			total: 126.98,
		}

		// Slightly off amounts (within tolerance)
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.68, Date: monarch.Date{Time: baseDate}}, // Off by 1 cent
			{ID: "txn2", Amount: -8.30, Date: monarch.Date{Time: baseDate}},   // Off by 1 cent
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{118.67, 8.31}

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.True(t, result.AllFound, "Should match within tolerance")
		assert.Len(t, result.Matches, 2)
	})

	t.Run("matches within date tolerance", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER333",
			date:  baseDate,
			total: 126.98,
		}

		// Transactions on different dates
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.67, Date: monarch.Date{Time: baseDate.AddDate(0, 0, -3)}}, // 3 days before
			{ID: "txn2", Amount: -8.31, Date: monarch.Date{Time: baseDate.AddDate(0, 0, 4)}},    // 4 days after
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{118.67, 8.31}

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.True(t, result.AllFound, "Should match within 5 day tolerance")
	})

	t.Run("skips transactions outside date tolerance", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER444",
			date:  baseDate,
			total: 118.67,
		}

		// Transaction 6 days away (outside 5 day tolerance)
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.67, Date: monarch.Date{Time: baseDate.AddDate(0, 0, 6)}},
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{118.67}

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.False(t, result.AllFound, "Should skip transactions outside date tolerance")
		assert.Nil(t, result.Matches[0])
	})

	t.Run("prefers closest date match", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER555",
			date:  baseDate,
			total: 100.00,
		}

		// Multiple transactions with same amount, different dates
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -100.00, Date: monarch.Date{Time: baseDate.AddDate(0, 0, -3)}}, // 3 days before
			{ID: "txn2", Amount: -100.00, Date: monarch.Date{Time: baseDate}},                    // Same day
			{ID: "txn3", Amount: -100.00, Date: monarch.Date{Time: baseDate.AddDate(0, 0, 2)}},  // 2 days after
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{100.00}

		result, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.NoError(t, err)

		assert.True(t, result.AllFound)
		assert.Equal(t, "txn2", result.Matches[0].Transaction.ID, "Should pick same-day transaction")
		assert.Equal(t, 0.0, result.Matches[0].DateDiff, "Date diff should be 0")
	})

	t.Run("error - no amounts provided", func(t *testing.T) {
		order := &mockOrder{id: "ORDER", date: baseDate, total: 100}
		transactions := []*monarch.Transaction{}
		usedIDs := make(map[string]bool)
		amounts := []float64{}

		_, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no amounts provided")
	})

	t.Run("error - negative amount", func(t *testing.T) {
		order := &mockOrder{id: "ORDER", date: baseDate, total: 100}
		transactions := []*monarch.Transaction{}
		usedIDs := make(map[string]bool)
		amounts := []float64{100.00, -50.00}

		_, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid amount at index 1")
	})

	t.Run("error - zero amount", func(t *testing.T) {
		order := &mockOrder{id: "ORDER", date: baseDate, total: 100}
		transactions := []*monarch.Transaction{}
		usedIDs := make(map[string]bool)
		amounts := []float64{100.00, 0.00}

		_, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid amount at index 1")
	})

	t.Run("error - sum mismatch", func(t *testing.T) {
		order := &mockOrder{
			id:    "ORDER666",
			date:  baseDate,
			total: 126.98,
		}

		// Charges don't sum to order total
		transactions := []*monarch.Transaction{
			{ID: "txn1", Amount: -118.00, Date: monarch.Date{Time: baseDate}},
			{ID: "txn2", Amount: -8.00, Date: monarch.Date{Time: baseDate}},
		}

		usedIDs := make(map[string]bool)
		amounts := []float64{118.00, 8.00} // Sum = 126.00, not 126.98

		_, err := matcher.FindMultipleMatches(order, transactions, usedIDs, amounts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "charge sum")
		assert.Contains(t, err.Error(), "does not match order total")
	})
}

// TestMatcher_validateMultiMatchSum tests sum validation logic
func TestMatcher_validateMultiMatchSum(t *testing.T) {
	config := DefaultConfig()
	matcher := NewMatcher(config)

	t.Run("valid sum within tolerance", func(t *testing.T) {
		result := &MultiMatchResult{
			Matches: []*MatchResult{
				{Transaction: &monarch.Transaction{Amount: -118.67}},
				{Transaction: &monarch.Transaction{Amount: -8.31}},
			},
		}
		orderTotal := 126.98

		err := matcher.validateMultiMatchSum(result, orderTotal)
		assert.NoError(t, err)
	})

	t.Run("floating-point safe validation", func(t *testing.T) {
		// Amounts that might not sum exactly due to floating-point
		result := &MultiMatchResult{
			Matches: []*MatchResult{
				{Transaction: &monarch.Transaction{Amount: -0.1}},
				{Transaction: &monarch.Transaction{Amount: -0.2}},
			},
		}
		orderTotal := 0.3

		err := matcher.validateMultiMatchSum(result, orderTotal)
		assert.NoError(t, err, "Should handle floating-point arithmetic")
	})

	t.Run("error - sum exceeds tolerance", func(t *testing.T) {
		result := &MultiMatchResult{
			Matches: []*MatchResult{
				{Transaction: &monarch.Transaction{Amount: -118.00}},
				{Transaction: &monarch.Transaction{Amount: -8.00}},
			},
		}
		orderTotal := 126.98 // Diff = 0.98 (exceeds 0.01 tolerance)

		err := matcher.validateMultiMatchSum(result, orderTotal)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "charge sum")
	})

	t.Run("error - no matches", func(t *testing.T) {
		result := &MultiMatchResult{
			Matches: []*MatchResult{},
		}

		err := matcher.validateMultiMatchSum(result, 100.00)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no matches to validate")
	})

	t.Run("error - nil match in result", func(t *testing.T) {
		result := &MultiMatchResult{
			Matches: []*MatchResult{
				{Transaction: &monarch.Transaction{Amount: -100.00}},
				nil, // Partial match
			},
		}

		err := matcher.validateMultiMatchSum(result, 100.00)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil match at index 1")
	})
}
