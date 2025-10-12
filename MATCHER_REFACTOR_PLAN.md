# Matcher Refactor Plan - Extract Transaction Matching Logic

**Goal:** Extract duplicated transaction matching logic into a shared, tested component

**Duration:** 1-2 days
**Risk:** LOW (matching is isolated, easy to test)
**Value:** HIGH (194 lines deduped, consistent matching, easier to maintain)

---

## 🎯 Success Criteria

**Before:**
```
❌ 4 separate matching implementations (194 lines duplicated)
❌ Bug fixes need to be applied in multiple places
❌ Inconsistent behavior between providers
```

**After:**
```
✅ 1 shared matcher implementation in internal/matcher
✅ Both providers use identical matching logic
✅ >90% test coverage on matcher
✅ Bug fixes applied once, work everywhere
✅ Easy to enhance (fuzzy matching, ML, etc.) later
```

---

## 📋 Step-by-Step Plan (TDD Approach)

### Phase 1: Create Package Structure (30 min)

#### 1.1 Create directory and files
```bash
mkdir -p internal/matcher
touch internal/matcher/matcher.go
touch internal/matcher/matcher_test.go
touch internal/matcher/types.go
```

#### 1.2 Create basic types (types.go)
```go
package matcher

import (
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
)

// Config holds matcher configuration
type Config struct {
	AmountTolerance float64 // Default: 0.01 (1 cent)
	DateTolerance   int     // Days tolerance (default: 5)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		AmountTolerance: 0.01,
		DateTolerance:   5,
	}
}

// MatchResult contains match information
type MatchResult struct {
	Transaction *monarch.Transaction
	DateDiff    float64 // Days difference
	AmountDiff  float64 // Absolute amount difference
	Confidence  float64 // 0-1 score (for future use)
}
```

---

### Phase 2: Write Tests First (TDD) (2 hours)

#### 2.1 Test file structure (matcher_test.go)

```go
package matcher

import (
	"testing"
	"time"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock order for testing
type mockOrder struct {
	id     string
	date   time.Time
	total  float64
}

func (m *mockOrder) GetID() string           { return m.id }
func (m *mockOrder) GetDate() time.Time      { return m.date }
func (m *mockOrder) GetTotal() float64       { return m.total }
func (m *mockOrder) GetSubtotal() float64    { return m.total * 0.9 }
func (m *mockOrder) GetTax() float64         { return m.total * 0.1 }
func (m *mockOrder) GetTip() float64         { return 0 }
func (m *mockOrder) GetFees() float64        { return 0 }
func (m *mockOrder) GetItems() []providers.OrderItem { return nil }
func (m *mockOrder) GetProviderName() string { return "test" }
func (m *mockOrder) GetRawData() interface{} { return nil }

// Helper to create test transaction
func makeTransaction(id string, amount float64, date time.Time) *monarch.Transaction {
	return &monarch.Transaction{
		ID:     id,
		Amount: amount,
		Date:   monarch.Date{Time: date},
	}
}

// Test cases to write:
```

#### 2.2 Write test cases (these should FAIL initially)

```go
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
```

#### 2.3 Run tests (they should FAIL)
```bash
go test ./internal/matcher -v
# Expected: All tests fail because implementation doesn't exist yet
```

---

### Phase 3: Implement Matcher (2-3 hours)

#### 3.1 Basic structure (matcher.go)

```go
package matcher

import (
	"math"

	"github.com/eshaffer321/monarchmoney-go/pkg/monarch"
	"github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
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

		// Calculate amount difference
		// Monarch transactions are negative for expenses, orders are positive
		txAmount := math.Abs(tx.Amount)
		amountDiff := math.Abs(orderAmount - txAmount)

		// Require exact match within tolerance
		if amountDiff > m.config.AmountTolerance {
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
```

#### 3.2 Run tests (they should PASS)
```bash
go test ./internal/matcher -v
# Expected: All tests pass ✅
```

#### 3.3 Check coverage
```bash
go test ./internal/matcher -cover
# Goal: >90% coverage
```

---

### Phase 4: Add Special Cases (1-2 hours)

#### 4.1 Add Costco-specific return handling

**Test case:**
```go
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
```

