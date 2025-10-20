# Logger Rollout Guide

This guide walks through adding Maven-style logging across all projects.

## ✅ Completed: monarchmoney-sync-backend

**Status:** Fully implemented

The main application now uses Maven-style logging:
```
[INFO] [sync] [20:40:56] Starting sync provider=Costco lookback_days=17
[INFO] [costco] [20:40:57] Fetching online orders start_date=2025-09-30
[WARN] [sync] [20:41:22] No matching transaction found order_id=21134300600532510011111
```

## 🔄 Next Steps: External Libraries

### 1. Update costco-go

**Files:** 3 files, ~61 log calls to replace

**Prompt:** `docs/costco-go-logger-injection.md`

**Quick Start:**
```bash
cd /Users/erickshaffer/code/costco-go
# Copy the prompt from docs/costco-go-logger-injection.md and paste to Claude/AI
# Or work through it manually following the guide
```

**Key Changes:**
- Add `logger *slog.Logger` to `Client` struct
- Update `NewClient(config, logger)` signature
- Replace all `log.*` calls in:
  - `pkg/costco/client.go` (~40 calls)
  - `pkg/costco/helpers.go` (~15 calls)
  - `pkg/costco/types.go` (~6 calls)

**Testing:**
```bash
# After changes
go test ./pkg/costco/... -v
go build ./cmd/costco-cli/
```

### 2. Update walmart-client

**Files:** 4 files, ~86 log calls to replace

**Prompt:** `docs/walmart-client-logger-injection.md`

**Quick Start:**
```bash
cd /Users/erickshaffer/code/walmart-api/walmart-client
# Copy the prompt from docs/walmart-client-logger-injection.md and paste to Claude/AI
# Or work through it manually following the guide
```

**Key Changes:**
- Add `logger *slog.Logger` to `WalmartClient` struct
- Update `NewWalmartClient(config, logger)` signature
- Replace all `log.*` calls in:
  - `client.go` (~30 calls)
  - `purchase_history.go` (~25 calls)
  - `orderledger.go` (~20 calls)
  - `example_json.go`, `test_tip.go` (~11 calls)

**Testing:**
```bash
# After changes
go test . -v
go build ./cmd/walmart/
```

### 3. Update monarchmoney-sync-backend Integration

**After both libraries are updated:**

Update the provider initialization to pass loggers:

**`internal/cli/providers.go`:**
```go
// Costco
costcoLogger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "costco")
costcoClient := costcogo.NewClient(costcoConfig, costcoLogger) // Pass logger!
return costco.NewProvider(costcoClient, costcoLogger), nil

// Walmart
walmartLogger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "walmart")
walmartClient, err := walmartclient.NewWalmartClient(walmartConfig, walmartLogger) // Pass logger!
return walmart.NewProvider(walmartClient, walmartLogger), nil
```

**Update go.mod dependencies:**
```bash
cd /Users/erickshaffer/code/monarchmoney-sync-backend
go get github.com/costco-go/pkg/costco@latest
go get github.com/eshaffer321/walmart-client@latest
go mod tidy
```

## Expected Final Output

Once all three projects are updated, you'll see consistent logging:

```
monarch-sync: Costco (PRODUCTION mode)
Database: monarch_sync.db
Provider: Costco | Lookback: 17 days

[INFO] [sync] [20:40:56] Starting sync provider=Costco lookback_days=17 dry_run=false
[INFO] [costco] [20:40:56] fetching online orders start_date=2025-09-30 end_date=2025-10-17
[INFO] [costco] [20:40:57] fetched online orders order_count=6
[INFO] [sync] [20:40:59] Processing order index=1 total=6
[INFO] [sync] [20:41:08] Multiple categories detected split_count=4
[INFO] [monarch] [20:41:08] Applying splits transaction_id=224736066360937319
[INFO] [sync] [20:41:08] Successfully applied splits order_id=21134300600592510141205
[WARN] [sync] [20:41:22] No matching transaction found order_id=21134300600532510011111
------------------------------------------------------------
Summary: Processed=4 Skipped=0 Errors=2

Sync completed successfully.
```

## Colors

When running in a terminal, logs are automatically colored:
- **[INFO]** in **cyan**
- **[WARN]** in **yellow**
- **[ERROR]** in **red**
- Timestamps in **gray**

Colors automatically disable when piping to files.

## Rollback Plan

If there are issues with the external libraries:

1. **Revert the integration changes** in `providers.go`:
   ```go
   // Pass nil for logger (backwards compatible)
   costcoClient := costcogo.NewClient(costcoConfig, nil)
   walmartClient := walmartclient.NewWalmartClient(walmartConfig, nil)
   ```

2. **Pin old versions** in go.mod if needed:
   ```bash
   go get github.com/costco-go/pkg/costco@<old-version>
   ```

## Testing Checklist

After updating each library:

- [ ] All existing tests pass
- [ ] Build succeeds without errors
- [ ] CLI tools work as expected
- [ ] Logger injection test added
- [ ] Nil logger doesn't crash
- [ ] README updated

After integrating in monarchmoney-sync-backend:

- [ ] Build succeeds
- [ ] `./monarch-sync costco -dry-run -days 7 -verbose` shows consistent logs
- [ ] `./monarch-sync walmart -dry-run -days 7 -verbose` shows consistent logs
- [ ] Colors appear in terminal
- [ ] No colors when piping: `./monarch-sync costco -verbose 2>&1 | cat`

## Documentation

- **Main logging docs:** `docs/logging.md`
- **costco-go prompt:** `docs/costco-go-logger-injection.md`
- **walmart-client prompt:** `docs/walmart-client-logger-injection.md`
- **Demo:** `examples/logging_demo.go`

## Questions?

- Check `docs/logging.md` for implementation details
- Review `examples/logging_demo.go` for examples
- The `MavenHandler` source is in `internal/infrastructure/logging/maven_handler.go`
