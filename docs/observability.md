# Observability Guide

## Overview

The system now has comprehensive observability features to help you understand exactly what's happening during order processing. This includes detailed tracing, performance metrics, error tracking, and a powerful dashboard for visualization.

## 🚀 Enhanced Dashboard

### Starting the Dashboard

```bash
go run cmd/dashboard-v2/main.go
# Visit http://localhost:8080
```

### Features

#### 1. Real-Time Statistics
- Total orders processed
- Success/failure/skip rates
- Average processing duration
- Currently processing orders
- Per-provider breakdown

#### 2. Trace Viewer
Click on any order to see:
- Complete processing timeline
- Step-by-step execution details
- Duration for each step
- All log messages
- Error details if failed
- Metadata (amounts, confidence scores, item counts)

#### 3. Error Analysis
- Recent errors with full context
- Click to see the complete trace
- Error patterns by provider
- Common failure points

#### 4. Performance Monitoring
- Step-by-step performance metrics
- Min/max/average durations
- Failure rates per step
- Provider-specific performance
- Bottleneck identification

#### 5. Search & Filter
- Search by order ID
- Filter by date range
- Filter by provider
- Filter by status
- Export traces as JSON

## 📊 Tracing System

### How It Works

Every order processing creates a detailed trace that captures:

1. **Trace Metadata**
   - Unique trace ID
   - Provider name
   - Order ID
   - Start/end times
   - Final status
   - Total duration

2. **Processing Steps**
   Each major operation is tracked as a step:
   - Fetching order details
   - Matching with Monarch transaction
   - Categorizing items
   - Creating splits
   - Applying to Monarch

3. **Step Details**
   For each step we capture:
   - Start/end times
   - Duration
   - Success/failure status
   - Relevant data (amounts, counts, etc.)
   - Error messages if failed
   - Log entries during the step

### Example Trace

```json
{
  "id": "abc123",
  "provider": "walmart",
  "order_id": "18420337004257359578",
  "start_time": "2025-01-09T10:30:00Z",
  "end_time": "2025-01-09T10:30:05Z",
  "duration": "5s",
  "status": "success",
  "steps": [
    {
      "name": "FetchOrderDetails",
      "start_time": "2025-01-09T10:30:00Z",
      "end_time": "2025-01-09T10:30:02Z",
      "duration": "2s",
      "status": "success",
      "details": {
        "item_count": 5,
        "order_total": 45.67
      }
    },
    {
      "name": "MatchTransaction",
      "duration": "500ms",
      "status": "success",
      "details": {
        "transaction_id": "monarch_123",
        "confidence": 0.95,
        "amount_diff": 0.50
      }
    },
    {
      "name": "CategorizeItems",
      "duration": "1.5s",
      "status": "success",
      "details": {
        "categories_found": 3,
        "cached_hits": 2
      }
    },
    {
      "name": "CreateSplits",
      "duration": "100ms",
      "status": "success",
      "details": {
        "split_count": 4,
        "includes_tip": true
      }
    },
    {
      "name": "ApplyToMonarch",
      "duration": "900ms",
      "status": "success"
    }
  ],
  "metadata": {
    "order_total": 45.67,
    "transaction_amount": 46.17,
    "match_confidence": 0.95,
    "item_count": 5,
    "split_count": 4,
    "dry_run": false
  }
}
```

## 🔍 Drill-Down Capabilities

### Order History View
See all processing attempts for a specific order:
- Multiple sync attempts
- Success/failure history
- Changes over time
- Retry patterns

### Timeline View
Visual timeline showing:
- When each step started/ended
- Parallel vs sequential operations
- Where time is being spent
- Bottlenecks and delays

### Provider Comparison
Compare performance across providers:
- Success rates
- Average processing times
- Common errors
- Volume differences

## 📈 Metrics & KPIs

### Key Metrics Tracked

1. **Success Metrics**
   - Overall success rate
   - Per-provider success rate
   - First-attempt success rate
   - Retry success rate

2. **Performance Metrics**
   - Average processing time
   - P50/P95/P99 latencies
   - Step-by-step durations
   - API call response times

3. **Volume Metrics**
   - Orders per hour/day
   - Items processed
   - Splits created
   - Categories used

4. **Error Metrics**
   - Error rate by type
   - Most common failures
   - Provider-specific errors
   - Recovery success rate

## 🛠️ Using Traces for Debugging

### Common Scenarios

#### 1. "Why did this order fail?"
- Go to dashboard
- Search for the order ID
- Click to view trace
- See exact error and where it occurred
- Check step details for context

#### 2. "Why is processing slow?"
- Go to Performance tab
- Look at step durations
- Identify slowest steps
- Check if it's API calls or processing

#### 3. "Are we having issues with a provider?"
- Check provider statistics
- Compare success rates
- Look at recent errors
- Check for patterns

#### 4. "What happened during a specific time?"
- Use date range search
- Filter by time period
- Review all traces
- Look for anomalies

### Trace Export

You can export any trace as JSON for:
- Sharing with team
- Further analysis
- Bug reports
- Audit trail

## 🔄 Integration with Processing

The tracing system is automatically integrated with order processing:

```go
// Traces are created automatically
ctx := context.Background()
trace := traceStore.StartTrace("walmart", orderID)
ctx = observability.ContextWithTrace(ctx, trace)

// Steps are tracked
trace.StartStep("FetchOrderDetails")
// ... do work ...
trace.CompleteStep("FetchOrderDetails", true, details)

// Logs are captured
trace.AddLog("info", "Matched transaction", fields)

// Completion is recorded
trace.Complete("success", nil)
```

## 📊 Dashboard Tabs Explained

### Recent Traces Tab
- Live view of processing
- Color-coded by status
- Click for details
- Auto-refreshes every 30s

### Errors Tab
- Recent failures only
- Full error messages
- Direct link to trace
- Grouped by type

### Performance Tab
- Table of all processing steps
- Statistical analysis
- Performance trends
- Bottleneck identification

### Search Tab
- Find specific orders
- Date range queries
- Advanced filtering
- Bulk export

## 🎯 Benefits

1. **Complete Visibility**
   - See everything that happens
   - No more black box processing
   - Full audit trail

2. **Fast Debugging**
   - Quickly identify issues
   - See exact failure points
   - Understand error context

3. **Performance Optimization**
   - Identify slow operations
   - Track improvements
   - Compare providers

4. **Business Intelligence**
   - Processing statistics
   - Success metrics
   - Volume tracking
   - Trend analysis

## 🔮 Future Enhancements

Planned improvements:
- Prometheus metrics export
- Grafana dashboards
- Alert system for failures
- Distributed tracing with OpenTelemetry
- Performance regression detection
- Automatic retry visualization
- Cost tracking per provider

## 💡 Tips

1. **Keep the dashboard open** during sync runs to watch real-time progress
2. **Use filters** to focus on specific providers or statuses
3. **Export interesting traces** for later analysis
4. **Check performance tab** regularly to catch degradation early
5. **Review error patterns** to identify systematic issues

## 🚨 Troubleshooting

### Dashboard won't load
- Check if database exists: `ls processing.db`
- Verify port 8080 is available
- Check console for errors

### No traces showing
- Traces are in-memory and expire after 24 hours
- Run a sync to generate new traces
- Check if database has historical data

### Performance issues
- Dashboard auto-refreshes every 30s
- Large trace counts may slow loading
- Use filters to reduce data
- Clear old traces from database