**Implementation update:**
```go
func (m *Matcher) FindMatch(...) (*MatchResult, error) {
	orderAmount := order.GetTotal()
	orderDate := order.GetDate()

	// Handle returns (negative amounts)
	isReturn := orderAmount < 0
	if isReturn {
		orderAmount = -orderAmount // Make positive for comparison
	}

	for _, tx := range transactions {
		// ... existing checks ...

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

		amountDiff := math.Abs(orderAmount - txAmount)
		// ... rest of logic ...
	}
}
```

#### 4.2 Add Walmart ledger support (optional for now)

Create a separate function for ledger-based matching:
```go
// FindMatchUsingLedger attempts to match using order ledger data
// This is Walmart-specific and optional
func (m *Matcher) FindMatchUsingLedger(
	order providers.Order,
	ledgerCharges []float64, // Actual charges from ledger
	transactions []*monarch.Transaction,
	usedTransactionIDs map[string]bool,
) (*MatchResult, error) {
	// Implementation similar to FindMatch but uses ledgerCharges instead of order.GetTotal()
	// This handles cases where Walmart splits an order into multiple charges
}
```

**Note:** We can add this later. For now, keep it simple.

---

### Phase 5: Integration (2-3 hours)

#### 5.1 Update Walmart sync

**File:** `cmd/walmart-sync-with-db/main.go`

**Before (lines 548-577):**
```go
func findBestMatch(order *OrderInfo, transactions []*monarch.Transaction, usedTransactionIDs map[string]bool) *monarch.Transaction {
	const AmountTolerance = 0.01
	const DateToleranceDays = 3
	// ... 30 lines of logic ...
	return bestMatch
}
```

**After:**
```go
import "github.com/eshaffer321/monarchmoney-sync-backend/internal/matcher"

// Remove findBestMatch function entirely

// In main() or initializeClients():
matcherConfig := matcher.Config{
	AmountTolerance: 0.01,
	DateTolerance:   3, // Walmart uses 3 days
}
transactionMatcher := matcher.NewMatcher(matcherConfig)

// Where findBestMatch was called (line 158):
// OLD: match := findBestMatch(order, transactions, usedTransactionIDs)
// NEW:
matchResult, err := transactionMatcher.FindMatch(order, transactions, usedTransactionIDs)
if err != nil {
	fmt.Printf("  ❌ Matching error: %v\n", err)
	continue
}
var match *monarch.Transaction
if matchResult != nil {
	match = matchResult.Transaction
}
```

**Need to adapt OrderInfo to providers.Order:**
```go
// Add at top of file
type OrderInfoAdapter struct {
	*OrderInfo
}

func (o *OrderInfoAdapter) GetItems() []providers.OrderItem { return nil }
func (o *OrderInfoAdapter) GetProviderName() string { return "walmart" }
func (o *OrderInfoAdapter) GetSubtotal() float64 { return o.TotalAmount * 0.9 }
func (o *OrderInfoAdapter) GetTax() float64 { return o.TotalAmount * 0.1 }
func (o *OrderInfoAdapter) GetTip() float64 { return 0 }
func (o *OrderInfoAdapter) GetFees() float64 { return 0 }
func (o *OrderInfoAdapter) GetRawData() interface{} { return o }
func (o *OrderInfoAdapter) GetID() string { return o.OrderID }
func (o *OrderInfoAdapter) GetDate() time.Time { return o.OrderDate }
func (o *OrderInfoAdapter) GetTotal() float64 { return o.TotalAmount }

// Then use:
adapter := &OrderInfoAdapter{OrderInfo: order}
matchResult, err := transactionMatcher.FindMatch(adapter, transactions, usedTransactionIDs)
```

#### 5.2 Update Costco sync

**File:** `cmd/costco-sync/main.go`

**Before (lines 328-381):**
```go
func findBestMatch(order providers.Order, transactions []*monarch.Transaction, usedTransactionIDs map[string]bool) (*monarch.Transaction, float64) {
	// ... 54 lines ...
}
```

