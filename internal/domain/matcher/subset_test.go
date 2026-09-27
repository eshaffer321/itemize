package matcher

import (
	"testing"
	"time"

	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubsetSummingTo_PrefersExactTotalOverSmallerToleratedSubset(t *testing.T) {
	transactions := []*monarch.Transaction{
		{ID: "near", Amount: -9.99},
		{ID: "penny", Amount: -0.01},
	}

	matches := subsetSummingTo(transactions, 10.00, 0.01, nil)

	require.Len(t, matches, 2)
	assert.Equal(t, []string{"near", "penny"}, []string{matches[0].ID, matches[1].ID})
}

// Regression: order 112-7815140-3755432 ($55.31 total, one $59.36 card charge).
// Unanchored discovery paired two unrelated charges from other orders
// ($25.44 + $29.87) because they happened to sum to the order total.
func TestFindSubsetByTotalIncluding_RejectsSubsetWithoutOrderCharge(t *testing.T) {
	orderDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	order := &mockOrder{id: "112-7815140-3755432", date: orderDate, total: 55.31}
	transactions := []*monarch.Transaction{
		{ID: "own-charge", Amount: -59.36, Date: monarch.Date{Time: orderDate}},
		{ID: "other-order-a", Amount: -25.44, Date: monarch.Date{Time: orderDate}},
		{ID: "other-order-b", Amount: -29.87, Date: monarch.Date{Time: orderDate}},
	}
	m := NewMatcher(DefaultConfig())

	matches, err := m.FindSubsetByTotalIncluding(order, transactions, map[string]bool{}, []float64{59.36})

	assert.Error(t, err)
	assert.Nil(t, matches)
}

func TestFindSubsetByTotalIncluding_AcceptsSubsetContainingOrderCharge(t *testing.T) {
	orderDate := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	order := &mockOrder{id: "multi-shipment", date: orderDate, total: 103.27}
	transactions := []*monarch.Transaction{
		{ID: "known", Amount: -52.55, Date: monarch.Date{Time: orderDate}},
		{ID: "late", Amount: -50.72, Date: monarch.Date{Time: orderDate.AddDate(0, 0, 3)}},
		{ID: "unrelated", Amount: -12.00, Date: monarch.Date{Time: orderDate}},
	}
	m := NewMatcher(DefaultConfig())

	matches, err := m.FindSubsetByTotalIncluding(order, transactions, map[string]bool{}, []float64{52.55})

	require.NoError(t, err)
	require.Len(t, matches, 2)
	assert.ElementsMatch(t, []string{"known", "late"}, []string{matches[0].ID, matches[1].ID})
}
