# Itemize

CLI tool that syncs purchases from Walmart, Costco, Amazon, and Target with Monarch. Automatically splits transactions by category using AI.

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

#### Contributor checks

Install local tools and hooks before committing:

```bash
make install-tools
make install-hooks
```

The pre-commit hook runs formatting checks, `golangci-lint`, `go vet`, and `go test ./...`.
To bypass once in an emergency, run `ITEMIZE_SKIP_PRECOMMIT=1 git commit ...`.

### Configure

Set environment variables:
```bash
# Required — Monarch no longer accepts the old MONARCH_TOKEN bearer token. Log in at
# app.monarch.com, open DevTools → Network, find a request to api.monarch.com/graphql,
# and copy its full Cookie request header ("sessionid=...; csrftoken=...").
export MONARCH_COOKIE="sessionid=...; csrftoken=..."

# Pick one LLM backend:
export OPENAI_API_KEY="your_openai_key"
# or
export ANTHROPIC_API_KEY="your_anthropic_key"
```

Or create `config.yaml`:
```yaml
monarch:
  cookie: "${MONARCH_COOKIE}"

openai:
  api_key: "${OPENAI_API_KEY}"
  model: "gpt-6-luna"

anthropic:
  api_key: "${ANTHROPIC_API_KEY}"
  model: "claude-haiku-4-5"

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
./itemize target -dry-run -days 14

# Apply changes
./itemize walmart -days 14
./itemize costco -days 7
./itemize amazon -days 7
./itemize target -days 14
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
Set up an account with Itemize's guided login command:

```bash
./itemize amazon setup -account erick
```

Itemize creates a persistent browser profile under `~/.itemize/amazon/`, opens Chromium, and waits
for you to sign into Amazon. After setup, verify the account with a small dry run:

```bash
./itemize amazon -account erick -dry-run -days 14 -max 1
```

Itemize uses [amazon-go](https://github.com/eshaffer321/amazon-go) as a library. The login flow uses
Playwright to open Chromium. If Playwright is not already installed globally or in this repo,
install it once:

```bash
npm install playwright
```

If Playwright lives elsewhere, add `-playwright-root <dir>` to `amazon setup`, where `<dir>`
contains `node_modules/playwright`. The lower-level `-import-browser-profile` flag remains
available for importing a specific existing Chromium/Playwright profile.

#### Multiple Amazon accounts

If you sync more than one Amazon account, use `-account` to pick a cookie account per run instead of
setting `AMAZON_ACCOUNT_NAME` (useful for one-off manual runs where you don't want to remember
an env var, or for cron jobs where the account name should be visible right in the crontab line):

```bash
./itemize amazon -list-accounts        # see saved amazon-go cookie accounts
./itemize amazon -account amazon-wife  # sync a specific account
./itemize amazon amazon-wife           # shorthand for the same account
```

`AMAZON_ACCOUNT_NAME` still works and is used as the default when `-account` is omitted — cron
jobs relying on the env var need no changes.

### Target
Reads in-store receipts linked to your Target Circle account and online orders, through the JSON
APIs target.com's own order pages use ([target-go](https://github.com/daslicious/target-go); its
[PROTOCOL.md](https://github.com/daslicious/target-go/blob/main/PROTOCOL.md) documents them).
Target has no API login, so you save a browser login once and Itemize renews it before every run:

1. In Chrome, open an **Incognito window** and sign in at target.com. Target replaces the refresh
   token on every renewal, so the login must not stay in use in a browser that would renew it too.
2. Open target.com/orders, then DevTools → Network, filter `history`, and reload.
3. Right-click the `history` request marked `{}` (not the preflight) → Copy → **Copy as cURL (bash)**.
4. Save it; Itemize replays the request and saves it only if Target accepts it:

   ```bash
   pbpaste | ./itemize target set-session -account default    # or: xclip -o -selection clipboard | ...
   ```

5. Close the Incognito window **without signing out** (signing out ends the login).

Logins are saved to `~/.itemize/target/session-<account>.json` (`ITEMIZE_TARGET_DIR` to change).
Each run renews the 24-hour access token with a refresh token that lasts 180 days. Use
`-account` for more than one login; receipts on a shared Target Circle account show up in each
and are synced once.

```bash
./itemize target orders -account default -days 30   # orders as Itemize reads them (JSON), no Monarch
./itemize target refresh -account default           # renew the login now
```

An online order shipped in parts is charged once per shipment; each charge is matched and split on
its own, as `<order>`, `<order>-2`, …  Gift cards, manufacturer coupons and the reusable-bag
discount are left out of the amount to match. An expired login fails with
`target auth check failed`; Target's bot protection with `target blocked the request` (wait an hour
or more). Requests are paced 3-5 seconds apart.

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
- API tokens, keys, or cookies (`MONARCH_COOKIE`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.)
- Costco email or password
- Amazon account names or cookie file paths
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
