package matcher

import (
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/adapters/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock order for testing
type mockOrder struct {
	id    string
	date  time.Time
	total float64
}

func (m *mockOrder) GetID() string                   { return m.id }
func (m *mockOrder) GetDate() time.Time              { return m.date }
func (m *mockOrder) GetTotal() float64               { return m.total }
func (m *mockOrder) GetSubtotal() float64            { return m.total * 0.9 }
func (m *mockOrder) GetTax() float64                 { return m.total * 0.1 }
func (m *mockOrder) GetTip() float64                 { return 0 }
func (m *mockOrder) GetFees() float64                { return 0 }
func (m *mockOrder) GetItems() []providers.OrderItem { return nil }
func (m *mockOrder) GetProviderName() string         { return "test" }
func (m *mockOrder) GetRawData() interface{}         { return nil }

// Helper to create test transaction
func makeTransaction(id string, amount float64, date time.Time) *monarch.Transaction {
	return &monarch.Transaction{
		ID:     id,
		Amount: amount,
		Date:   monarch.Date{Time: date},
	}
}

func TestMatcher_ExactMatch(t *testing.T) {
	// Arrange
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.00, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)),
		makeTransaction("tx2", -150.00, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)),
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "tx1", result.Transaction.ID)
	assert.Equal(t, 0.0, result.DateDiff)
	assert.InDelta(t, 0.0, result.AmountDiff, 0.001)
}

func TestMatcher_WithinOneCent(t *testing.T) {
	// Arrange
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.01, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)),
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should match (within 1 cent)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "tx1", result.Transaction.ID)
}

func TestMatcher_MoreThanOneCent_NoMatch(t *testing.T) {
	// Arrange
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.02, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)),
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should NOT match (more than 1 cent)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMatcher_DateTolerance(t *testing.T) {
	// Arrange
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	// Test within tolerance (5 days)
	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.00, time.Date(2025, 10, 13, 0, 0, 0, 0, time.UTC)), // +3 days
		makeTransaction("tx2", -100.00, time.Date(2025, 10, 7, 0, 0, 0, 0, time.UTC)),  // -3 days
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should match one (picks closest)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Transaction.ID == "tx1" || result.Transaction.ID == "tx2")
}

func TestMatcher_BeyondDateTolerance_NoMatch(t *testing.T) {
	// Arrange
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	// 6 days away (beyond 5 day tolerance)
	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.00, time.Date(2025, 10, 16, 0, 0, 0, 0, time.UTC)),
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should NOT match
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMatcher_AlreadyUsed_Skipped(t *testing.T) {
	// Arrange
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.00, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)),
	}

	usedIDs := map[string]bool{
		"tx1": true, // Already used
	}

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should NOT match (already used)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMatcher_MultipleMatches_PicksClosestDate(t *testing.T) {
	// Arrange
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.00, time.Date(2025, 10, 13, 0, 0, 0, 0, time.UTC)), // +3 days
		makeTransaction("tx2", -100.00, time.Date(2025, 10, 11, 0, 0, 0, 0, time.UTC)), // +1 day (closest)
		makeTransaction("tx3", -100.00, time.Date(2025, 10, 8, 0, 0, 0, 0, time.UTC)),  // -2 days
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should pick tx2 (closest date)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "tx2", result.Transaction.ID)
	assert.InDelta(t, 1.0, result.DateDiff, 0.1)
}

func TestMatcher_NegativeAmounts(t *testing.T) {
	// Test that both order and transaction amounts are handled correctly
	// Monarch transactions are negative for expenses
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00, // Order shows positive
	}

	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.00, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)), // Transaction is negative
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should match
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "tx1", result.Transaction.ID)
}

func TestMatcher_EmptyTransactions(t *testing.T) {
	// Arrange
	matcher := NewMatcher(DefaultConfig())
	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	transactions := []*monarch.Transaction{} // Empty
	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMatcher_CustomConfig(t *testing.T) {
	// Arrange - Custom config with tighter tolerance
	config := Config{
		AmountTolerance: 0.005, // Half a cent
		DateTolerance:   2,     // Only 2 days
	}
	matcher := NewMatcher(config)

	order := &mockOrder{
		id:    "order1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: 100.00,
	}

	// This would match with default config but not custom
	transactions := []*monarch.Transaction{
		makeTransaction("tx1", -100.01, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)), // More than 0.005
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should NOT match with tighter tolerance
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMatcher_CostcoReturns(t *testing.T) {
	// Costco returns show as negative amounts
	// Monarch returns show as positive amounts
	matcher := NewMatcher(DefaultConfig())

	order := &mockOrder{
		id:    "return1",
		date:  time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC),
		total: -50.00, // Negative = return
	}

	transactions := []*monarch.Transaction{
		makeTransaction("tx1", 50.00, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)), // Positive
	}

	usedIDs := make(map[string]bool)

	// Act
	result, err := matcher.FindMatch(order, transactions, usedIDs)

	// Assert - Should match (flipped signs)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "tx1", result.Transaction.ID)
}
