package matcher

import (
	"testing"

	"github.com/eshaffer321/monarch-go/v2/pkg/monarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubsetSummingTo_PrefersExactTotalOverSmallerToleratedSubset(t *testing.T) {
	transactions := []*monarch.Transaction{
		{ID: "near", Amount: -9.99},
		{ID: "penny", Amount: -0.01},
	}

	matches := subsetSummingTo(transactions, 10.00, 0.01)

	require.Len(t, matches, 2)
	assert.Equal(t, []string{"near", "penny"}, []string{matches[0].ID, matches[1].ID})
}
