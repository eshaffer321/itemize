# Costco-Go Logger Injection Prompt

Copy this prompt to use with the `costco-go` project:

---

## Task: Add Optional Logger Injection to costco-go

### Context

The `costco-go` library currently uses Go's standard `log` package directly. I want to add support for **optional logger injection** using Go's standard `log/slog` package while maintaining backward compatibility.

### Goals

1. **Add optional `*slog.Logger` parameter** to `NewClient()` and `Client` struct
2. **Replace all `log.*` calls** with `logger.*` calls using structured logging
3. **Maintain backward compatibility** - if no logger is provided, use a no-op logger
4. **Use structured logging** - Convert all logs to use key=value pairs instead of string formatting

### Current Implementation

The library currently uses:
```go
log.Printf("INFO fetching online orders ...")
log.Printf("ERROR failed to decode ...")
```

### Desired Pattern

**Client struct update:**
```go
type Client struct {
    config Config
    client *http.Client
    token  *Token
    logger *slog.Logger  // ADD THIS
}
```

**Constructor update:**
```go
func NewClient(config Config, logger *slog.Logger) *Client {
    // If no logger provided, use a no-op logger (discards all output)
    if logger == nil {
        logger = slog.New(slog.NewTextHandler(io.Discard, nil))
    }

    return &Client{
        config: config,
        client: &http.Client{Timeout: 30 * time.Second},
        logger: logger,
    }
}
```

**Convert log calls from:**
```go
log.Printf("INFO fetching online orders client=costco start_date=%s end_date=%s page_number=%d page_size=%d",
    startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), pageNumber, pageSize)
```

**To:**
```go
c.logger.Info("fetching online orders",
    "client", "costco",
    "start_date", startDate.Format("2006-01-02"),
    "end_date", endDate.Format("2006-01-02"),
    "page_number", pageNumber,
    "page_size", pageSize)
```

**For errors:**
```go
// From:
log.Printf("ERROR failed to decode graphql response client=costco error=\"%v\"", err)

// To:
c.logger.Error("failed to decode graphql response",
    "client", "costco",
    "error", err)
```

**For warnings:**
```go
// From:
log.Printf("WARN failed to decode receipts as array, trying object format client=costco")

// To:
c.logger.Warn("failed to decode receipts as array, trying object format",
    "client", "costco")
```

### Files to Update

The following files in the `pkg/costco/` directory contain logging calls (approximately 61 total):

1. **`client.go`** - Main client struct and NewClient constructor
   - Update `Client` struct to include `logger *slog.Logger` field
   - Update `NewClient()` to accept optional logger parameter
   - Replace ~40 log calls with structured logging

2. **`helpers.go`** - Helper functions
   - Functions will need to accept `logger` parameter or access via receiver
   - Replace ~15 log calls

3. **`types.go`** - Type definitions
   - Replace ~6 log calls in methods

For each file:
1. Add `logger *slog.Logger` field to any struct that logs
2. Update constructor to accept optional logger parameter
3. Convert all `log.*` calls to structured `c.logger.*` calls
4. Remove the `"log"` import, add `"log/slog"` and `"io"`

### Backward Compatibility

**Critical:** Existing code should continue to work without changes:

```go
// Old way (still works, but logs go nowhere)
client := costcogo.NewClient(config, nil)

// New way (with custom logger)
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
client := costcogo.NewClient(config, logger)
```

### Testing Requirements

1. **Unit tests should pass** - Existing tests should work without modification
2. **Add logger injection test** - Verify logs go to the provided logger
3. **Test no-op behavior** - Verify nil logger doesn't crash and doesn't output

Example test:
```go
func TestClientWithLogger(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewTextHandler(&buf, nil)
    logger := slog.New(handler)

    client := NewClient(config, logger)
    // ... perform operation that logs ...

    output := buf.String()
    if !strings.Contains(output, "expected log message") {
        t.Errorf("Expected log output not found")
    }
}

func TestClientWithoutLogger(t *testing.T) {
    // Should not crash with nil logger
    client := NewClient(config, nil)
    // ... perform operation that would normally log ...
    // Should succeed without panicking
}
```

### Log Level Mapping

- Current `log.Printf("INFO ...")` → `logger.Info(...)`
- Current `log.Printf("ERROR ...")` → `logger.Error(...)`
- Current `log.Printf("WARN ...")` → `logger.Warn(...)`
- Current `log.Printf("DEBUG ...")` → `logger.Debug(...)` (if any exist)

### Key Points

1. **Use `io.Discard` for nil logger** - Not `os.Stdout` or `os.Stderr`
2. **Keep "client" attribute** - Always include `"client", "costco"` in logs
3. **Use structured logging** - Convert format strings to key=value pairs
4. **Don't change API** - Just add optional logger parameter at the end
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
    costcogo "github.com/costco-go/pkg/costco"
)

// Create a structured logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

// Pass logger to client
client := costcogo.NewClient(config, logger)
\`\`\`

If no logger is provided, the client will silently discard all logs:

\`\`\`go
// Logs are discarded
client := costcogo.NewClient(config, nil)
\`\`\`

All logs use structured logging with key=value pairs for easy parsing and filtering.
```

### Search Patterns

Use these patterns to find code that needs updating:

```bash
# Find all log calls
grep -r "log\." . --include="*.go" | grep -v "logger\."

# Find all log imports
grep -r '"log"' . --include="*.go"
```

### Common Mistakes to Avoid

1. ❌ Don't use `slog.Default()` - Always use the injected logger
2. ❌ Don't output to stdout/stderr if nil - Use `io.Discard`
3. ❌ Don't change function signatures of exported functions (except adding optional logger at end)
4. ❌ Don't mix string formatting (`%s`, `%d`) with structured logging - use key=value pairs

### Definition of Done

- [ ] All `log.*` calls replaced with `logger.*` calls
- [ ] `NewClient()` accepts optional `*slog.Logger` parameter
- [ ] Nil logger uses `io.Discard` handler (no output)
- [ ] All tests pass
- [ ] Added test for logger injection
- [ ] Updated README with logging documentation
- [ ] No breaking changes to existing API

---

## Additional Context

This change is part of standardizing logging across multiple Go projects. The consuming application uses a Maven-style logger that formats output as:

```
[INFO] [costco] [HH:MM:SS] message key=value key=value
```

By accepting `*slog.Logger`, the library will automatically format logs consistently with the rest of the application.
