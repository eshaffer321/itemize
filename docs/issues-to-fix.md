# Code Audit Report - Monarch Money Sync Backend

## 📊 Executive Summary

The codebase has evolved from a single-purpose Walmart sync tool to a multi-provider capable system with observability. The refactoring is partially complete with good foundations but some areas need consolidation.

## 🏗️ Current Architecture

### ✅ What's Working Well

1. **Core Functionality**
   - Transaction matching and splitting works reliably
   - AI categorization with OpenAI is accurate
   - Tax distribution logic is solid
   - Delivery tip handling as separate split
   - Dashboard provides good visibility

2. **Data Layer**
   - SQLite storage tracks processing history
   - Duplicate prevention works
   - Processing records are persisted

3. **Observability**
   - Enhanced dashboard with drill-down capability
   - Trace system infrastructure in place
   - Clickable cards to detail views
   - API endpoints for data access

### ⚠️ Areas of Concern

1. **Duplicate Code**
   - Two processor packages: `/processor` and `/internal/processor`
   - Two dashboard implementations: `/cmd/dashboard` and `/cmd/dashboard-v2`
   - Provider abstraction started but not integrated

2. **Incomplete Refactoring**
   - Provider interfaces created but sync tool still uses direct Walmart client
   - Trace system built but not integrated with sync tool
   - Storage v2 created but not used

3. **Module Organization**
   - Mix of flat structure (`/categorizer`, `/splitter`) and internal (`/internal/`)
   - Inconsistent package naming

## 📁 File-by-File Analysis

### Production Code (Keep)

| File | Purpose | Status | Action |
|------|---------|--------|--------|
| `cmd/walmart-sync-with-db/main.go` | Main sync tool | ✅ Working | Needs refactor to use providers |
| `cmd/dashboard-v2/main.go` | Enhanced dashboard | ✅ Working | Keep, delete v1 |
| `categorizer/categorizer.go` | AI categorization | ✅ Working | Keep as-is |
| `splitter/splitter.go` | Transaction splitting | ✅ Working | Keep as-is |
| `storage/storage.go` | Database layer | ✅ Working | Migrate to v2 |
| `storage/storage_v2.go` | Enhanced storage | 🆕 New | Integrate with sync |

### New Provider Abstraction (Partially Complete)

| File | Purpose | Status | Action |
|------|---------|--------|--------|
| `internal/providers/types.go` | Provider interfaces | ✅ Good | Keep |
| `internal/providers/registry.go` | Provider management | ✅ Good | Keep |
| `internal/providers/walmart/provider.go` | Walmart adapter | ✅ Good | Integrate |
| `internal/processor/processor.go` | Provider-agnostic processor | 🆕 New | Integrate |
| `internal/config/config.go` | Configuration system | 🆕 New | Use it |

### Observability

| File | Purpose | Status | Action |
|------|---------|--------|--------|
| `internal/observability/tracer.go` | Trace system | ✅ Good | Integrate with sync |
| `internal/observability/logger.go` | Structured logging | ✅ Good | Use throughout |

### Duplicate/Obsolete

| File | Purpose | Status | Action |
|------|---------|--------|--------|
| `processor/processor.go` | Old processor | ⚠️ Partial use | Remove, use internal/processor |
| `cmd/dashboard/main.go` | Old dashboard | 📦 Obsolete | Delete |
| `cmd/enrich-history/main.go` | History enrichment | 🚧 Incomplete | Fix or remove |

## 🔍 Code Quality Issues

### 1. **Error Handling**
```go
// Current - swallowing errors
record, err := store.GetRecord(orderID)
if err != nil {
    // Just continues
}

// Should be
if err != nil {
    logger.Error("failed to get record", slog.String("error", err.Error()))
    return fmt.Errorf("get record: %w", err)
}
```

### 2. **Magic Numbers**
```go
// Found in multiple places
if math.Abs(amountDiff) <= 0.50 { // Should be const
time.Sleep(2 * time.Second) // Should be configurable
```

### 3. **Missing Context Usage**
Many functions don't accept context for cancellation:
```go
// Current
func ProcessOrder(order Order) error

// Should be
func ProcessOrder(ctx context.Context, order Order) error
```

### 4. **Incomplete Provider Integration**
The sync tool still directly uses Walmart client instead of provider abstraction:
```go
// Current in walmart-sync-with-db
clients.walmart.GetOrderSummaries()

// Should be
provider.FetchOrders(ctx, opts)
```

## 📈 Metrics

