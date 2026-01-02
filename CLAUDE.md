# Developer Guide for AI Assistants

**Last Updated:** December 2024
**Project Status:** Production-ready CLI application with Web UI

## What This Project Is

A CLI application that syncs Walmart, Costco, and Amazon purchases with Monarch Money, automatically categorizing items and splitting transactions. It now includes:
- **CLI tool** for command-line syncing
- **API server** (`./monarch-sync serve`) for programmatic access
- **Web UI** (Next.js) for monitoring and triggering syncs

## Quick Reference

### Build and Run
```bash
# Build Go backend
go build -o monarch-sync ./cmd/monarch-sync/

# Run with dry-run (preview, no changes)
./monarch-sync walmart -dry-run -days 14 -verbose
./monarch-sync costco -dry-run -days 7

# Apply changes
./monarch-sync walmart -days 14
./monarch-sync costco -days 7 -max 10

# Force reprocess already-processed orders
./monarch-sync walmart -force

# Start API server (for web UI)
./monarch-sync serve -port 8085
```

### Web UI
```bash
cd web

# Install dependencies
npm install

# Start development server (port 3000)
npm run dev

# Build for production
npm run build

# Start production server
npm start
```

### Test
```bash
# Go tests (all)
go test ./... -v

# Specific layer
go test ./internal/domain/... -v
go test ./internal/adapters/providers/walmart/... -v

# Coverage
go test ./... -cover

# Race detection
go test ./... -race
```

### Frontend E2E Tests (Playwright)
```bash
cd web

# Run all E2E tests (requires dev server running or uses webServer config)
npx playwright test

# Run specific test file
npx playwright test navigation.spec.ts

# Run tests with UI mode (interactive)
npx playwright test --ui

# Run tests with visible browser
npx playwright test --headed

# Generate HTML report
npx playwright show-report
```

**Playwright Test Files:**
- `web/e2e/navigation.spec.ts` - Navigation and page loading tests
- `web/e2e/sync.spec.ts` - Sync page functionality
- `web/e2e/dark-mode.spec.ts` - Theme switching tests
- `web/e2e/search.spec.ts` - Search and filtering
- `web/e2e/date-filter.spec.ts` - Date range filtering

### Configuration
The app reads from `config.yaml` or environment variables:
- `MONARCH_TOKEN` - Monarch Money API token (required)
- `OPENAI_APIKEY` or `OPENAI_API_KEY` - OpenAI API key (required)
- Database at `monarch_sync.db` (auto-created)

## Architecture at a Glance

**Layered architecture** with clear separation of concerns:

```
internal/
├── application/sync/      # Orchestrator - coordinates the workflow
├── domain/                # Pure business logic (no dependencies)
│   ├── categorizer/       # AI categorization with OpenAI
│   ├── matcher/           # Fuzzy transaction matching
│   └── splitter/          # Transaction split creation
├── adapters/              # External integrations
│   ├── providers/         # Costco & Walmart order fetching
│   └── clients/           # Monarch & OpenAI client builders
├── infrastructure/        # Technical foundations
│   ├── config/            # YAML/env config loading
│   ├── storage/           # SQLite for deduplication
│   └── logging/           # Structured logging (slog)
└── cli/                   # Command-line interface
```

**Dependency flow:** CLI → Application → Domain ← Adapters → Infrastructure

See [docs/architecture.md](docs/architecture.md) for complete details.

### Web Frontend Architecture

```
web/
├── src/
│   ├── app/(app)/           # Next.js App Router pages
│   │   ├── page.tsx         # Dashboard
│   │   ├── orders/          # Orders list and detail
│   │   ├── runs/            # Sync runs list
│   │   ├── sync/            # Sync page with job detail
│   │   └── transactions/    # Transactions list and detail
│   ├── components/          # Catalyst UI components
│   └── lib/
│       └── api/             # API client and types
├── e2e/                     # Playwright E2E tests
└── playwright.config.ts     # Playwright configuration
```