**After:**
```go
import "github.com/eshaffer321/monarchmoney-sync-backend/internal/matcher"

// Remove findBestMatch function entirely

// In main():
matcherConfig := matcher.Config{
	AmountTolerance: 0.01,
	DateTolerance:   5, // Costco uses 5 days
}
transactionMatcher := matcher.NewMatcher(matcherConfig)

// Where findBestMatch was called (line 193):
// OLD: match, daysDiff := findBestMatch(order, costcoTransactions, usedTransactionIDs)
// NEW:
matchResult, err := transactionMatcher.FindMatch(order, costcoTransactions, usedTransactionIDs)
if err != nil {
	fmt.Printf("   ❌ Matching error: %v\n", err)
	continue
}
var match *monarch.Transaction
var daysDiff float64
if matchResult != nil {
	match = matchResult.Transaction
	daysDiff = matchResult.DateDiff
}
```

#### 5.3 Update Costco dry-run

**File:** `cmd/costco-dry-run/main.go`

Same changes as costco-sync above.

---

### Phase 6: Testing & Validation (1-2 hours)

#### 6.1 Unit tests pass
```bash
go test ./internal/matcher -v -cover
# Expected: >90% coverage, all tests pass
```

#### 6.2 Build all binaries
```bash
go build -o walmart-sync-with-db ./cmd/walmart-sync-with-db/
go build -o costco-sync ./cmd/costco-sync/
go build -o costco-dry-run ./cmd/costco-dry-run/
# Expected: All compile successfully
```

#### 6.3 Run walmart sync (dry-run)
```bash
./walmart-sync-with-db
# Expected: Same results as before (5 matched, 2 errors)
```

#### 6.4 Run costco sync (dry-run)
```bash
./costco-sync -dry-run
# Expected: Same results as before (1-3 orders matched)
```

#### 6.5 Compare results

Create a simple test:
```bash
# Before refactor (save output)
git stash
./walmart-sync-with-db > before.txt

# After refactor
git stash pop
go build -o walmart-sync-with-db ./cmd/walmart-sync-with-db/
./walmart-sync-with-db > after.txt

# Compare
diff before.txt after.txt
# Expected: No differences (or only timing/formatting differences)
```

---

### Phase 7: Cleanup & Documentation (30 min)

#### 7.1 Delete old code

**Delete from walmart-sync-with-db/main.go:**
- `findBestMatch()` function (lines 548-577)
- `findBestMatchUsingLedger()` function (lines 500-546) - **Keep for now**, will refactor later

**Delete from costco-sync/main.go:**
- `findBestMatch()` function (lines 328-381)

**Delete from costco-dry-run/main.go:**
- `findBestMatch()` function (lines 267-329)

#### 7.2 Update comments

Add to top of cmd files:
```go
// Transaction matching is handled by internal/matcher package
```

#### 7.3 Update README
```markdown
## Internal Packages

- `internal/matcher` - Transaction matching logic (shared by all providers)
- `internal/categorizer` - OpenAI item categorization (shared)
- `internal/splitter` - Transaction splitting (currently Walmart-specific, being refactored)
- `internal/storage` - Database operations (shared)
```

#### 7.4 Add package documentation

```go
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
```

---

### Phase 8: Commit (15 min)

```bash
git add internal/matcher/
git add cmd/walmart-sync-with-db/main.go
git add cmd/costco-sync/main.go
git add cmd/costco-dry-run/main.go

git commit -m "$(cat <<'EOF'
refactor: Extract transaction matching to shared matcher package

Changes:
- Created internal/matcher package with generic matching logic
- Removed 194 lines of duplicated matching code from cmd files
- Added comprehensive test suite (>90% coverage)
- Both Walmart and Costco now use identical matching logic

Benefits:
- Single source of truth for matching decisions
- Bug fixes applied once, work everywhere
- Easy to enhance matching algorithm in future
- Testable in isolation

Testing:
- All matcher unit tests pass
- Walmart sync produces identical results
- Costco sync produces identical results

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## 🎯 Success Metrics

### Quantitative
- ✅ 194 lines of code removed (duplicated matching logic)
- ✅ 1 new package created (internal/matcher)
- ✅ >90% test coverage on matcher
- ✅ 0 lines of matching logic in cmd files
- ✅ 100% reuse of matching logic between providers

### Qualitative
- ✅ Matching behavior is consistent
- ✅ Bug fixes are easier (one place to change)
- ✅ Adding new providers is easier (just use matcher)
- ✅ Code is testable in isolation
- ✅ Future enhancements are centralized

---

## ⚠️ Risks & Mitigation

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| Break existing matching | LOW | HIGH | Thorough testing, compare before/after |
| New bugs introduced | LOW | MEDIUM | Comprehensive unit tests, dry-run testing |
| Performance regression | VERY LOW | LOW | Matching is O(n), same as before |
| Provider-specific edge cases | MEDIUM | MEDIUM | Keep adapter pattern, test returns |

---

## 🚀 Optional Enhancements (Future)

Once basic matcher is working, we can add:

### 1. Fuzzy Matching (v2)
```go
type MatchStrategy int