- **Total Go Files**: 20
- **Lines of Code**: ~4,500 (production)
- **Test Coverage**: ~30% (needs improvement)
- **Packages**: 11
- **External Dependencies**: 5 main ones

## 🎯 Recommendations

### Immediate Actions (Priority 1)

1. **Complete Provider Integration**
   ```go
   // Update sync tool to use provider registry
   registry := providers.NewRegistry(logger)
   registry.Register(walmart.NewProvider(client, logger))
   processor := processor.New(registry, monarch, categorizer, splitter, storage, logger)
   ```

2. **Consolidate Processors**
   - Delete `/processor` package
   - Use `/internal/processor` everywhere
   - Update imports

3. **Remove Old Dashboard**
   ```bash
   rm -rf cmd/dashboard
   ```

4. **Integrate Tracing**
   - Add trace creation to sync tool
   - Record steps during processing
   - Visible in dashboard

### Short Term (Priority 2)

1. **Migrate to Storage V2**
   - Update sync tool to save detailed splits
   - Capture item-level data
   - Store tax/tip breakdown

2. **Use Configuration System**
   - Load from `config.yaml`
   - Remove hardcoded values
   - Environment variable overrides

3. **Implement Structured Logging**
   - Replace all `fmt.Printf` with `logger.Info`
   - Add context fields
   - Use log levels appropriately

### Medium Term (Priority 3)

1. **Add Tests**
   - Unit tests for critical paths
   - Integration tests for providers
   - Mock external dependencies

2. **Error Recovery**
   - Retry logic for transient failures
   - Better error messages
   - Graceful degradation

3. **Performance Optimization**
   - Concurrent order processing
   - Batch API calls where possible
   - Cache optimization

## 🧹 Cleanup Commands

```bash
# Remove duplicate/old code
rm -rf cmd/dashboard
rm -rf processor  # After migrating to internal/processor

# Update imports
find . -name "*.go" -exec sed -i '' 's|monarchmoney-sync-backend/processor|monarchmoney-sync-backend/internal/processor|g' {} \;

# Run formatter
go fmt ./...

# Update dependencies
go mod tidy
```

## 📊 Health Score

| Category | Score | Notes |
|----------|-------|-------|
| **Functionality** | 9/10 | Core features work well |
| **Architecture** | 6/10 | Partially refactored, needs completion |
| **Code Quality** | 7/10 | Generally clean, some duplication |
| **Testing** | 3/10 | Minimal test coverage |
| **Documentation** | 8/10 | Good docs, well commented |
| **Observability** | 7/10 | Dashboard good, tracing not integrated |
| **Maintainability** | 6/10 | Provider abstraction will help |

**Overall: 6.6/10** - Functional but needs refactoring completion

## ✅ What's Great

1. **Business Logic**: Categorization, splitting, and matching work reliably
2. **User Experience**: Dashboard provides excellent visibility
3. **Extensibility**: Provider abstraction sets good foundation
4. **Documentation**: Well-documented architecture and plans

## 🚨 Critical Issues

1. **Incomplete Refactoring**: Half-migrated to provider pattern
2. **No Tracing Integration**: Built but not used
3. **Storage V2 Unused**: Enhanced schema not utilized
4. **Test Coverage**: Very low, risky for production

## 🎯 Next Steps Priority

1. **Today**: 
   - Complete provider integration in sync tool
   - Remove duplicate processor package
   - Delete old dashboard

2. **This Week**:
   - Integrate tracing system
   - Migrate to storage v2
   - Add structured logging

3. **This Month**:
   - Add comprehensive tests
   - Implement configuration system
   - Add Costco provider when SDK ready

## 💡 Architecture Recommendations

### Suggested Final Structure
```
cmd/
  sync/                 # Universal sync tool (rename from walmart-sync-with-db)
  dashboard/            # Keep only v2
  
internal/
  providers/            # All provider implementations
  processor/            # Core processing logic
  categorizer/          # Move here from root
  splitter/            # Move here from root
  storage/             # Move here from root
  observability/       # Logging, metrics, tracing
  config/              # Configuration management

pkg/
  monarch/             # Monarch client wrapper (if needed)

test/
  integration/         # Integration tests
  fixtures/           # Test data
```

## 🏁 Conclusion

The codebase is **functionally solid** but **architecturally incomplete**. The provider abstraction and observability improvements are well-designed but need to be fully integrated. With ~2-3 days of focused refactoring, this could be a very clean, extensible system ready for multiple providers.

**Recommendation**: Complete the provider integration first, then gradually improve testing and observability. The business logic is proven and working - focus on the architectural cleanup.