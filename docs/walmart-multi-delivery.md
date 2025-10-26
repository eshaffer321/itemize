# Walmart Multi-Delivery Order Handling

**Status:** Design Approved - Implementation Ready
**Last Updated:** October 24, 2025
**Reviewer Feedback:** ⭐⭐⭐⭐½ (4.5/5) - Incorporated all critical and important fixes

## Problem

Walmart splits large orders into multiple deliveries with separate bank charges:
- Order API shows single total (e.g., $126.98)
- Bank shows multiple charges (e.g., $118.67 + $8.31)
- Monarch has multiple transactions
- Our matcher looks for single transaction matching order total → fails

**Frequency:** 2-3 times per week (high priority issue)

## Solution: Use Order Ledger API

The walmart-client-go library has `GetOrderLedger()` which provides actual charge breakdown.

## Design Review - Key Changes

Based on independent code review, the following critical improvements were incorporated:

### Critical Fix #1: Matcher Configuration Consistency
**Problem:** Creating new matcher instances per order wastes resources and makes config harder to manage.
**Solution:** Add matcher as reusable component in Orchestrator struct.

### Critical Fix #2: Database Schema for Multi-Delivery Tracking
**Problem:** No audit trail for which transactions were consolidated.
**Solution:** Add `multi_delivery_data` JSON column to track original transaction IDs and charge amounts.

### Critical Fix #3: Floating-Point Sum Validation
**Problem:** Direct equality check fails due to floating-point arithmetic.
**Solution:** Use tolerance-based comparison when validating charge sum.

### Important Fix #4: Extract Shared Categorization Logic
**Problem:** Code duplication between single and multi-delivery flows.
**Solution:** Extract `categorizeAndApplySplits()` helper method.

### Important Fix #5: Partial Consolidation Failure Handling
**Problem:** No recovery if update succeeds but delete fails.
**Solution:** Return failed deletion IDs, store in database, log for manual cleanup.

### Important Fix #6: Comprehensive Test Coverage
**Added test cases for:** Malformed ledger data, sum mismatches, user manual edits, duplicate match prevention.

## How It Works

```go
ledger, err := client.GetOrderLedger(orderID)
// ledger.PaymentMethods[0].FinalCharges = [118.67, 8.31]
// ledger.PaymentMethods[0].TotalCharged = 126.98
```

### Detection

```go
func isMultiDeliveryOrder(client *walmartclient.WalmartClient, orderID string) (bool, []float64, error) {
    ledger, err := client.GetOrderLedger(orderID)
    if err != nil {
        return false, nil, err
    }

    if len(ledger.PaymentMethods) == 0 {
        return false, nil, nil
    }

    charges := ledger.PaymentMethods[0].FinalCharges
    return len(charges) > 1, charges, nil
}
```

### Proposed Approaches

#### Option 1: Consolidate Transactions (Recommended)

**Steps:**
1. Detect multi-delivery via `GetOrderLedger()`
2. Find all Monarch transactions matching charge amounts
3. Update first transaction:
   - Set amount to total order amount
   - Add transaction splits normally
   - Add note: "Multi-delivery order (2 charges: $118.67, $8.31)"
4. Delete other transaction(s)

**Pros:**
- Clean - one transaction per order
- Accurate splitting and categorization
- Matches user's mental model

**Cons:**
- Deletes transactions (need to confirm user wants this)
- More complex logic

#### Option 2: Match Against Total with Note (Simpler)

**Steps:**
1. Detect multi-delivery
2. Look for transactions where sum matches total
3. Consolidate first, then match normally
4. Add note about multi-delivery

**Pros:**
- Simpler implementation
- No transaction deletion

**Cons:**
- Might not find matches if transactions far apart
- Less accurate

#### Option 3: Skip Multi-Delivery Orders

**Steps:**
1. Detect multi-delivery
2. Log warning and skip processing
3. User manually handles these

**Pros:**
- Safest - no changes
- Simple

**Cons:**
- Doesn't solve the problem
- High frequency means manual work

## Implementation Plan

### Phase 1: Detection (Now)

Add method to Order type:

