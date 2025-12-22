# Monarch Money Sync

CLI tool that syncs purchases from Walmart, Costco, and Amazon with Monarch Money. Automatically splits transactions by category using AI.

## What it does

1. Fetches orders from retailers with item details
2. Matches them to transactions in Monarch Money
3. Categorizes items using OpenAI
4. Splits the transaction by category with proportional tax

**Example**: A $150 Walmart transaction becomes:
- $104.57 Groceries (milk, bread, eggs...)
- $28.42 Household (paper towels, cleaning supplies)
- $17.32 Personal Care (shampoo, toothpaste)

## Setup

### Prerequisites

- Go 1.24+
- Monarch Money account
- OpenAI API key
- Retailer account(s)

### Install

```bash
git clone https://github.com/eshaffer321/monarchmoney-sync-backend
cd monarchmoney-sync-backend
go build -o monarch-sync ./cmd/monarch-sync/
```

### Configure

Set environment variables:
```bash
export MONARCH_TOKEN="your_monarch_token"
export OPENAI_API_KEY="your_openai_key"
```

Or create `config.yaml`:
```yaml
monarch:
  api_key: "${MONARCH_TOKEN}"

openai:
  api_key: "${OPENAI_API_KEY}"
  model: "gpt-4o"

storage:
  database_path: "monarch_sync.db"
```

## Usage

```bash
# Preview changes (dry run)
./monarch-sync walmart -dry-run -days 14
./monarch-sync costco -dry-run -days 7
./monarch-sync amazon -dry-run -days 7

# Apply changes
./monarch-sync walmart -days 14
./monarch-sync costco -days 7
./monarch-sync amazon -days 7
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
amazon-scraper --login  # authenticate once
```

## Troubleshooting

**"No matching transaction found"**
- Transaction hasn't posted to Monarch yet (wait 1-3 days)
- Amount differs by more than $0.01
- Date differs by more than 5 days

**"Order already processed"**
- Use `-force` to reprocess

**OpenAI errors**
- Check `OPENAI_API_KEY` is set and has credits

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
