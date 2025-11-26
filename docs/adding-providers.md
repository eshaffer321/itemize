# Adding New Providers

This guide explains how to add support for new stores (like Costco, Target, Amazon) to the Monarch Money Sync Backend.

## Overview

The system uses a provider abstraction pattern that makes it easy to add new stores. Each provider implements a common interface, allowing the core processing logic to work with any store.

## Step-by-Step Guide

### 1. Create Your Store's SDK

First, you need an SDK that can fetch order data from your store. This is typically a separate repository.

Example structure for a Costco client:
```go
// github.com/yourusername/costco-api/costco-client/client.go
type CostcoClient struct {
    // Client implementation
}

func (c *CostcoClient) GetOrders() ([]Order, error) {
    // Fetch orders from Costco
}
```

### 2. Implement the Provider Interface

Create a new provider package in `internal/providers/[storename]/`:

```go
// internal/providers/costco/provider.go
package costco

import (
    "context"
    "time"
    
    "github.com/eshaffer321/monarchmoney-sync-backend/internal/providers"
    costcoclient "github.com/yourusername/costco-api/costco-client"
)

type Provider struct {
    client    *costcoclient.CostcoClient
    logger    *slog.Logger
    rateLimit time.Duration
}

func NewProvider(client *costcoclient.CostcoClient, logger *slog.Logger) *Provider {
    return &Provider{
        client:    client,
        logger:    logger.With(slog.String("provider", "costco")),
        rateLimit: 1 * time.Second,
    }
}

// Implement all required methods:
func (p *Provider) Name() string { return "costco" }
func (p *Provider) DisplayName() string { return "Costco" }
func (p *Provider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) { ... }
func (p *Provider) GetOrderDetails(ctx context.Context, orderID string) (providers.Order, error) { ... }
func (p *Provider) SupportsDeliveryTips() bool { return false }
func (p *Provider) SupportsRefunds() bool { return true }
func (p *Provider) SupportsBulkFetch() bool { return true }
func (p *Provider) GetRateLimit() time.Duration { return p.rateLimit }
func (p *Provider) HealthCheck(ctx context.Context) error { ... }
```

### 3. Implement Order and OrderItem Adapters

Create adapters that convert your store's data structures to the common interfaces:

```go
// internal/providers/costco/order.go
type CostcoOrder struct {
    id           string
    date         time.Time
    total        float64
    subtotal     float64
    tax          float64
    items        []providers.OrderItem
    rawOrder     *costcoclient.Order
}

func (o *CostcoOrder) GetID() string { return o.id }
func (o *CostcoOrder) GetDate() time.Time { return o.date }
func (o *CostcoOrder) GetTotal() float64 { return o.total }
// ... implement all Order interface methods

type CostcoOrderItem struct {
    item *costcoclient.Item
}

func (i *CostcoOrderItem) GetName() string { return i.item.Description }
func (i *CostcoOrderItem) GetPrice() float64 { return i.item.Price * float64(i.item.Quantity) }
// ... implement all OrderItem interface methods
```

### 4. Register the Provider

Update the main application to register your provider:

```go
// cmd/sync/main.go (or wherever providers are registered)

import (
    "github.com/eshaffer321/monarchmoney-sync-backend/internal/providers/costco"
    costcoclient "github.com/yourusername/costco-api/costco-client"
)

func main() {
    // ... existing code ...
    
    // Register providers
    registry := providers.NewRegistry(logger)
    
    // Register Walmart (existing)
    if cfg.Providers.Walmart.Enabled {
        walmartClient := walmartclient.NewClient()
        walmartProvider := walmart.NewProvider(walmartClient, logger)
        registry.Register(walmartProvider)
    }
    
    // Register Costco (new)
    if cfg.Providers.Costco.Enabled {
        costcoClient := costcoclient.NewClient()
        costcoProvider := costco.NewProvider(costcoClient, logger)
        registry.Register(costcoProvider)
    }
}
```

### 5. Update Configuration

Add your provider to the configuration:

```yaml
# config.yaml
providers:
  costco:
    enabled: true
    rate_limit: 1s
    lookback_days: 30
    max_orders: 0
```

### 6. Handle Provider-Specific Details

Each provider may have unique characteristics:

#### Authentication
- **Cookies**: Like Walmart, may need browser cookies
- **API Keys**: Some stores provide official APIs
- **OAuth**: Some require OAuth flows

#### Data Differences
- **Membership Fees**: Costco has annual fees
- **Bulk Items**: Different quantity handling
- **Returns**: Some providers track returns differently
- **Categories**: Store-specific category names

