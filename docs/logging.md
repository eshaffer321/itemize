# Logging

This project uses **Maven-style logging** with consistent formatting across all components.

## Format

```
[LEVEL] [SYSTEM] [HH:MM:SS] message key=value key=value
```

### Examples

```
[INFO] [sync] [20:40:56] Starting sync provider=Costco lookback_days=17
[INFO] [costco] [20:40:57] Fetching online orders start_date=2025-09-30
[INFO] [sync] [20:41:08] Multiple categories detected split_count=4
[WARN] [sync] [20:41:22] No matching transaction found order_id=21134300600532510011111
[ERROR] [storage] [20:42:15] Failed to save record error="database locked"
```

## Colors

Logs are **automatically colored** when output to a terminal (TTY):

- **INFO**: Cyan
- **WARN**: Yellow
- **ERROR**: Red
- **DEBUG**: Gray
- **Timestamps**: Gray

Colors are automatically **disabled** when piping to a file or non-terminal output.

## System Prefixes

Each component has a **system prefix** to identify the source:

- `[sync]` - Orchestrator and application logic
- `[costco]` - Costco provider and costco-go client
- `[walmart]` - Walmart provider and walmart-client
- `[monarch]` - Monarch Money API client
- `[storage]` - Database operations
- `[categorizer]` - AI categorization logic

## Creating System-Scoped Loggers

```go
import "github.com/eshaffer321/monarchmoney-sync-backend/internal/infrastructure/logging"

// Create a logger with system prefix
cfg := config.LoadOrEnv()
logger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "mycomponent")

// Use the logger
logger.Info("Starting operation", "items", 42)
// Output: [INFO] [mycomponent] [15:04:05] Starting operation items=42
```

## Injecting Loggers into External Libraries

Our logging is designed to be **portable** - you can pass the same `*slog.Logger` to external libraries for consistent formatting.

### For costco-go

The `costco-go` library currently uses the standard Go `log` package. To get consistent logging:

**Option 1: Propose Logger Injection (Recommended)**

Submit a PR to `costco-go` to accept an optional `*slog.Logger`:

```go
// In costco-go
type Client struct {
    logger *slog.Logger
    // ... existing fields
}

func NewClient(config Config, logger *slog.Logger) *Client {
    if logger == nil {
        logger = slog.New(slog.NewTextHandler(io.Discard, nil)) // no-op
    }
    return &Client{logger: logger}
}

// Then in methods:
func (c *Client) FetchOrders(...) {
    c.logger.Info("fetching online orders",
        "start_date", startDate,
        "page_number", 1)
}
```

**Then in our code:**

```go
costcoLogger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "costco")
costcoClient := costcogo.NewClient(costcoConfig, costcoLogger)
```

**Option 2: Wrap the Client (Current)**

For now, we create a system-scoped logger and pass it to our wrapper:

```go
// In providers.go
costcoLogger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "costco")
costcoClient := costcogo.NewClient(costcoConfig)
return costco.NewProvider(costcoClient, costcoLogger), nil
```

Our provider wrapper logs operations with the `[costco]` prefix. The underlying `costco-go` logs will still use the standard format until we update the library.

### For walmart-client

Same approach as costco-go:

```go
walmartLogger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "walmart")
walmartClient := walmartclient.NewWalmartClient(walmartConfig, walmartLogger)
```

## Configuration

Logging is configured in `config.yaml` or environment variables:

```yaml
observability:
  logging:
    level: info      # debug, info, warn, error
    format: maven    # Currently only maven is supported
```

Environment variables:
```bash
LOG_LEVEL=debug
LOG_FORMAT=maven
```

## Verbosity Control

Use the `-verbose` flag to see detailed logs:

```bash
# Minimal output
./monarch-sync costco -days 14

# Verbose output (shows all INFO logs)
./monarch-sync costco -days 14 -verbose

# Debug output (shows DEBUG logs too)
LOG_LEVEL=debug ./monarch-sync costco -days 14 -verbose
```

## Implementation Details

### Custom slog.Handler

We implement a custom `slog.Handler` in `internal/infrastructure/logging/maven_handler.go` that:

1. Formats logs in Maven style
2. Auto-detects TTY for color support
3. Extracts `system` attribute for the `[SYSTEM]` prefix
4. Maintains compatibility with standard `slog` interface

### Logger Factory

The `logging.NewLogger()` and `logging.NewLoggerWithSystem()` functions in `logger.go` create pre-configured loggers:

```go
// Base logger (no system prefix)
logger := logging.NewLogger(cfg.Observability.Logging)

// System-scoped logger
syncLogger := logging.NewLoggerWithSystem(cfg.Observability.Logging, "sync")
```

### WithAttrs for System Prefixes

System prefixes are added using `slog.Logger.With()`:

```go
logger.With("system", "costco")
```

The handler extracts the `system` attribute and displays it in brackets rather than as a key=value pair.

## Best Practices

1. **Always use system-scoped loggers** - Creates consistent, scannable output
2. **Use structured logging** - Add key=value pairs for context:
   ```go
   logger.Info("Processing order", "order_id", orderID, "total", amount)
   ```
3. **Log at appropriate levels**:
   - `DEBUG` - Detailed trace for debugging
   - `INFO` - Normal operations
   - `WARN` - Recoverable issues
   - `ERROR` - Failures that require attention

4. **Keep messages concise** - The message should describe *what*, attributes provide *details*:
   ```go
   // Good
   logger.Info("Matched transaction", "order_id", orderID, "transaction_id", txID)

   // Bad
   logger.Info(fmt.Sprintf("Matched transaction %s to order %s", txID, orderID))
   ```

## Testing

The Maven handler can be tested with different outputs:

```go
func TestLogger() {
    var buf bytes.Buffer
    handler := logging.NewMavenHandler(&buf, &slog.HandlerOptions{})
    logger := slog.New(handler).With("system", "test")

    logger.Info("Test message", "key", "value")

    // buf contains: [INFO] [test] [HH:MM:SS] Test message key=value
}
```

## Future Enhancements

- Add support for JSON output format (for log aggregation)
- Support disabling timestamps with flag
- Add log file output in addition to stdout
- Implement log rotation for file-based logging
