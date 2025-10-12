# Codebase Audit - Code Smells & Architecture Issues

**Date:** 2025-10-10
**Auditor:** Claude Code + User Review

---

## Executive Summary

The codebase has **two parallel architectures** that never merged:

1. **Legacy approach**: Individual cmd files with duplicated logic (costco-sync, walmart-sync-with-db)
2. **Modern approach**: Provider abstraction with centralized processor (internal/providers, internal/processor)

**Critical Finding:** The working sync commands (costco-sync, walmart-sync-with-db) DO NOT use the modern architecture that exists in internal/.

---

## 🚩 Major Code Smells

### 1. **Duplicate Split Logic** (Severity: HIGH)
**Location:**
- `internal/splitter/splitter.go` (471 lines) - Walmart-specific
- `cmd/costco-sync/main.go` lines 420-560 (140+ lines) - Costco implementation

**Problem:**
- Same business operation (split transactions by category) implemented twice
- Different formatting: Costco shows all items, Walmart truncates at 4+
- Inconsistent notes format: Costco uses commas, Walmart uses "and X more"

**Impact:**
- User confusion (different output for same operation)
- Maintenance burden (fix bugs in two places)
- Testing complexity (need tests for both implementations)

---

### 2. **Unused Architecture** (Severity: CRITICAL)
**Location:**
- `internal/providers/` - Provider abstraction (EXISTS but unused by main commands)
- `internal/processor/processor.go` - Centralized processor (EXISTS but unused)
- `cmd/sync/main.go` - Generic sync command (EXISTS but not used)

**Problem:**
- Clean provider abstraction exists but `costco-sync` and `walmart-sync-with-db` bypass it
- They reimple ment everything: fetching, matching, splitting, applying

**Evidence:**
```bash
# Working commands (566+ lines each, duplicate logic)
cmd/costco-sync/main.go          # 566 lines - does everything inline
cmd/walmart-sync-with-db/main.go # 583 lines - does everything inline

# Unused modern architecture
internal/processor/processor.go   # 405 lines - processes all providers generically
internal/providers/costco/provider.go
internal/providers/walmart/provider.go
cmd/sync/main.go                  # Generic sync that uses processor
```

---

### 3. **Business Logic in CMD Layer** (Severity: HIGH)
**Locations:**
- `cmd/costco-sync/main.go:328-381` - findBestMatch (matching logic)
- `cmd/costco-sync/main.go:383-574` - createSplits (splitting logic)
- `cmd/walmart-sync-with-db/main.go:541-577` - findBestMatch
- `cmd/walmart-sync-with-db/main.go:500-546` - findBestMatchUsingLedger

