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

### 2026-07-01: Release binaries would panic on startup — cgo sqlite driver incompatible with cross-compiled builds

**Description:**
While building a GoReleaser release pipeline, discovered that setting `CGO_ENABLED=0` (required to cross-compile darwin/linux × amd64/arm64 binaries from a single build host) causes `mattn/go-sqlite3` to compile a no-op stub instead of failing the build. The resulting binary builds successfully but panics the instant it tries to open the database:
```
Failed to initialize storage: failed to enable foreign keys: Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work. This is a stub
```
Every release binary built this way would have been completely non-functional.

**Test Case:**
```bash
CGO_ENABLED=0 go build -o itemize-nocgo ./cmd/itemize/   # builds without error
./itemize-nocgo walmart -dry-run -days 1                  # panics on storage init
```

**Root Cause:**
`mattn/go-sqlite3` wraps the SQLite C library via cgo. When cgo is disabled it falls back to a stub implementation (guarded by Go build tags) that compiles but is non-functional at runtime. Since Go's `go build` doesn't fail on this, the bug is silent until the binary actually runs.

**Fix Applied:**
Replaced `github.com/mattn/go-sqlite3` with `modernc.org/sqlite`, a pure-Go SQLite implementation that requires no cgo. Changed the `database/sql` driver name from `"sqlite3"` to `"sqlite"` in `internal/infrastructure/storage/sqlite.go` and `migrations_test.go` (the goose dialect string stays `"sqlite3"` — that's the SQL-syntax dialect, unrelated to the driver name). This also surfaced a second, pre-existing bug: `modernc.org/sqlite` is stricter than `mattn/go-sqlite3` about scanning SQL `NULL` into a non-nullable Go `string` field — `SearchItems`'s `date(p.order_date)` could return `NULL` and was previously silently coerced to `""` by the old driver. Fixed by wrapping with `COALESCE(date(p.order_date), '')` in the query.

**Verification:**
- `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/itemize/` now cross-compiles cleanly.
- `go test ./... -race` passes, including the previously-failing `TestStorage_SearchItems` subtests.
- Manually reproduced the original panic against the old driver before the fix, confirmed it's gone after.

---

### 2026-05-31: Amazon charge validation incorrectly rejects orders with non-reducing non-bank entries

**Description:**
`itemize amazon -dry-run` emitted `WARN: Charge validation failed ... bank charges ($68.86) exceed expected ($61.87) by $6.99` and skipped the order. The $6.99 was a non-bank transaction entry (promotional credit / reward earned) that appears in Amazon's transaction list with no card digits but does NOT actually reduce the card charge. The validator subtracted it from the order total to compute `expectedSum`, making the real bank charge look like an overcharge.

**Test Case:**
```go
// validator/charges_test.go: TestValidateCharges_BankMatchesOrderTotalWithNonBankEntry
// bankCharges=[68.86], orderTotal=68.86, nonBankAmount=6.99
// Expected: valid (bank charge equals full order total)
// Actual before fix: invalid ("bank charges exceed expected by $6.99")
```

**Root Cause:**
`ValidateCharges` only checked `bankSum ≈ orderTotal - nonBankAmount`. When a non-bank entry doesn't reduce the card charge (e.g. rewards earned, promotional accounting), the formula underestimates what the bank should receive.

**Fix Applied:**
Added a secondary check in `ValidateCharges`: if `bankSum ≈ orderTotal` (within 2¢ tolerance), the validation passes regardless of the non-bank amount. The bank charged the full order total, which is a valid scenario.

**Verification:**
- `TestValidateCharges_BankMatchesOrderTotalWithNonBankEntry` now passes.
- All existing validator tests continue to pass.

---

### 2026-05-31: Amazon gift-card-only orders cause handler error instead of skip

**Description:**
`itemize amazon -dry-run` emitted `ERROR: Handler error order_id=112-4444156-8489869 error=failed to get bank charges: no bank charges found (order paid entirely with gift cards/points)`. Orders paid entirely with gift cards/points have no bank transaction in Monarch to match, so they should be silently skipped — not crash the handler.

**Test Case:**
```go
// handlers/amazon_test.go: TestAmazonHandler_ProcessOrder_FullyGiftCardOrder
// GetFinalCharges() returns ErrGiftCardOrder
// Expected: result.Skipped=true, no error returned
// Actual before fix: returned a fatal error, logged as ERROR
```

**Root Cause:**
`GetFinalCharges()` returned a plain `fmt.Errorf(...)` for this case instead of a sentinel error. The handler only checked `errors.Is(err, ErrPaymentPending)` and treated everything else as a fatal error.

**Fix Applied:**
Introduced `ErrGiftCardOrder` sentinel in `amazon/order.go` and updated `GetFinalCharges()` to return it. The handler now checks `errors.Is(err, ErrGiftCardOrder)` and skips the order with reason `"paid entirely with gift cards/points"`.

**Verification:**
- `TestAmazonHandler_ProcessOrder_FullyGiftCardOrder` now passes.
- `TestOrder_GetFinalCharges_OnlyGiftCard` updated to use `errors.Is(err, ErrGiftCardOrder)`.
- All existing tests continue to pass.

---

### 2026-05-31: Costco mixed-tender receipts matched against full receipt total

**Description:**
Costco in-warehouse receipts can be paid with multiple tenders, such as part Costco Visa and part cash/rebate. Itemize matched the full receipt total against Monarch, so mixed-tender receipts were skipped when Monarch only had the card-funded portion.

**Test Case:**
```go
// provider_test.go: TestConvertReceipt_MixedTenderUsesCardAmount
// Receipt total is $150.00, paid $100.00 by COSTCO VISA and $50.00 cash.
// Expected: order total is $100.00 and items/subtotal/tax are scaled to the card-paid portion.
// Actual before fix: order total remained $150.00, so no Monarch transaction matched.
```

**Root Cause:**
`convertReceipt` always used `receipt.Total` for `Order.GetTotal()`. For mixed tender purchases, the matchable Monarch transaction is the bank-card tender amount, not the full Costco receipt total.

**Fix Applied:**
Added Costco tender detection for bank-card payments, use the card tender total as the order total when present, and proportionally allocate item prices, subtotal, and tax to that card-paid amount.

**Verification:**
- New test `TestConvertReceipt_MixedTenderUsesCardAmount` fails before the fix and passes after.
- Existing Costco discount-netting tests pass.
- `go test ./...` passes.
- Temporary-db dry-run for receipt `21134300601232605091211` matched Monarch transaction `243571290918193554` at `$209.65`.

---

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
