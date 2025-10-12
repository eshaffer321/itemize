# Walmart → Monarch Money Sync

[![CI](https://github.com/eshaffer321/monarchmoney-walmart-server/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/eshaffer321/monarchmoney-walmart-server/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/eshaffer321/monarchmoney-walmart-server/graph/badge.svg)](https://codecov.io/gh/eshaffer321/monarchmoney-walmart-server)

Automatically sync and categorize Walmart purchases in Monarch Money by splitting transactions based on individual items purchased.

## ✨ Features

- 🔍 **Automatic Transaction Matching** - Matches Walmart orders with Monarch Money transactions (handles delivery tips)
- 🤖 **AI-Powered Categorization** - Uses OpenAI GPT-4 to intelligently categorize items based on your Monarch categories
- ✂️ **Smart Transaction Splitting** - Groups items by category and splits transactions with proportional tax distribution
- 📝 **Detailed Notes** - Includes item details in each split for transparency
- 🔒 **Duplicate Prevention** - Tracks processed orders to avoid reprocessing
- 🏃 **Dry Run Mode** - Preview changes before applying them

## 📸 Example

**Before**: Single Walmart transaction for $150.00

**After**: Automatically split into:
- $50.00 - Groceries (milk, bread, eggs, cheese)
- $30.00 - Home Spending (paper towels, cleaning supplies)  
- $40.00 - Electronics (phone charger, batteries)
- $30.00 - Personal Care (shampoo, toothpaste)

## 🏗️ Architecture

```
Walmart API → Go Backend → OpenAI → Monarch Money API
     ↓            ↓          ↓            ↓
Get Orders    Match &    Categorize    Split & Update
             Process      Items        Transactions
```

## 🚀 Quick Start

### Prerequisites

1. **Go 1.21+** installed
2. **Monarch Money** account with API access
3. **OpenAI API** key
4. **Walmart** account

### Installation

```bash
# Clone the repository
git clone https://github.com/eshaffer321/monarchmoney-sync-backend
cd monarchmoney-sync-backend

# Install dependencies
go mod download
```

### Configuration

1. **Set Environment Variables**

```bash
export MONARCH_TOKEN="your_monarch_token"
export OPENAI_APIKEY="your_openai_api_key"
```

2. **Initialize Walmart Cookies**

```bash
# 1. Log into walmart.com in your browser
# 2. Go to your orders page  
# 3. Open Developer Tools (F12)
# 4. Go to Network tab
# 5. Find any GraphQL request
# 6. Right-click → Copy → Copy as cURL
# 7. Save to a file called 'curl.txt'

# Then run:
go run cmd/test-walmart/main.go
```

## 💻 Usage

### Basic Sync (Dry Run)

```bash
# Preview what would happen (no changes made)
go run cmd/walmart-sync/main.go --dry-run --days 14
```

### Apply Changes

```bash
# Actually update Monarch transactions
go run cmd/walmart-sync/main.go --dry-run=false --days 14
```

### CLI Options

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `true` | Preview changes without applying |
| `--days` | `14` | Number of days to look back |
| `--max` | `0` | Maximum orders to process (0 = all) |
| `--verbose` | `false` | Show detailed output |
| `--force` | `false` | Reprocess already processed orders |
| `--log` | `processing_log.json` | Processing log file path |

### Interactive Mode

Test with a single order and confirm before applying:

```bash
go run cmd/walmart-sync-test/main.go
```

## 📋 Example Output

```
🛒 Walmart → Monarch Money Sync
================================

🔍 DRY RUN MODE - No changes will be made
📅 Looking back 14 days for orders

🛍️ Fetching Walmart orders...
Found 3 orders

💳 Fetching Monarch transactions...
Found 5 Walmart transactions in Monarch

🔄 Processing orders...
======================================================================

[1/3] Processing order 18420337004257359578
  ✅ Matched with transaction: $7.57 on 2025-09-05
  ✂️  Splitting into categories...
  📊 Created 2 splits
     • $5.28 - Great Value Cracker Cut Sliced 4 Cheese Tray...
     • $2.29 - Great Value Birthday Party Candle, Multicolor...
  🔍 [DRY RUN] Would apply 2 splits

======================================================================
📊 SUMMARY
   Processed: 1 orders
   Skipped:   0 orders
   Errors:    0 orders
```

## 🧪 Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover
```

### Test Individual Components

```bash
# Test Walmart data fetching
go run cmd/test-walmart/main.go

# Test transaction matching
go run cmd/test-matching/main.go

# Test splitting logic
go run cmd/test-split-simple/main.go

# Test OpenAI categorization
go run cmd/test-categorization/main.go
```

### Project Structure

```
cmd/
  walmart-sync/         # Main CLI tool
  walmart-sync-test/    # Interactive test mode
  test-walmart/         # Walmart cookie initialization
  test-matching/        # Transaction matching test
  test-split-simple/    # Split logic test

categorizer/            # OpenAI categorization logic
processor/              # Transaction matching
splitter/               # Split creation logic
queue/                  # Queue abstraction (Redis-ready)
```

## 🔧 Troubleshooting

### "Walmart cookies not found"

Re-run the cookie initialization:
1. Get a fresh cURL command from walmart.com
2. Save to `curl.txt`
3. Run `go run cmd/test-walmart/main.go`

### "No matching transaction found"

- Transaction may not have posted in Monarch yet (wait 1-3 days)
- Verify amounts match within $0.50
- Check dates are within 3 days

### "Transaction already has splits"

The transaction was already processed. Use `--force` to reprocess.

### Parsing errors for some orders

Orders with weighted items (produce) may fail due to decimal quantities. This is a known limitation.

## 🎯 How It Works

1. **Fetch Orders**: Retrieves recent Walmart orders using saved browser cookies
2. **Match Transactions**: Finds corresponding transactions in Monarch Money with fuzzy matching
3. **Categorize Items**: Uses OpenAI to categorize each item based on your Monarch categories  
4. **Create Splits**: Groups items by category and distributes tax proportionally
5. **Update Monarch**: Applies the splits with detailed notes about what's in each category

### Tax Distribution

Tax is distributed proportionally based on each category's subtotal:
- Category Tax = (Category Subtotal / Order Subtotal) × Total Tax
- This approximates actual tax (some items may be tax-exempt)

## 🚧 Limitations

- Weighted items (produce sold by pound) may fail to parse
- Tax distribution is proportional (doesn't account for tax-exempt items)
- Requires manual cookie refresh periodically
- OpenAI API costs apply (~$0.01 per order)

## 🔮 Future Enhancements

- [ ] Redis queue for distributed processing
- [ ] Automatic cookie refresh
- [ ] Web UI for monitoring and configuration
- [ ] Support for Sam's Club and other retailers
- [ ] Receipt OCR for accurate tax handling
- [ ] Budget impact analysis
- [ ] Scheduled/automated runs
- [ ] Chrome extension for easier setup

## 📄 License

MIT

## 🤝 Contributing

Pull requests welcome! Please include tests for new features.

## 🙏 Acknowledgments

- [monarchmoney-go](https://github.com/eshaffer321/monarchmoney-go) SDK
- OpenAI GPT-4 for categorization
- Inspired by the need for better purchase tracking