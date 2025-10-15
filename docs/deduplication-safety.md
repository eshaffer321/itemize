# Deduplication & Database Safety

## Overview
This document explains how the sync system prevents duplicate processing and protects your Monarch Money budget from corruption.

## Two-Layer Protection System

The system has **TWO independent layers** of deduplication protection:

### Layer 1: Database Tracking (Primary)
**Location:** `internal/storage/storage.go:305-309`

```go
func (s *Storage) IsProcessed(orderID string) bool {
    query := `SELECT 1 FROM processing_records WHERE order_id = ? AND status = 'success'`
    return err == nil
}
```

**What it does:**
- Tracks successfully processed orders in SQLite database
- Checks `order_id` + `status='success'` before processing
- Can be bypassed with `--force` flag

**Used at:** `internal/sync/orchestrator.go:67`
```go
if !opts.Force && o.storage != nil && o.storage.IsProcessed(order.GetID()) {
    o.logger.Info("Skipping already processed order", "order_id", order.GetID())
    return false, true, nil // Skipped
}
```

### Layer 2: Monarch Transaction State (Safety Net)
**Location:** `internal/sync/orchestrator.go:114-119`

```go
// Check if already has splits
if match.HasSplits {
    o.logger.Info("Transaction already has splits", "transaction_id", match.ID)
    return false, true, nil // Skipped
}
```

**What it does:**
- Checks if the matched Monarch transaction already has splits applied
- This check happens **even if database tracking is missing**
- Cannot be bypassed (built-in safety)

## What Happens If You Nuke the Database?

### Scenario: Delete `monarch_sync.db` and Re-run Sync

**Step-by-step trace:**

1. **Fetch Orders**
   - System fetches last 14 days of Walmart/Costco orders
   - Includes previously processed orders

2. **Database Check (Layer 1) - FAILS**
   ```go
   o.storage.IsProcessed(order.GetID()) // Returns FALSE (no DB record)
   ```
   - ❌ Layer 1 protection is gone
   - System proceeds to next step

3. **Match Transaction**
   - Matches order to Monarch transaction by date/amount
   - Finds the same transaction that was split before

4. **HasSplits Check (Layer 2) - SUCCEEDS** ✅
   ```go
   if match.HasSplits {  // Returns TRUE (transaction already split)
       return false, true, nil // SKIPPED
   }
   ```
   - ✅ Layer 2 protection kicks in
   - **Transaction is skipped**
   - **No duplicate splits created**
   - **Budget stays safe**

### Result: Your Budget is Safe

Even with a nuked database, **Layer 2 protects you** because:
- Monarch API returns `HasSplits: true` for already-split transactions
- System checks this **before** attempting to create splits
- Already-split transactions are automatically skipped

## Status Tracking in Database

Only `success` status prevents reprocessing:

| Status | Count | Will Reprocess? | Why? |
|--------|-------|----------------|------|
| `success` | 15 | ❌ No | Already successfully processed |
| `dry-run` | 16 | ✅ Yes | Was simulation only, not applied |
| `failed` | 15 | ✅ Yes | Failed, can retry |
| `no-match` | 1 | ✅ Yes | No transaction found, can retry |

**Why this design?**
- Allows retrying failed orders
- Dry-run mode doesn't mark as "processed"
- Only actual successful splits are protected

## Force Flag Behavior

The `--force` flag **bypasses Layer 1 only**:

```bash
./monarch-sync walmart sync --force
```

**What it does:**
- ✅ Bypasses database `IsProcessed()` check (Layer 1)
- ❌ Does NOT bypass `HasSplits` check (Layer 2)

**Use case:**
- Retry an order that's marked "success" but you want to reprocess
- Layer 2 still prevents duplicate splits on already-split transactions

## Migration to V2: Provider-Aware Deduplication

### Current Problem (V1)
```sql
-- V1 checks only order_id
SELECT 1 FROM processing_records WHERE order_id = ? AND status = 'success'
```

**Issue:** If Walmart order `123` and Costco order `123` both exist, can't distinguish them.

### Solution (V2)
```sql
-- V2 checks order_id + provider
SELECT 1 FROM processing_records_v2
WHERE order_id = ? AND provider = ? AND status = 'success'
```

**Benefits:**
- ✅ Tracks Walmart and Costco separately
- ✅ Same order ID across providers won't conflict
- ✅ More accurate deduplication

### Migration Strategy

**Existing 47 records will be migrated as:**
```sql
INSERT INTO processing_records_v2 (..., provider, ...)
SELECT ..., 'unknown', ...
FROM processing_records
```

**Why `provider='unknown'`?**
- Honest: we don't know if they were Walmart or Costco
- Safe: won't accidentally skip future orders
- Fixable: can manually update if needed

**After migration, dedupe check becomes:**
```go
// V1 (current)
o.storage.IsProcessed(order.GetID())

// V2 (provider-aware)
o.storage.IsProcessed(order.GetID(), order.GetProviderName())
```

### Handling Migrated Records

**Scenario:** Migrated record has `provider='unknown'` and `status='success'`

1. Run Walmart sync with order ID `123`
2. `IsProcessed('123', 'walmart')` checks: `order_id='123' AND provider='walmart'`
3. Finds **NO MATCH** (migrated record has `provider='unknown'`)
4. Proceeds to process order
5. **Layer 2 protects:** If transaction already split, skipped ✅

**If you want to preserve Layer 1 protection for migrated records:**
```sql
-- Update migrated records to specific provider
UPDATE processing_records_v2
SET provider = 'walmart'
WHERE provider = 'unknown' AND status = 'success';
```

## Safety Recommendations