**Problem:**
- Presentation layer (cmd) contains core business logic
- Impossible to reuse matching/splitting without running the whole command
- No unit tests for this logic (it's in main functions)

---

### 4. **Inconsistent Transaction Matching** (Severity: HIGH)
**Just Fixed Today** ✅

**What was wrong:**
- Walmart: 20% amount tolerance + $5 variance
- Costco: 10% amount tolerance OR $5 variance
- No deduplication - multiple orders could match same transaction

**What we fixed:**
- Both now use ±$0.01 exact matching
- Added transaction deduplication via `usedTransactionIDs` map
- Consistent behavior across both providers

**But:** This fix is only in the legacy cmd files, not in the modern processor!

---

### 5. **Splitter Package is Walmart-Specific** (Severity: MEDIUM)
**Location:** `internal/splitter/splitter.go`

**Problem:**
```go
import (
    walmart "github.com/eshaffer321/walmart-client"  // ❌ Concrete dependency
)

func (s *Splitter) SplitTransaction(ctx context.Context, order *walmart.Order, ...) {
    // Only works with Walmart orders
}
```

**Why it's wrong:**
- Named "splitter" but only works with Walmart
- Should accept `providers.Order` interface
- Costco can't use it, so they rewrote everything

---

### 6. **Too Many CMD Binaries** (Severity: LOW)
**Count:** 14 separate commands in cmd/

```
cmd/
├── api/                    # API server
├── audit-report/           # Generate reports
├── costco-example/         # Example/test
├── costco-sync/            # ⭐ Actual sync (has --dry-run flag)
├── enrich-history/         # One-off script
├── sync/                   # Generic sync (unused!)
├── test-costco/            # Test script
├── walmart-analyze/        # Analysis tool
├── walmart-debug/          # Debug tool
├── walmart-sync-with-db/   # ⭐ Actual sync (used)
├── walmart-test-ledger/    # Test script
├── dashboard/              # Separate binary?
└── (more)

Note: costco-dry-run was removed - use `costco-sync --dry-run` instead
```

**Problem:**
- Many one-off scripts that should be subcommands
- Confusion about which to use (sync? costco-sync? both?)
- Build complexity (14 binaries)

**Better:** Single binary with subcommands:
```bash
monarchmoney-sync sync --provider=costco
monarchmoney-sync sync --provider=walmart
monarchmoney-sync analyze walmart
monarchmoney-sync dashboard
```

---

## 📊 Architecture Diagram (Current State)

```
┌─────────────────────────────────────────┐
│          USED IN PRODUCTION             │
├─────────────────────────────────────────┤
│ cmd/costco-sync/main.go (566 lines)     │
│   ├─ Fetch orders (inline)              │
│   ├─ Fetch transactions (inline)        │
│   ├─ Match (inline, 566 lines)          │
│   ├─ Split (inline, 140 lines)          │
│   └─ Apply (inline)                     │
│                                          │
│ cmd/walmart-sync-with-db/main.go        │
│   ├─ Fetch orders (inline)              │
│   ├─ Fetch transactions (inline)        │
│   ├─ Match (inline)                     │
│   ├─ Split (calls internal/splitter)    │
│   └─ Apply (inline)                     │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│         EXISTS BUT NOT USED             │
├─────────────────────────────────────────┤
│ internal/processor/processor.go         │
│   └─ Uses ↓                             │
│                                          │
│ internal/providers/                     │
│   ├─ registry.go (provider management)  │
│   ├─ costco/provider.go                 │
│   ├─ walmart/provider.go                │
│   └─ types.go (Order interface)         │
│                                          │
│ cmd/sync/main.go (203 lines)            │
│   └─ Uses processor (proper!)           │
└─────────────────────────────────────────┘
```

---

## 🎯 Why This Happened (Root Cause Analysis)

### Timeline (hypothesized from code):
1. **Phase 1:** Built walmart-specific solution with `internal/splitter`
2. **Phase 2:** Added Costco, copy-pasted logic into `cmd/costco-sync`
3. **Phase 3:** Realized need for abstraction, built `internal/providers` + `internal/processor`
4. **Phase 4:** Never migrated working commands to use new architecture
5. **Result:** Two parallel systems, old one still in use

### Why old system still used:
- **It works** - "if it ain't broke, don't fix it"
- **Migration risk** - switching could break production syncs
- **Time pressure** - easier to patch old code than refactor
- **Incomplete abstraction** - splitter still Walmart-specific, blocking full migration

---

## 📏 Code Metrics

### File Sizes (concern threshold: >300 lines)
```
583 lines - cmd/walmart-sync-with-db/main.go  ❌ Too big for cmd
566 lines - cmd/costco-sync/main.go           ❌ Too big for cmd
557 lines - internal/observability/tracer.go  ⚠️  Borderline
471 lines - internal/splitter/splitter.go     ⚠️  Borderline
405 lines - internal/processor/processor.go   ✅ Reasonable
```

### Duplication Assessment
- **Transaction Matching:** 2 implementations (cmd/costco-sync, cmd/walmart-sync-with-db)
- **Split Creation:** 2 implementations (internal/splitter, cmd/costco-sync)
- **Order Fetching:** 2 implementations (inline in each cmd, vs provider interface)
- **Database Interaction:** Duplicated storage calls across cmd files

### Test Coverage Gaps
- `cmd/costco-sync/main.go` - 0% (no tests for 566 lines)
- `cmd/walmart-sync-with-db/main.go` - 0% (no tests for 583 lines)
- `internal/splitter/splitter.go` - Unknown (need to check)
- `internal/processor/processor.go` - Unknown

---

## 🔥 Technical Debt Summary

| Issue | Severity | Impact | Effort to Fix |
|-------|----------|--------|---------------|
| Duplicate split logic | HIGH | User confusion, maintenance burden | MEDIUM |
| Unused modern architecture | CRITICAL | Wasted effort, continued duplication | HIGH |
| Business logic in CMD | HIGH | Untestable, unreusable | MEDIUM |
| Walmart-specific splitter | MEDIUM | Blocks provider unification | LOW |
| Too many binaries | LOW | Build complexity | LOW |
| Inconsistent notes format | MEDIUM | User experience | LOW |

**Total Estimated Effort:** 2-3 weeks for complete refactor

---

## ✅ What's Actually Good

1. **Provider abstraction exists** - Well-designed interfaces in internal/providers/
2. **Processor pattern** - Clean separation in internal/processor/
3. **Storage layer** - Good database abstraction
4. **Categorizer** - Seems well-designed and tested
5. **Recent matching fix** - Now using strict ±$0.01 matching (safe)

---

## Next: Refactoring Plan

See `REFACTORING_PLAN.md` for detailed migration strategy.
