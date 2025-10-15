# Monarch Money Sync Backend

Automatically sync and categorize purchases from multiple retailers (Walmart, Costco) in Monarch Money by intelligently splitting transactions based on individual items purchased.

## ✨ Features

- 🔄 **Multi-Provider Support** - Walmart and Costco with extensible provider architecture
- 🔍 **Automatic Transaction Matching** - Fuzzy matching between orders and Monarch transactions
- 🤖 **AI-Powered Categorization** - Uses OpenAI GPT-4 to intelligently categorize items
- ✂️ **Smart Transaction Splitting** - Groups items by category with proportional tax distribution
- 📝 **Detailed Notes** - Includes item details in each split for transparency
- 🔒 **Duplicate Prevention** - SQLite tracking to avoid reprocessing orders
- 🏃 **Dry Run Mode** - Preview changes before applying them
- 📊 **Processing History** - Complete audit trail of all synced orders

## 📸 Example

**Before**: Single Walmart transaction for $150.31

**After**: Automatically split into:
- $104.57 - Groceries (milk, bread, eggs, cheese, produce)
- $28.42 - Home & Garden (paper towels, cleaning supplies)
- $17.32 - Personal Care (shampoo, toothpaste)

Each split includes detailed notes listing the specific items in that category.

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Application                          │
│              (cmd/monarch-sync/main.go)                      │
└──────────────────┬──────────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────────┐
│              Application Layer                               │
│         Orchestrates the sync workflow                       │
│           (internal/application/sync)                        │
└─┬─────────────┬────────────┬──────────────┬────────────────┘
  │             │            │              │
  ▼             ▼            ▼              ▼
┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────┐
│  Domain  │ │ Adapters │ │Infrastructure│ │     CLI      │
│  Layer   │ │  Layer   │ │   Layer    │ │   Layer      │
└──────────┘ └──────────┘ └───────────┘ └──────────────┘
│           │            │              │
│ matcher   │ providers  │ config       │ flags
│ splitter  │ clients    │ storage      │ output
│categorizer│ (costco)   │ logging      │ providers
│           │ (walmart)  │              │
└───────────┴────────────┴──────────────┴──────────────┘
```

## 🚀 Quick Start

### Prerequisites

1. **Go 1.24+** installed
2. **Monarch Money** account with API token
3. **OpenAI API** key
4. **Walmart** and/or **Costco** account credentials

### Installation

```bash
# Clone the repository
git clone https://github.com/eshaffer321/monarchmoney-sync-backend
cd monarchmoney-sync-backend

# Build the CLI
go build -o monarch-sync ./cmd/monarch-sync/
```

### Configuration

Create a `config.yaml` or set environment variables:

```yaml
monarch:
  api_key: "${MONARCH_TOKEN}"

openai:
  api_key: "${OPENAI_API_KEY}"  # or OPENAI_APIKEY
  model: "gpt-4o"

storage:
  database_path: "monarch_sync.db"

providers:
  costco:
    enabled: true
    lookback_days: 14
  walmart:
    enabled: true
    lookback_days: 14
```

**Environment Variables** (alternative):
```bash
export MONARCH_TOKEN="your_monarch_token"
export OPENAI_APIKEY="your_openai_api_key"  # or OPENAI_API_KEY
```

## 💻 Usage

### Costco Sync

```bash
# Dry run (preview only, no changes)
./monarch-sync costco -dry-run -days 14 -verbose

# Apply changes
./monarch-sync costco -days 14 -verbose
```

### Walmart Sync

```bash
# Dry run (preview only, no changes)
./monarch-sync walmart -dry-run -days 14 -verbose

# Apply changes
./monarch-sync walmart -days 14 -verbose
```

### CLI Options

| Flag | Default | Description |
|------|---------|-------------|
| `-dry-run` | `false` | Preview changes without applying |
| `-days` | `14` | Number of days to look back |
| `-max` | `0` | Maximum orders to process (0 = all) |
| `-verbose` | `false` | Show detailed logging output |
| `-force` | `false` | Reprocess already processed orders |

## 📋 Example Output

```
🛒 Walmart → Monarch Money Sync
===================================================
🔍 DRY RUN MODE - No changes will be made

💾 Using database: monarch_sync.db
📅 Configuration:
   Provider: Walmart
   Lookback: 7 days
   Max orders: 3
   Force reprocess: false

🛍️ Fetching orders...
Found 3 orders

💳 Fetching Monarch transactions...
Found 4 Walmart transactions

🔄 Processing orders...

[1/3] Processing order 200013706046836
  ✅ Matched with transaction: $110.52 on 2025-10-09
  🤖 Categorizing 12 items...
  ✂️  Creating splits...
  📊 Split into 2 categories
  🔍 [DRY RUN] Would apply 2 splits

