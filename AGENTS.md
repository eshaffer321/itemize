# AGENTS.md

**Last Updated:** June 2026

CLI + API server that syncs Walmart, Costco, and Amazon purchases into Monarch Money:
matches each order to its Monarch transaction, categorizes items with OpenAI, and splits
the transaction by category. Local single-user tool — not a SaaS, extension, or real-time service.

> This file is the single source of truth for agent guidance. `CLAUDE.md` is a symlink to it.

## Commands

```bash
# Build
go build -o itemize ./cmd/itemize/

# Preview a sync — real OpenAI calls, NO Monarch writes. Use this to test safely.
./itemize walmart -dry-run -days 14 -verbose
./itemize costco  -dry-run -days 7

# Apply
./itemize walmart -days 14
./itemize costco  -days 7 -max 10
./itemize walmart -force          # reprocess already-processed orders

# Serve
./itemize serve -port 8085

# Test
go test ./...              # all
go test ./... -race        # race detector (run before committing)
go test ./... -cover       # coverage (keep 80%+)
go test ./internal/domain/categorizer/... -v   # single package
```

## Configuration

Reads `config.yaml` or env vars:
- `MONARCH_TOKEN` — Monarch API token (required)
- LLM categorizer — set **one** of:
  - `OPENAI_API_KEY` (also `OPENAI_APIKEY`) with optional `OPENAI_MODEL` (default `gpt-5.4-nano`)
  - `ANTHROPIC_API_KEY` (also `CLAUDE_API_KEY`) with optional `ANTHROPIC_MODEL` (default `claude-haiku-4-5-20251001`)
- `CATEGORIZER_PROVIDER` — `openai` or `anthropic` to force a backend when both keys are set (auto-detected otherwise)
- SQLite DB auto-created at `monarch_sync.db`

## Architecture

Layered, with the domain core kept dependency-free. Flow: **CLI → application → domain ← adapters → infrastructure.**

```
internal/
  application/sync/   orchestrator + per-provider handlers (walmart, costco, amazon, simple)
  domain/             pure logic: categorizer/ (pluggable LLM backend), matcher/ (fuzzy), splitter/ (splits + tax)
  adapters/           providers/ (walmart, costco, amazon) and clients/ (Monarch, openai/, anthropic/)
  infrastructure/     config/, storage/ (SQLite + goose migrations), logging/ (slog)
  cli/                flag parsing + output
```

The categorizer depends on a `ChatClient` interface; concrete LLM backends
live under `internal/adapters/clients/{openai,anthropic}`. Selection happens
in `internal/adapters/clients/clients.go:newChatClient`.

Key entry points:
- Orchestrator — `internal/application/sync/orchestrator.go`
- Matcher (amount ±$0.01, date ±5 days) — `internal/domain/matcher/matcher.go`
- Categorizer (model set here today) — `internal/domain/categorizer/categorizer.go`
- Splitter (tax is proportional: `category_subtotal/total_subtotal * total_tax`) — `internal/domain/splitter/splitter.go`
- Storage / dedup — `internal/infrastructure/storage/sqlite.go`

## Conventions agents must follow

**TDD, and it's mandatory for bug fixes.** Before fixing a bug: write a failing test that
reproduces it, confirm it fails, then fix, then confirm green, then run the full suite, then log
it in `docs/bug-fixes.md`. Don't fix-first.

**Keep the domain layer pure.** No HTTP, DB, file I/O, or clock access in `internal/domain/` —
only pure functions over interfaces (`providers.OrderProvider`, `Order`, `OrderItem`). This is
what makes it testable without mocks.

**Log by level, not by `if verbose`.** `-verbose` sets the logger to debug; let the handler filter.

```go
// don't
if opts.Verbose { logger.Info("Processing order", "id", order.ID) }
// do
logger.Debug("Processing order", "id", order.ID)
```
Debug = diagnostics, Info = normal ops, Warn = recoverable, Error = needs attention.

**Never bypass dedup casually.** SQLite tracks processed orders to prevent duplicate splits.
`-force` is the only override and only when intentional — see `docs/deduplication-safety.md`.

**Commits:** `type: brief description` (`feat|fix|refactor|test|docs|chore`). Run `go test ./...`
and a `-dry-run` before committing.

## Adding a provider

Implement `providers.OrderProvider` in `internal/adapters/providers/<name>/`, add a config struct
in `config.go`, register in `internal/cli/providers.go`, add a handler in
`internal/application/sync/handlers/`, and test. Full walkthrough: `docs/adding-providers.md`.

## Gotchas

- **"No matching transaction found"** — transaction not posted yet (wait 1–3 days), amount off by
  >$0.01, or date off by >5 days. Use `-verbose` to see match scoring.
- **"Order already processed"** — expected; dedup working. `-force` to reprocess.
- **Provider auth** — Walmart cookies in `~/.walmart-api/cookies.json`; Costco creds saved by costco-go.
- **DB reset** — back up and delete `monarch_sync.db`; migrations (goose) re-run on startup.

## Deeper docs

`docs/architecture.md` · `docs/testing.md` · `docs/adding-providers.md` ·
`docs/deduplication-safety.md` · `docs/bug-fixes.md` · `docs/logging.md` · `docs/design-migrations.md`
