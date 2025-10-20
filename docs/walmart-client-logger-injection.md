# Walmart-Client Logger Injection Prompt

Copy this prompt to use with the `walmart-client` project:

---

## Task: Add Optional Logger Injection to walmart-client

### Context

The `walmart-client` library currently uses Go's standard `log` package directly. I want to add support for **optional logger injection** using Go's standard `log/slog` package while maintaining backward compatibility.

### Goals

1. **Add optional `*slog.Logger` parameter** to `NewWalmartClient()` and `WalmartClient` struct
2. **Replace all `log.*` calls** with `logger.*` calls using structured logging
3. **Maintain backward compatibility** - if no logger is provided, use a no-op logger
4. **Use structured logging** - Convert all logs to use key=value pairs instead of string formatting

### Current Implementation

The library currently uses:
```go
log.Printf("Fetching purchase history...")
log.Printf("Error: %v", err)
```

### Desired Pattern

**WalmartClient struct update:**
```go
type WalmartClient struct {
    config     ClientConfig
    httpClient *http.Client
    cookies    []*http.Cookie
    logger     *slog.Logger  // ADD THIS
}
```

**Constructor update:**
```go
func NewWalmartClient(config ClientConfig, logger *slog.Logger) (*WalmartClient, error) {
    // If no logger provided, use a no-op logger (discards all output)
    if logger == nil {
        logger = slog.New(slog.NewTextHandler(io.Discard, nil))
    }

    client := &WalmartClient{
        config:     config,
        httpClient: &http.Client{Timeout: 30 * time.Second},
        logger:     logger,
    }

    // ... rest of initialization ...
    return client, nil
}
```

**Convert log calls from:**
```go
log.Printf("Fetching purchase history for page %d", page)
```

**To:**
```go
w.logger.Info("fetching purchase history",
    "page", page)
```

**For errors:**
```go
// From:
log.Printf("Error fetching orders: %v", err)

// To:
w.logger.Error("failed to fetch orders",
    "error", err)
```

**For debug/trace:**
```go
// From:
log.Printf("Found %d orders", len(orders))

// To:
w.logger.Info("found orders",
    "count", len(orders))
```

### Files to Update

The following files contain logging calls (approximately 86 total):

1. **`client.go`** - Main client struct and NewWalmartClient constructor
   - Update `WalmartClient` struct to include `logger *slog.Logger` field
   - Update `NewWalmartClient()` to accept optional logger parameter
   - Replace ~30 log calls with structured logging

2. **`purchase_history.go`** - Purchase history fetching
   - Access logger via `w.logger`
   - Replace ~25 log calls

3. **`orderledger.go`** - Order ledger operations
   - Access logger via `w.logger`
   - Replace ~20 log calls

4. **`example_json.go`** and **`test_tip.go`** - Examples/utilities
   - Update to pass nil logger or create example logger
   - Replace ~11 log calls

For each file:
1. Add `logger *slog.Logger` field to `WalmartClient` struct
2. Update constructor to accept optional logger parameter
3. Convert all `log.*` calls to structured `w.logger.*` calls
4. Remove the `"log"` import, add `"log/slog"` and `"io"`

### Backward Compatibility

**Critical:** Existing code should continue to work without changes:

```go
// Old way (still works, but logs go nowhere)
client, err := walmartclient.NewWalmartClient(config, nil)

// New way (with custom logger)
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
client, err := walmartclient.NewWalmartClient(config, logger)
```

### Testing Requirements

1. **Unit tests should pass** - Existing tests should work without modification
2. **Add logger injection test** - Verify logs go to the provided logger
3. **Test no-op behavior** - Verify nil logger doesn't crash and doesn't output

Example test:
```go
func TestWalmartClientWithLogger(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewTextHandler(&buf, nil)
    logger := slog.New(handler)

    client, err := NewWalmartClient(config, logger)
    if err != nil {
        t.Fatal(err)
    }

    // ... perform operation that logs ...

    output := buf.String()
    if !strings.Contains(output, "expected log message") {
        t.Errorf("Expected log output not found")
    }
}

func TestWalmartClientWithoutLogger(t *testing.T) {
    // Should not crash with nil logger
    client, err := NewWalmartClient(config, nil)
    if err != nil {
        t.Fatal(err)
    }
    // ... perform operation that would normally log ...
    // Should succeed without panicking
}
```