============================================================
📊 SUMMARY
   Processed: 2
   Skipped:   0
   Errors:    1

📈 ALL TIME STATS
   Total Orders: 5
   Total Splits: 4
   Total Amount: $619.15
   Success Rate: 40.0%
```

## 🧪 Development

### Project Structure

```
monarchmoney-sync-backend/
├── cmd/
│   └── monarch-sync/        # Main CLI entry point
├── internal/
│   ├── application/         # Workflow orchestration
│   │   └── sync/            # Sync orchestrator
│   ├── domain/              # Business logic (pure functions)
│   │   ├── categorizer/     # AI-powered categorization
│   │   ├── matcher/         # Transaction matching algorithm
│   │   └── splitter/        # Transaction splitting logic
│   ├── adapters/            # External integrations
│   │   ├── providers/       # Retailer APIs
│   │   │   ├── costco/      # Costco implementation
│   │   │   └── walmart/     # Walmart implementation
│   │   └── clients/         # API clients (Monarch, OpenAI)
│   ├── infrastructure/      # Technical foundations
│   │   ├── config/          # Configuration management
│   │   ├── storage/         # SQLite persistence
│   │   └── logging/         # Structured logging
│   └── cli/                 # CLI interface
│       ├── flags.go         # Flag parsing
│       ├── output.go        # User-facing output
│       └── providers.go     # Provider initialization
├── config.yaml              # Configuration file
├── CLAUDE.md                # Development guide
└── README.md
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific package tests
go test ./internal/domain/matcher/... -v
go test ./internal/domain/categorizer/... -v
go test ./internal/adapters/providers/walmart/... -v
```

### Adding a New Provider

See [docs/adding-providers.md](docs/adding-providers.md) for a complete guide.

Quick overview:
1. Implement the `OrderProvider` interface in `internal/adapters/providers/`
2. Add configuration in `internal/infrastructure/config/`
3. Register in `internal/cli/providers.go`

## 🔧 How It Works

### 1. Order Fetching
Each provider (Walmart, Costco) implements the `OrderProvider` interface and fetches orders with full item details.

### 2. Transaction Matching
The matcher uses fuzzy logic to match orders with Monarch transactions:
- Amount matching (within $0.50 tolerance)
- Date matching (within 5 days tolerance)
- Confidence scoring

### 3. Item Categorization
OpenAI analyzes each item name and maps it to your Monarch categories:
```
"Great Value Milk 1 Gallon" → "Groceries"
"Bounty Paper Towels"       → "Home & Garden"
"Colgate Toothpaste"        → "Personal Care"
```

Results are cached to minimize API calls.

### 4. Transaction Splitting
Items are grouped by category and tax is distributed proportionally:
```
Category Tax = (Category Subtotal / Order Subtotal) × Total Tax
```

### 5. Monarch Update
Splits are created in Monarch with detailed notes listing all items in each category.

## 🔧 Troubleshooting

### "No matching transaction found"

The order hasn't been imported to Monarch yet, or the transaction details don't match:
- Wait 1-3 days for the transaction to post
- Verify amounts match within $0.50
- Check dates are within 5 days
- Use `-verbose` to see matching details

### "OpenAI API key not found"

Set the environment variable:
```bash
export OPENAI_APIKEY="sk-..."  # or OPENAI_API_KEY
```

Or add to `config.yaml`:
```yaml
openai:
  api_key: "sk-..."
```

### "Order already processed"

Use `-force` to reprocess:
```bash
./monarch-sync walmart -force
```

### Database Schema Migration

The app automatically migrates from old schema versions on startup. If you encounter issues, backup and delete `monarch_sync.db` to start fresh.

## 🚧 Limitations

- Tax distribution is proportional (doesn't account for tax-exempt items)
- Provider credentials require manual setup
- OpenAI API costs apply (~$0.01-0.05 per order)

## 🔮 Future Enhancements

- [ ] Additional providers (Sam's Club, Amazon, Target)
- [ ] Web UI for monitoring and configuration
- [ ] Automatic provider credential refresh
- [ ] Receipt OCR for accurate tax handling
- [ ] Budget impact analysis and alerts
- [ ] Scheduled/automated runs
- [ ] Category learning from user corrections

## 📄 License

MIT

## 🙏 Acknowledgments

- [monarchmoney-go](https://github.com/eshaffer321/monarchmoney-go) - Monarch Money API SDK
- [walmart-api](https://github.com/eshaffer321/walmart-api) - Walmart API client
- [costco-go](https://github.com/eshaffer321/costco-go) - Costco API client
- OpenAI GPT-4 for intelligent item categorization