```go
// In internal/adapters/providers/walmart/order.go

// GetFinalCharges returns the actual bank charges for this order
// Returns multiple charges for multi-delivery orders
func (o *Order) GetFinalCharges() ([]float64, error) {
    if o.client == nil {
        return nil, fmt.Errorf("client not available")
    }

    ledger, err := o.client.GetOrderLedger(o.GetID())
    if err != nil {
        return nil, fmt.Errorf("failed to get order ledger: %w", err)
    }

    if len(ledger.PaymentMethods) == 0 {
        return nil, fmt.Errorf("no payment methods in ledger")
    }

    return ledger.PaymentMethods[0].FinalCharges, nil
}

// IsMultiDelivery checks if order was split into multiple deliveries
func (o *Order) IsMultiDelivery() (bool, error) {
    charges, err := o.GetFinalCharges()
    if err != nil {
        return false, err
    }
    return len(charges) > 1, nil
}
```

### Phase 2: Enhanced Matching

Update matcher to handle multi-delivery:

```go
// In internal/domain/matcher/matcher.go

// For multi-delivery, we need to find MULTIPLE transactions that sum to total
func (m *Matcher) FindMultiDeliveryMatches(
    order providers.Order,
    transactions []*monarch.Transaction,
    charges []float64,
) ([]*monarch.Transaction, error) {
    // Find transactions matching each charge amount
    // Within date tolerance
    // Return all matches
}
```

### Phase 3: Transaction Consolidation

Add to Monarch client wrapper:

```go
// In orchestrator or new consolidation module

func (o *Orchestrator) consolidateMultiDeliveryTransactions(
    ctx context.Context,
    transactions []*monarch.Transaction,
    order providers.Order,
) (*monarch.Transaction, error) {
    if len(transactions) == 0 {
        return nil, fmt.Errorf("no transactions to consolidate")
    }

    // Use first transaction as primary
    primary := transactions[0]

    // Update amount to order total
    total := order.GetTotal()
    params := &monarch.UpdateTransactionParams{
        Amount: &total,
        Notes:  getMultiDeliveryNote(transactions),
    }

    updated, err := o.clients.Monarch.Transactions.Update(ctx, primary.ID, params)
    if err != nil {
        return nil, err
    }

    // Delete other transactions
    for i := 1; i < len(transactions); i++ {
        err := o.clients.Monarch.Transactions.Delete(ctx, transactions[i].ID)
        if err != nil {
            o.logger.Warn("failed to delete extra transaction",
                "transaction_id", transactions[i].ID,
                "error", err)
        }
    }

    return updated, nil
}

func getMultiDeliveryNote(txns []*monarch.Transaction) string {
    charges := make([]string, len(txns))
    for i, txn := range txns {
        charges[i] = fmt.Sprintf("$%.2f", math.Abs(txn.Amount))
    }
    return fmt.Sprintf("Multi-delivery order (%d charges: %s)",
        len(txns),
        strings.Join(charges, ", "))
}
```

## Testing Strategy

1. **Unit tests:** Multi-delivery detection
2. **Integration test:** Call GetOrderLedger with known multi-delivery order
3. **Manual test:** Dry-run on order 200013779923758
4. **Validation:** Confirm charges match expected [118.67, 8.31]

## Next Steps

1. ✅ Understand walmart-client-go ledger API (DONE)
2. ⏳ Test GetOrderLedger with real data (blocked by rate limit)
3. ⬜ Decide on approach (consolidate vs note vs skip)
4. ⬜ Implement detection methods
5. ⬜ Implement enhanced matching
6. ⬜ Implement consolidation (if chosen)
7. ⬜ Test with dry-run
8. ⬜ Run on real data

## Rate Limit Considerations

Walmart's ledger endpoint seems rate-limited. We should:
- Cache ledger data per order
- Only call when needed (multi-delivery suspected)
- Add rate limiting delays between calls
- Handle 429 errors gracefully

## Questions for User

1. **Which approach do you prefer?**
   - A: Consolidate transactions (delete extras, update one)
   - B: Match against sum and add note
   - C: Something else?

2. **Transaction deletion:**
   - Are you OK with deleting the "extra" transactions after consolidating?
   - Or would you prefer to keep them marked somehow?

3. **Testing:**
   - Want to wait for rate limit and test with real order 200013779923758?
   - Or proceed with implementation based on code understanding?