### Log Level Mapping

Use these guidelines for log levels:

- **Info**: Normal operations (fetching data, parsing results, counts)
  - "fetching purchase history", "found orders", "saved cookies"

- **Warn**: Recoverable issues or unexpected but handled situations
  - "no orders found", "retrying request", "using cached data"

- **Error**: Failures that prevent operation
  - "failed to fetch orders", "failed to parse response", "authentication failed"

- **Debug**: Detailed trace information (if any exist)
  - "request details", "response body", "cookie values"

### Structured Logging Examples

**Before:**
```go
log.Printf("Fetching orders from %s to %s", startDate, endDate)
log.Printf("Found %d orders totaling $%.2f", count, total)
log.Printf("Error: %v", err)
```

**After:**
```go
w.logger.Info("fetching orders",
    "start_date", startDate,
    "end_date", endDate)

w.logger.Info("found orders",
    "count", count,
    "total", total)

w.logger.Error("operation failed",
    "error", err)
```

### Key Points

1. **Use `io.Discard` for nil logger** - Not `os.Stdout` or `os.Stderr`
2. **Use structured logging** - Convert format strings to key=value pairs
3. **Don't change API** - Just add optional logger parameter at the end
4. **Update examples** - Show logger usage in example files
5. **Update README** - Document the logger injection feature

### Example README Addition

Add this section to the README:

```markdown
## Logging

The client supports optional logger injection using Go's standard `log/slog` package:

\`\`\`go
import (
    "log/slog"
    "os"
    walmartclient "github.com/eshaffer321/walmart-client"
)

// Create a structured logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

// Pass logger to client
client, err := walmartclient.NewWalmartClient(config, logger)
\`\`\`

If no logger is provided, the client will silently discard all logs:

\`\`\`go
// Logs are discarded
client, err := walmartclient.NewWalmartClient(config, nil)
\`\`\`

All logs use structured logging with key=value pairs for easy parsing and filtering.
```

### Search Patterns

Use these patterns to find code that needs updating:

```bash
# Find all log calls
grep -r "log\." . --include="*.go" | grep -v "logger\." | grep -v "_test.go"

# Find all log imports
grep -r '"log"' . --include="*.go"

# Count log calls
grep -rn "log\." . --include="*.go" | grep -v "logger\." | wc -l
```

### Common Mistakes to Avoid

1. ❌ Don't use `slog.Default()` - Always use the injected logger
2. ❌ Don't output to stdout/stderr if nil - Use `io.Discard`
3. ❌ Don't change function signatures of exported functions (except adding optional logger at end)
4. ❌ Don't mix string formatting (`%s`, `%d`) with structured logging - use key=value pairs
5. ❌ Don't log sensitive data (passwords, full cookie values, etc.)

### Special Considerations for Walmart Client

**Cookie Logging:**
When logging cookie information, be careful not to log full cookie values:

```go
// Bad - logs sensitive data
w.logger.Info("cookie saved", "value", cookieValue)

// Good - logs metadata only
w.logger.Info("cookie saved",
    "name", cookie.Name,
    "domain", cookie.Domain,
    "expires", cookie.Expires)
```

**Rate Limiting:**
Log rate limit operations with structured data:

```go
w.logger.Info("rate limit applied",
    "wait_duration", duration,
    "requests_made", count)
```

### Definition of Done

- [ ] All `log.*` calls replaced with `logger.*` calls
- [ ] `NewWalmartClient()` accepts optional `*slog.Logger` parameter
- [ ] Nil logger uses `io.Discard` handler (no output)
- [ ] All tests pass
- [ ] Added test for logger injection
- [ ] Updated README with logging documentation
- [ ] No breaking changes to existing API
- [ ] No sensitive data logged (cookies, passwords, etc.)
- [ ] Examples updated to show logger usage

---

## Additional Context

This change is part of standardizing logging across multiple Go projects. The consuming application uses a Maven-style logger that formats output as:

```
[INFO] [walmart] [HH:MM:SS] message key=value key=value
```

By accepting `*slog.Logger`, the library will automatically format logs consistently with the rest of the application.