**Frontend Tech Stack:**
- **Next.js 15** with App Router
- **TypeScript** for type safety
- **Tailwind CSS** for styling
- **Catalyst UI** component library
- **Playwright** for E2E testing

**Key Patterns:**
- Server components (page.tsx) for data fetching
- Client components for interactivity (e.g., `orders-table.tsx` with sorting)
- API types in `web/src/lib/api/types.ts`
- Shared table components with sorting in `web/src/components/table.tsx`

## Development Methodology: TDD

### Core Workflow
1. **Write test first** - Define expected behavior
2. **Run test, watch it fail** - Verify test actually tests something
3. **Write minimal code** - Just enough to pass
4. **Run test, watch it pass** - Verify implementation
5. **Refactor** - Clean up while keeping tests green

### Bug Fixing (MANDATORY)
When you discover a bug:
1. **STOP** - Don't fix it yet
2. **Write failing test** - Reproduce the bug
3. **Run test** - Confirm it fails as expected
4. **Fix the bug** - Minimal implementation
5. **Run test** - Confirm it passes
6. **Run all tests** - Ensure no regression
7. **Document** in [docs/bug-fixes.md](docs/bug-fixes.md)

### Test Requirements
- **All tests must pass** before committing
- **Maintain 80%+ coverage**
- **Domain layer:** Unit tests, no mocks (pure functions)
- **Application layer:** Integration tests with mock clients
- **No skipped tests** in CI (integration tests can be skipped locally)

See [docs/testing.md](docs/testing.md) for detailed guidelines.

## How the Sync Works

### End-to-End Flow
1. **Fetch orders** from provider (Walmart/Costco) with item details
2. **Fetch Monarch transactions** filtered by merchant name
3. **Match orders to transactions** using fuzzy matching (amount ± $0.01, date ± 5 days)
4. **Categorize items** with OpenAI (cached to reduce API calls)
5. **Create splits** grouping items by category with proportional tax
6. **Update Monarch** via monarchmoney-go SDK
7. **Save to SQLite** for deduplication and audit trail

### Key Files
- **Orchestrator:** [internal/application/sync/orchestrator.go](internal/application/sync/orchestrator.go)
- **Matcher:** [internal/domain/matcher/matcher.go](internal/domain/matcher/matcher.go)
- **Categorizer:** [internal/domain/categorizer/categorizer.go](internal/domain/categorizer/categorizer.go)
- **Splitter:** [internal/domain/splitter/splitter.go](internal/domain/splitter/splitter.go)
- **Storage:** [internal/infrastructure/storage/storage.go](internal/infrastructure/storage/storage.go)

## Common Tasks

### Adding a New Provider
See [docs/adding-providers.md](docs/adding-providers.md) for complete guide.

Quick steps:
1. Create `internal/adapters/providers/newprovider/`
2. Implement `OrderProvider` interface from [types.go](internal/adapters/providers/types.go)
3. Add config struct in [internal/infrastructure/config/config.go](internal/infrastructure/config/config.go)
4. Register provider in [internal/cli/providers.go](internal/cli/providers.go)
5. Add test in `provider_test.go`

### Modifying Matching Logic
1. Update [internal/domain/matcher/matcher.go](internal/domain/matcher/matcher.go)
2. Add tests in `matcher_test.go`
3. Adjust `Config` struct for tolerance settings

### Changing Categorization
1. Core logic in [internal/domain/categorizer/categorizer.go](internal/domain/categorizer/categorizer.go)
2. OpenAI integration in `openai_client.go`
3. Caching in `cache.go` (in-memory for now)
4. Consider cache invalidation strategy

### Modifying Split Logic
1. Update [internal/domain/splitter/splitter.go](internal/domain/splitter/splitter.go)
2. Tax distribution is proportional: `(category_subtotal / total_subtotal) * total_tax`
3. Tests cover edge cases (single item, all same category, etc.)

