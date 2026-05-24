# Bug Fix Log

## Format
Each bug fix entry should include:
- Date discovered
- Bug description
- Test case that reproduces the bug
- Fix applied
- Commit reference

---

## Bug Fixes

### 2026-05-24: Costco discount applied by description instead of item number

**Description:**
Some Costco receipt line items reference their parent via description (e.g. `/AAA BATTERY`) rather than item number (e.g. `/1234567`). The discount lookup only checked the item-number map, so these discounts were logged as "orphaned" and silently dropped, leaving the item price too high.

**Test Case:**
```go
// provider_test.go: "discount references parent by description instead of item number"
// Discount item has ItemDescription01 = "/AAA BATTERY"; parent has ItemNumber "379938"
// Expected: discount of -$2.50 applied → price $12.49
// Actual before fix: price remained $14.99, WARN logged
```

**Root Cause:**
`convertReceipt` built a single `itemMap` keyed by `ItemNumber`. `GetParentItemNumber()` strips the leading `/`, returning `"AAA BATTERY"` — a string that never matches a numeric key.

**Fix Applied:**
Added a parallel `descMap` keyed by uppercased `ItemDescription01`. When the item-number lookup misses, fall back to the description map before declaring the discount orphaned.

**Verification:**
- New test `discount references parent by description instead of item number` passes.
- All existing discount-netting tests continue to pass.
- `go test ./... -race` green.

---

### 2025-09-01: No bugs discovered yet
The project was developed using strict TDD methodology from the start, preventing bugs through test-first development.

---

## Template for Future Bug Fixes

### Date: YYYY-MM-DD: Bug Title

**Description:**
Brief description of the bug and its impact.

**Test Case:**
```go
func TestBugScenario(t *testing.T) {
    // Test that reproduces the bug
}
```

**Root Cause:**
Explanation of why the bug occurred.

**Fix Applied:**
```go
// Code changes that fixed the bug
```

**Verification:**
- Test now passes
- No regression in other tests
- Manual testing confirmed

**Commit:** `commit-hash`

---

## Bug Prevention Strategies

1. **Always write tests first** (TDD)
2. **Test edge cases and error conditions**
3. **Use static analysis tools**
4. **Code reviews before merging**
5. **Integration testing for complex flows**
6. **Monitor production for unexpected errors**

## Common Bug Categories to Watch

### Input Validation
- Missing validation
- Incorrect validation rules
- Type conversion errors

### Concurrency
- Race conditions
- Deadlocks
- Incorrect mutex usage

### Error Handling
- Unhandled errors
- Incorrect error types
- Missing error context

### Integration
- API contract mismatches
- Timeout issues
- Retry logic failures

### Performance
- Memory leaks
- Inefficient queries
- N+1 problems