const (
	StrategyExact MatchStrategy = iota
	StrategyFuzzy
	StrategyLedger
)

config.Strategy = StrategyFuzzy
config.FuzzyThreshold = 0.95
```

### 2. Machine Learning (v3)
```go
// Learn from user corrections
matcher.LearnFromFeedback(order, transaction, wasCorrect bool)
```

### 3. Multiple Match Handling (v2)
```go
// Return all candidates, let caller decide
results, err := matcher.FindAllMatches(order, transactions, usedIDs)
```

### 4. Ledger Integration (v2)
```go
// Move Walmart ledger logic into matcher
result, err := matcher.FindMatchWithLedger(order, ledger, transactions, usedIDs)
```

---

## 📋 Checklist

### Phase 1: Setup
- [ ] Create `internal/matcher` directory
- [ ] Create `matcher.go`, `matcher_test.go`, `types.go`
- [ ] Define Config, MatchResult types

### Phase 2: Tests (TDD)
- [ ] Write TestMatcher_ExactMatch
- [ ] Write TestMatcher_WithinOneCent
- [ ] Write TestMatcher_MoreThanOneCent_NoMatch
- [ ] Write TestMatcher_DateTolerance
- [ ] Write TestMatcher_BeyondDateTolerance_NoMatch
- [ ] Write TestMatcher_AlreadyUsed_Skipped
- [ ] Write TestMatcher_MultipleMatches_PicksClosestDate
- [ ] Write TestMatcher_NegativeAmounts
- [ ] Write TestMatcher_EmptyTransactions
- [ ] Write TestMatcher_CustomConfig
- [ ] Run tests (expect failures)

### Phase 3: Implementation
- [ ] Implement NewMatcher()
- [ ] Implement FindMatch() basic logic
- [ ] Run tests (expect passes)
- [ ] Check coverage (>90%)

### Phase 4: Special Cases
- [ ] Add Costco returns handling
- [ ] Test returns handling
- [ ] (Optional) Add ledger support

### Phase 5: Integration
- [ ] Update Walmart sync to use matcher
- [ ] Update Costco sync to use matcher
- [ ] Update Costco dry-run to use matcher
- [ ] Build all binaries (verify compilation)

### Phase 6: Testing
- [ ] Run Walmart sync dry-run
- [ ] Run Costco sync dry-run
- [ ] Compare before/after results
- [ ] Verify identical behavior

### Phase 7: Cleanup
- [ ] Delete old findBestMatch functions
- [ ] Update comments
- [ ] Add package documentation
- [ ] Update README

### Phase 8: Commit
- [ ] Stage changes
- [ ] Write descriptive commit message
- [ ] Commit and push

---

## 🎓 Lessons Applied

This refactoring follows the **categorizer pattern** which works perfectly:

| Aspect | Categorizer (model) | Matcher (this refactor) |
|--------|-------------------|----------------------|
| Generic types | ✅ Item, Category | ✅ Order interface |
| Clear interface | ✅ OpenAIClient | ✅ providers.Order |
| Dependency injection | ✅ Pass client, cache | ✅ Pass config |
| Well-tested | ✅ 447 lines tests | ✅ >90% coverage goal |
| No provider-specific | ✅ Works for all | ✅ Works for all |
| Both use identically | ✅ Yes | ✅ Yes |

---

**Ready to start? Let's begin with Phase 1! 🚀**