## Critical Principles

### Domain Layer is Pure
**Domain logic has ZERO external dependencies:**
- No HTTP calls
- No database access
- No file I/O
- Only pure functions working with interfaces

This makes it:
- Fast to test
- Easy to reason about
- Portable to other contexts

### Interfaces Over Concrete Types
Key interfaces:
- `providers.OrderProvider` - Any retailer
- `providers.Order` - Uniform order representation
- `providers.OrderItem` - Uniform item representation

This allows easy mocking and provider swapping.

### Deduplication is Critical
The SQLite database tracks processed orders to prevent:
- Duplicate splits
- Reprocessing on every run
- Accidental double charges

Override with `-force` flag only when intentional.

See [docs/deduplication-safety.md](docs/deduplication-safety.md).

### Configuration Flexibility
Supports both YAML and environment variables:
- YAML preferred for local development
- Env vars for production/containers
- Graceful fallback and validation

### Logging Pattern
**Use proper log levels, not conditional logging:**

```go
// ❌ WRONG - Don't do this
if opts.Verbose {
    logger.Info("Processing order", "id", order.ID)
}

// ✅ RIGHT - Use appropriate log level
logger.Debug("Processing order", "id", order.ID)
```

**Log Level Guidelines:**
- `logger.Debug()` - Detailed diagnostics, shown only with `-verbose` flag
- `logger.Info()` - Normal operations, always shown
- `logger.Warn()` - Recoverable issues or unexpected conditions
- `logger.Error()` - Failures requiring attention

**How It Works:**
- Loggers are created with level control via `config.LoggingConfig.Level`
- When `-verbose` flag is set, level is set to `"debug"` before logger creation
- slog's `HandlerOptions.Level` filters which messages are displayed
- Each system (sync, costco, walmart) has its own logger with proper level

**Implementation:**
```go
// In CLI layer (providers.go)
loggingCfg := cfg.Observability.Logging
if verbose {
    loggingCfg.Level = "debug"
}
logger := logging.NewLoggerWithSystem(loggingCfg, "system-name")
```

This gives us clean code with proper level-based filtering instead of scattered conditionals.

## File Organization

### Where Things Live
- **Entry point:** [cmd/monarch-sync/main.go](cmd/monarch-sync/main.go)
- **Business logic:** `internal/domain/`
- **Workflow coordination:** `internal/application/sync/`
- **External APIs:** `internal/adapters/`
- **Config/storage/logging:** `internal/infrastructure/`
- **CLI parsing/output:** `internal/cli/`

### When Adding New Code
Ask yourself:
- **Is this pure business logic?** → `internal/domain/`
- **Does this orchestrate a workflow?** → `internal/application/`
- **Does this talk to an external system?** → `internal/adapters/`
- **Is this infrastructure (config, DB, logging)?** → `internal/infrastructure/`
- **Is this CLI-specific?** → `internal/cli/`

## External Dependencies

