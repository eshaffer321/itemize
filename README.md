# Itemize

CLI tool that syncs purchases from Walmart, Costco, and Amazon with Monarch. Automatically splits transactions by category using AI.

## What it does

1. Fetches orders from retailers with item details
2. Matches them to transactions in Monarch
3. Categorizes items using an LLM (OpenAI or Anthropic Claude)
4. Splits the transaction by category with proportional tax

**Example**: A $150 Walmart transaction becomes:
- $104.57 Groceries (milk, bread, eggs...)
- $28.42 Household (paper towels, cleaning supplies)
- $17.32 Personal Care (shampoo, toothpaste)

## Setup

### Prerequisites

- Go 1.24+
- Monarch account
- An LLM API key — OpenAI or Anthropic (Claude)
- Retailer account(s)

### Install

```bash
go install github.com/eshaffer321/itemize/cmd/itemize@latest
```

Prebuilt binaries for macOS (amd64/arm64) and Linux (amd64/arm64) are also published on the
[GitHub Releases page](https://github.com/eshaffer321/itemize/releases) whenever a version tag
(`vX.Y.Z`) is pushed. Download the archive for your platform and extract the `itemize` binary.

Run `itemize -version` (or `itemize version`) at any time to confirm exactly which build you're
running — useful for checking whether a locally built binary is stale.

#### Build from source

For contributors, or to build against an unreleased commit:

```bash
git clone https://github.com/eshaffer321/itemize
cd itemize
go build -o itemize ./cmd/itemize/
```

### Configure

Set environment variables:
```bash
export MONARCH_TOKEN="your_monarch_token"

# Pick one LLM backend:
export OPENAI_API_KEY="your_openai_key"
# or
export ANTHROPIC_API_KEY="your_anthropic_key"
```

Or create `config.yaml`:
```yaml
monarch:
  api_key: "${MONARCH_TOKEN}"

openai:
  api_key: "${OPENAI_API_KEY}"
  model: "gpt-5.4-nano"

anthropic:
  api_key: "${ANTHROPIC_API_KEY}"
  model: "claude-haiku-4-5-20251001"

# Optional: force a backend when both keys are set.
# Leave blank to auto-detect from whichever key is present.
categorizer:
  provider: ""  # "openai" | "anthropic" | ""

storage:
  database_path: "monarch_sync.db"
```

#### Choosing an LLM backend

itemize picks the LLM backend based on which API key is configured:

- Only `OPENAI_API_KEY` set → OpenAI is used.
- Only `ANTHROPIC_API_KEY` (or `CLAUDE_API_KEY`) set → Claude is used.
- Both set → defaults to OpenAI; set `CATEGORIZER_PROVIDER=anthropic` to force Claude.

Override the model with `OPENAI_MODEL` or `ANTHROPIC_MODEL` per run.

## Usage

```bash
# Preview changes (dry run)
./itemize walmart -dry-run -days 14
./itemize costco -dry-run -days 7
./itemize amazon -dry-run -days 7

# Apply changes
./itemize walmart -days 14
./itemize costco -days 7
./itemize amazon -days 7
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-dry-run` | false | Preview without applying changes |
| `-days` | 14 | Days to look back for orders |
| `-max` | 0 | Max orders to process (0 = all) |
| `-verbose` | false | Show detailed logs |
| `-force` | false | Reprocess already-processed orders |

## Provider Setup

### Walmart
Requires cookies in `~/.walmart-api/cookies.json`. See [walmart-client-go](https://github.com/eshaffer321/walmart-client-go).

### Costco
Uses credentials saved by [costco-go](https://github.com/eshaffer321/costco-go).

### Amazon
Requires [amazon-order-scraper](https://www.npmjs.com/package/amazon-order-scraper) CLI:
```bash
npm install -g amazon-order-scraper
BROWSER_DATA_DIR="$HOME/.itemize/amazon" amazon-scraper --login  # authenticate once
```

`itemize` uses the same persistent browser profile base by default. If you set
`AMAZON_ACCOUNT_NAME`, pass the matching scraper profile during login:

```bash
BROWSER_DATA_DIR="$HOME/.itemize/amazon" amazon-scraper --login --profile "$AMAZON_ACCOUNT_NAME"
```

#### Multiple Amazon accounts

If you sync more than one Amazon account, use `-account` to pick a profile per run instead of
setting `AMAZON_ACCOUNT_NAME` (useful for one-off manual runs where you don't want to remember
an env var, or for cron jobs where the account name should be visible right in the crontab line):

```bash
./itemize amazon -list-accounts        # see which profiles you've logged into
./itemize amazon -account amazon-wife  # sync a specific account
```

`AMAZON_ACCOUNT_NAME` still works and is used as the default when `-account` is omitted — cron
jobs relying on the env var need no changes.

## Troubleshooting

**"No matching transaction found"**
- Transaction hasn't posted to Monarch yet (wait 1–3 days)
- Amount differs by more than $0.01
- Date differs by more than 5 days

**"Order already processed"**
- Use `-force` to reprocess

**OpenAI errors**
- Check `OPENAI_API_KEY` is set and has credits

## Telemetry

itemize collects anonymous usage data to help understand how the tool is being used and what errors occur in the wild. No personal information or credentials are ever sent.

**What is collected:**
- Which provider was run (`walmart`, `costco`, `amazon`)
- Sync flags used (`dry-run`, `lookback_days`, `force`, etc. — flag names only, no values that could contain secrets)
- Outcome counts (processed, skipped, errors)
- Error type and message when a sync fails
- OS and Go version (added automatically by the Sentry SDK)

**What is never collected:**
- API tokens or keys (`MONARCH_TOKEN`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.)
- Costco email or password
- Amazon account name or browser profile paths
- Order IDs, transaction IDs, or any financial data

**Opt out:**
```bash
export ITEMIZE_NO_TELEMETRY=1
# or the standard
export DO_NOT_TRACK=1
```

## Development

```bash
# Run tests
go test ./...

# With coverage
go test ./... -cover
```

See [CLAUDE.md](CLAUDE.md) for architecture details and development guide.

## License

MIT
