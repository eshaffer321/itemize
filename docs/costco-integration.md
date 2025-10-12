# Costco Integration Guide

## Overview

The Costco provider has been integrated into the Monarch Money sync backend, allowing you to automatically sync your Costco purchases (both online orders and in-store receipts) with Monarch Money.

## Features

- ✅ Fetch online orders from Costco.com
- ✅ Fetch in-store receipt data
- ✅ Automatic categorization using AI
- ✅ Transaction splitting by category
- ✅ Support for both delivery and warehouse purchases

## Setup

### 1. Environment Variables

Set your Costco credentials:

```bash
export COSTCO_EMAIL="your-email@example.com"
export COSTCO_PASSWORD="your-password"
```

### 2. Configuration File

Add Costco to your `config.yaml`:

```yaml
providers:
  costco:
    enabled: true
    email: ${COSTCO_EMAIL}
    password: ${COSTCO_PASSWORD}
    lookback_days: 30
```

### 3. Test the Connection

Run the example to verify your setup:

```bash
go run cmd/costco-example/main.go
```

## Usage

### Sync Costco Orders Only

```bash
# Dry run mode (recommended first)
go run cmd/sync/main.go -provider=costco -dry-run

# Production mode
go run cmd/sync/main.go -provider=costco
```

### Sync All Providers (Walmart + Costco)

```bash
# This will sync both Walmart and Costco if enabled
go run cmd/sync/main.go
```

### Using the Proven Working Tool

The `walmart-sync-with-db` tool has been tested and proven to work. To use it with Costco, you would need to modify it slightly, but for now, use the new `cmd/sync` tool which supports multiple providers.

## Architecture

### Provider Implementation

The Costco provider (`internal/providers/costco/provider.go`) implements the `OrderProvider` interface:

```go
type OrderProvider interface {
    Name() string
    DisplayName() string
    FetchOrders(ctx context.Context, opts FetchOptions) ([]Order, error)
    GetOrderDetails(ctx context.Context, orderID string) (Order, error)
    SupportsDeliveryTips() bool
    SupportsRefunds() bool
    SupportsBulkFetch() bool
    GetRateLimit() time.Duration
    HealthCheck(ctx context.Context) error
}
```

### Data Flow

1. **Fetch Orders**: Costco client fetches orders and receipts
2. **Convert**: Orders are converted to the common `providers.Order` format
3. **Match**: Orders are matched with Monarch Money transactions
4. **Categorize**: Items are categorized using OpenAI
5. **Split**: Transactions are split by category
6. **Update**: Monarch Money transactions are updated

## API Endpoints Used

### Online Orders
- `GetOnlineOrders()` - Fetches orders from Costco.com
- Returns order details including items, totals, and dates

### In-Store Receipts
- `GetReceipts()` - Fetches warehouse receipt summaries
- `GetReceiptDetail()` - Fetches detailed receipt with items

## Differences from Walmart

| Feature | Walmart | Costco |
|---------|---------|--------|
| Delivery Tips | ✅ Supported | ❌ Not applicable |
| Online Orders | ✅ | ✅ |
| In-Store Receipts | ❌ | ✅ |
| Item-level Details | ✅ Full details | ⚠️ Limited for receipts |
| Rate Limit | 2 seconds | 3 seconds (conservative) |

## Troubleshooting

### Authentication Issues

If you get authentication errors:
1. Verify your email/password are correct
2. Check if Costco requires 2FA (not currently supported)
3. Try logging into Costco.com manually first

### Missing Orders

If orders are missing:
1. Check the date range in your config
2. Verify orders appear on Costco.com
3. Try increasing the `lookback_days` setting

### Rate Limiting

The provider uses a conservative 3-second rate limit. If you encounter issues:
1. Increase the rate limit in the provider
2. Reduce the number of orders fetched at once

## Testing

### Unit Tests

```bash
# Run provider tests
go test ./internal/providers/costco/...
```

### Integration Tests

```bash
# Test with dry run
go run cmd/costco-example/main.go -dry-run

# Test specific date range
go run cmd/costco-example/main.go -days=7 -dry-run
```

## Dashboard Support

The React dashboard automatically supports Costco:
- View Costco orders alongside Walmart orders
- Filter by provider (Walmart/Costco)
- See categorization and splitting results
- Track sync status and errors

## Future Enhancements

- [ ] Support for Costco Business accounts
- [ ] Gas station receipt parsing
- [ ] Pharmacy purchase categorization
- [ ] Return/refund tracking
- [ ] Membership fee handling
- [ ] Executive rewards tracking

## Notes

- The Costco client is in `/Users/erickshaffer/code/costco-go`
- Uses GraphQL API similar to the Costco mobile app
- Requires valid Costco membership
- Some receipt details may be limited compared to online orders

## Support

For issues or questions:
1. Check the logs in `sync_traces.db`
2. Run in dry-run mode first
3. Verify credentials and connectivity
4. Check the dashboard at http://localhost:3000 for sync status