#### Transaction Matching
Different stores may appear differently in bank transactions:
- Walmart: "WALMART", "WAL-MART", "WALMART.COM"
- Costco: "COSTCO", "COSTCO WHSE", "COSTCO.COM"
- Target: "TARGET", "TGT", "TARGET.COM"

Update the matching logic if needed:
```go
// internal/processor/processor.go
func containsProvider(merchant, provider string) bool {
    switch provider {
    case "costco":
        return strings.Contains(merchant, "COSTCO")
    case "target":
        return strings.Contains(merchant, "TARGET") || strings.Contains(merchant, "TGT")
    default:
        return strings.Contains(merchant, strings.ToUpper(provider))
    }
}
```

## Example: Adding Costco Support

Here's a complete example of what you'd need to add Costco support:

### 1. Directory Structure
```
internal/providers/costco/
├── provider.go      # Main provider implementation
├── order.go         # Order/Item adapters
└── provider_test.go # Tests
```

### 2. Key Implementation Points

```go
// Special handling for Costco membership fees
func (p *Provider) FetchOrders(ctx context.Context, opts providers.FetchOptions) ([]providers.Order, error) {
    orders, err := p.client.GetOrders()
    if err != nil {
        return nil, err
    }
    
    var result []providers.Order
    for _, order := range orders {
        // Filter out membership renewal "orders"
        if order.Type == "MEMBERSHIP_RENEWAL" {
            continue
        }
        
        // Convert to common format
        result = append(result, p.convertOrder(order))
    }
    
    return result, nil
}

// Handle bulk quantities
type CostcoOrderItem struct {
    item *costcoclient.Item
}

func (i *CostcoOrderItem) GetName() string {
    // Include pack size in name for clarity
    if i.item.PackSize > 1 {
        return fmt.Sprintf("%s (Pack of %d)", i.item.Description, i.item.PackSize)
    }
    return i.item.Description
}
```

## Testing Your Provider

1. **Unit Tests**: Test the provider in isolation
```go
func TestCostcoProvider_FetchOrders(t *testing.T) {
    // Mock the Costco client
    // Test order fetching
    // Verify conversion to common format
}
```

2. **Integration Tests**: Test with real data (if available)
```go
func TestCostcoProvider_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    // Test with real Costco API
}
```

3. **Manual Testing**: Test the full flow
```bash
# Run with just your provider
go run cmd/sync/main.go --provider=costco --dry-run
```

## Common Pitfalls

1. **Date Parsing**: Different stores use different date formats
2. **Price Calculation**: Some stores include tax in item prices, others don't
3. **Quantity Handling**: Decimal vs integer quantities
4. **Rate Limiting**: Respect each store's API limits
5. **Error Handling**: Some stores have flaky APIs, add retries

## Provider Capabilities

Different providers support different features:

| Provider | Delivery Tips | Refunds | Bulk Fetch | Categories |
|----------|--------------|---------|------------|------------|
| Walmart  | ✅           | ❌      | ✅         | ❌         |
| Costco   | ❌           | ✅      | ✅         | ✅         |
| Target   | ✅           | ✅      | ✅         | ✅         |
| Amazon   | ❌           | ✅      | ❌         | ✅         |

## Observability

Your provider will automatically benefit from the structured logging:

```go
// Logs will include provider context
logger.Info("processing order",
    slog.String("provider", "costco"),
    slog.String("order_id", order.ID),
    slog.Float64("amount", order.Total),
)
```

## Checklist for New Provider

- [ ] Create store SDK/client (separate repo)
- [ ] Implement `OrderProvider` interface
- [ ] Create `Order` and `OrderItem` adapters
- [ ] Handle provider-specific quirks
- [ ] Add configuration section
- [ ] Register in main application
- [ ] Write unit tests
- [ ] Test with real data
- [ ] Update transaction matching logic
- [ ] Document any special requirements
- [ ] Add to provider capabilities table

## Future Enhancements

As you add providers, consider:

1. **Provider Plugins**: Load providers dynamically
2. **Provider Discovery**: Auto-detect available providers
3. **Provider Health Dashboard**: Monitor all providers
4. **Unified Authentication**: Handle various auth methods
5. **Provider-Specific Rules**: Custom categorization per store

## Need Help?

If you're adding a new provider and run into issues:

1. Check existing providers for examples
2. Review the test files for patterns
3. Enable debug logging for detailed output
4. File an issue with your specific challenge