### ✅ Safe Operations
- Delete database and re-run sync (Layer 2 protects)
- Use `--force` flag (Layer 2 still protects)
- Migrate to V2 with `provider='unknown'` (Layer 2 protects)
- Re-process `dry-run` or `failed` orders

### ⚠️ Caution Required
- Manually deleting splits in Monarch, then re-running sync
  - Layer 2 won't detect (no splits = looks processable)
  - Layer 1 will catch if DB record exists
  - Workaround: Delete DB record for that order first

### 🚫 Dangerous (Don't Do)
- Modifying `HasSplits` check in code
- Bypassing both Layer 1 and Layer 2
- Manually changing `status='success'` to `status='failed'` while splits still exist in Monarch

## Testing Deduplication

### Test 1: Database Protection
```bash
# Process an order
./monarch-sync walmart sync --dry-run=false

# Try again (should skip)
./monarch-sync walmart sync --dry-run=false
# Output: "Skipping already processed order"
```

### Test 2: Monarch State Protection
```bash
# Nuke database
rm monarch_sync.db

# Re-run sync
./monarch-sync walmart sync --dry-run=false
# Output: "Transaction already has splits" (Layer 2 kicks in)
```

### Test 3: Force Flag
```bash
# Force reprocess
./monarch-sync walmart sync --force
# Output: "Transaction already has splits" (Layer 2 still protects)
```

## Summary

**Key Takeaway:** Your budget is protected by **two independent layers**:

1. **Database tracking** - Fast, efficient, can be reset
2. **Monarch state check** - Ultimate safety net, can't be bypassed

**Even in worst case (nuked database + force flag):**
- Monarch's `HasSplits` flag prevents duplicate splits
- Your budget stays safe
- System logs skipped transactions

**For V2 migration:**
- Use `provider='unknown'` for migrated records
- Layer 2 protection ensures safety during transition
- Optionally update provider manually after migration

---

## Refactoring History: Single-Category Fix (2024-10-13)

### Problem Fixed
**OLD behavior (WRONG):**
- Single-category orders created 2 fake splits (including 1¢ "Rounding adjustment")
- Lines 379-434 in old orchestrator.go contained the hack
- Every grocery-only order would get: Main split + 1¢ tax split (or fake adjustment)

**NEW behavior (CORRECT):**
- Single-category orders: Use `Transactions.Update()` to set category + notes (NO splits)
- Multi-category orders: Use `Transactions.UpdateSplits()` to create proper splits
- No more fake splits or 1¢ hacks

### Implementation Details
Created new **Splitter package** (`internal/splitter/`):
- `CreateSplits()`: Returns `nil` for single category, splits for multi-category
- `GetSingleCategoryInfo()`: Extracts category/notes using cached categorization
- **Caching optimization**: Avoids duplicate AI calls when processing single-category orders

**Orchestrator changes** (`internal/sync/orchestrator.go:121-206`):
- Handles both single and multi-category paths
- Removed 164-line `createSplits()` method
- Added nil check for test compatibility

### Test Coverage Status

#### ✅ Completed Tests (Splitter Package)
- `TestSplitter_SingleCategory_ShouldNotSplit` - Returns nil for single category
- `TestSplitter_TwoCategories` - Basic 2-way split
- `TestSplitter_ThreeCategories` - 3+ categories with tax distribution
- `TestSplitter_NegativeAmount` - Returns/refunds (positive amounts)
- `TestSplitter_ItemDetailsInNotes` - Quantity formatting (x2, x3)
- `TestSplitter_CachedCategorization` - Verifies caching works
- `TestSplitter_RoundingAdjustment` - Splits sum to transaction amount
- `TestSplitter_RoundingAdjustment_LargeDiscrepancy` - Handles awkward tax rates

**Coverage:** 75% (internal/splitter)

#### ⚠️ Skipped Tests (Orchestrator Integration)

The following tests in `internal/sync/orchestrator_test.go` need updating:

1. **`TestOrchestrator_processOrder`** (line 19)
   - Status: SKIP
   - Reason: "Requires mocking Monarch client and categorizer"
   - **Needs:** Mock splitter + Monarch client, test both Update() and UpdateSplits() paths

2. **`TestOrchestrator_MatchesTransactions`** (line 294)
   - Status: SKIP
   - **Needs:** Test transaction matching with ±1¢ and ±3 days tolerance

3. **`TestOrchestrator_CreatesCorrectSplits`** (line 306)
   - Status: SKIP
   - **Needs:** End-to-end split creation test for both single/multi-category

4. **`TestOrchestrator_HandlesAlreadyProcessed`** (line 317)
   - Status: SKIP
   - **Needs:** Test idempotency with storage and HasSplits checks

5. **`TestOrchestrator_MaxOrders`** (line 327)
   - Status: SKIP
   - **Needs:** Test MaxOrders limit enforcement

6. **`TestOrchestrator_Integration`** (line 336)
   - Status: SKIP
   - **Needs:** Full end-to-end test (consider VCR/replay for API calls)

### Manual Testing Checklist
Before marking refactoring complete, manually test:
- [ ] Single-category Walmart order (should Update, not split)
- [ ] Multi-category Walmart order (should create 2+ splits)
- [ ] Multi-category Costco order (3+ categories)
- [ ] Return/refund (positive amount)
- [ ] Order with quantity > 1 items (verify notes formatting)
- [ ] Dry-run mode
- [ ] Re-run on already processed order (verify idempotency)

### Future Enhancements
- Add "repair" mode to consolidate old fake splits
- Track split creation method in storage (v1 hack vs v2 proper)
- Add metrics: single-category % vs multi-category %