### Required SDKs
- **monarchmoney-go** - [github.com/eshaffer321/monarchmoney-go](https://github.com/eshaffer321/monarchmoney-go)
  - Handles Monarch Money API
  - Transactions, categories, splits
- **walmart-client** - [github.com/eshaffer321/walmart-api/walmart-client](https://github.com/eshaffer321/walmart-api)
  - Fetches Walmart orders
  - Cookie-based authentication
- **costco-go** - [github.com/eshaffer321/costco-go](https://github.com/eshaffer321/costco-go)
  - Fetches Costco orders
  - Uses saved credentials

### API Integrations
- **OpenAI GPT-4** - Item categorization (~$0.01-0.05 per order)
- **Monarch Money** - Transaction management
- **Walmart.com** - Order history
- **Costco.com** - Order history

## Database Schema

SQLite database at `monarch_sync.db` with auto-migration:

```sql
-- Processing history and deduplication
CREATE TABLE processing_records (
  order_id TEXT PRIMARY KEY,
  provider TEXT,
  transaction_id TEXT,
  order_date TIMESTAMP,
  order_total REAL,
  transaction_amount REAL,
  item_count INTEGER,
  split_count INTEGER,
  processed_at TIMESTAMP,
  status TEXT,  -- 'success', 'failed', 'dry-run'
  error_message TEXT,
  match_confidence REAL,
  dry_run BOOLEAN
);

-- Sync run tracking
CREATE TABLE sync_runs (
  run_id INTEGER PRIMARY KEY,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  orders_found INTEGER,
  processed INTEGER,
  skipped INTEGER,
  errors INTEGER,
  dry_run BOOLEAN,
  lookback_days INTEGER
);
```

Schema auto-migrates on startup. See [internal/infrastructure/storage/storage.go](internal/infrastructure/storage/storage.go).

## Project Scope

- ✅ **CLI tool** - Primary command-line interface
- ✅ **API server** - HTTP endpoints for programmatic access (`./monarch-sync serve`)
- ✅ **Web UI** - Next.js dashboard for monitoring and triggering syncs
- ❌ **NOT a Chrome extension** - Direct provider API integration
- ❌ **NOT a SaaS** - Local deployment only
- ❌ **NOT real-time** - Manual runs, web-triggered, or scheduled via cron
- ❌ **NOT multi-user** - Single user per config

## Troubleshooting

### "No matching transaction found"
- Transaction hasn't posted to Monarch yet (wait 1-3 days)
- Amount mismatch > $0.01
- Date difference > 5 days
- Use `-verbose` to see matching details

### "Order already processed"
- Normal! Prevents duplicate processing
- Use `-force` to reprocess if needed

### OpenAI Errors
- Check `OPENAI_APIKEY` or `OPENAI_API_KEY` is set
- Verify API key has credits
- Check internet connectivity

### Provider Authentication
- **Walmart:** Cookies in `~/.walmart-api/cookies.json`
- **Costco:** Credentials in Costco config (saved by costco-go)

### Database Issues
- Backup and delete `monarch_sync.db` to reset
- Auto-migration handles schema updates
- Check file permissions

## Documentation

- **[README.md](README.md)** - User-facing documentation
- **[docs/architecture.md](docs/architecture.md)** - Detailed architecture
- **[docs/testing.md](docs/testing.md)** - Testing strategy
- **[docs/adding-providers.md](docs/adding-providers.md)** - Provider development guide
- **[docs/deduplication-safety.md](docs/deduplication-safety.md)** - How duplicate prevention works
- **[docs/bug-fixes.md](docs/bug-fixes.md)** - Log of bugs and test cases

## Working on This Project

### Before Starting Work
1. Read this guide
2. Review [docs/architecture.md](docs/architecture.md)
3. Run tests to ensure clean state: `go test ./...`
4. Check git status: `git status`

### While Working
1. **Write tests first** (TDD)
2. Keep tests passing
3. Follow layered architecture
4. Use meaningful commit messages
5. Document decisions in relevant docs

### Before Committing
1. Run all tests: `go test ./...`
2. Check coverage: `go test ./... -cover`
3. Run with `-dry-run` to verify CLI works
4. Update docs if architecture/behavior changed

### Commit Message Style
```
type: Brief description

Longer explanation if needed.

- Bullet points for details
- Reference issue numbers if applicable
```

Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

## Getting Help

When stuck:
1. Check [docs/](docs/) directory
2. Read relevant test files for examples
3. Look at existing provider implementations as reference
4. Review git history for similar changes

## Future Enhancements

Potential areas for expansion:
- Additional providers (Amazon, Target, Sam's Club)
- Web UI for configuration and monitoring
- Advanced categorization with learning
- Receipt OCR for precise tax handling
- Budget impact analysis
- Scheduled runs (systemd timer, cron)
- Multi-user support

See README.md "Future Enhancements" section.
