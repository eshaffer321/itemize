# Monarch Money Sync Backend

## Project Vision
A unified CLI application that automatically syncs purchases from multiple retailers (Walmart, Costco) with Monarch Money, intelligently categorizing and splitting transactions based on individual items purchased.

## Development Methodology: Test-Driven Development (TDD)

### Core TDD Workflow
1. **Write the test first** - Define expected behavior
2. **Run test and watch it fail** - Verify test is actually testing something
3. **Write minimal code** - Just enough to make test pass
4. **Run test and watch it pass** - Verify implementation works
5. **Refactor** - Clean up while keeping tests green

### Bug Fixing Process (MANDATORY)
When ANY bug is discovered:
1. **STOP** - Don't fix the bug yet
2. **Write a failing test** - Reproduce the bug in a test
3. **Run the test** - Confirm it fails for the right reason
4. **Fix the bug** - Implement the minimum fix
5. **Run the test** - Confirm it now passes
6. **Run all tests** - Ensure no regression
7. **Document** - Add to `/docs/bug-fixes.md` with test case

### Documentation Structure
```
/docs/
├── progress.md          # Current development status (UPDATE CONTINUOUSLY)
├── api.md              # API documentation
├── testing.md          # Testing strategy and guidelines
├── bug-fixes.md        # Log of bugs and their test cases
├── architecture.md     # System design decisions
└── setup.md           # Development environment setup
```

### Progress Tracking
**CRITICAL**: Maintain `/docs/progress.md` with:
- Current task being worked on
- Tests written today
- Tests passing/failing
- Next steps
- Blockers or questions
- Handoff notes for next session

Example entry:
```markdown
## 2024-01-15 Session
### Completed
- [x] Created health check endpoint
- [x] Tests: TestHealthCheck (passing)
- [x] Created order validation
- [x] Tests: TestOrderValidation (passing)

### In Progress
- [ ] Monarch API integration
- [ ] Test: TestMonarchConnection (written, failing)

### Next Steps
1. Fix authentication in Monarch client
2. Complete TestMonarchConnection
3. Write TestTransactionMatching

### Notes for Next Session
- MonarchClient.Connect() returns 401, check API key format
- See test file: handlers/walmart_test.go line 45
```

## Ultimate Goal
Transform single Walmart transactions in Monarch Money into properly categorized, split transactions that accurately reflect what was purchased. For example, a $150 Walmart transaction becomes:
- $50 - Groceries (milk, bread, eggs)
- $30 - Home & Garden (cleaning supplies)
- $40 - Electronics (phone charger, batteries)
- $30 - Personal Care (shampoo, toothpaste)

## Architecture Overview
```
CLI Application → Provider APIs → LLM API → Monarch Money API
     ↓                ↓            ↓            ↓
Sync Orders      Process      Categorize    Split & Update
from Retailers    Orders        Items        Transactions
```

## CLI Structure
The application uses a unified CLI with subcommands:

```bash
monarch-sync <command> [options]

# Provider-specific commands
monarch-sync costco sync
monarch-sync costco dry-run
monarch-sync walmart sync

# General commands  
monarch-sync sync
monarch-sync api
monarch-sync audit
monarch-sync consolidate
monarch-sync enrich
```

## Phase 1: Basic Sync (MVP)
**Goal**: Get retailer order data and match with Monarch transactions

### CLI Application
- [x] Unified CLI structure with subcommands
- [x] Provider-specific commands (costco, walmart)
- [x] General sync command for all providers
- [x] Configuration management
- [x] Database consolidation

### Provider Integration
- [x] Costco provider with sync and dry-run modes
- [x] Walmart provider integration
- [x] Use `github.com/eshaffer321/monarchmoney-go` SDK
- [x] Match orders with Monarch transactions by date/amount
- [x] Update transaction notes with item details

### Deliverables
- Working CLI that syncs retailer data
- Unified database for all processing history
- Basic matching with Monarch transactions
- Audit reporting and data consolidation

## Phase 2: Intelligent Categorization
**Goal**: Use LLM to categorize retailer items

### Features
- [x] Integrate with OpenAI API for categorization
- [x] Fetch Monarch categories via SDK
- [x] Create mapping between retailer items and Monarch categories
- [x] Cache categorization decisions for common items

### Example Flow
1. Receive: "Great Value Milk 1 Gallon - $3.99"
2. LLM determines: Category = "Groceries"
3. Cache: "Great Value Milk" → "Groceries" for future use

## Phase 3: Transaction Splitting
**Goal**: Split single retailer transactions into multiple categorized transactions

### Features
- [x] Use Monarch SDK to split transactions
- [x] Group items by category
- [x] Create split transactions with proper categories
- [x] Maintain audit trail of original transaction

### Example
Original Transaction: Walmart - $150.00
Becomes:
```
- Walmart (Groceries) - $50.00
- Walmart (Home & Garden) - $30.00
- Walmart (Electronics) - $40.00
- Walmart (Personal Care) - $30.00
```

## Phase 4: Advanced Features
- [ ] Bulk historical import
- [ ] Recurring purchase detection
- [ ] Budget impact analysis
- [ ] Category spending trends
- [ ] Manual override/training UI

## Technical Stack

### Chrome Extension
- Manifest V3
- Background service worker for API calls
- Local storage for caching
- No external dependencies

### Go Backend
- **Framework**: Unified CLI application
- **Monarch SDK**: `github.com/eshaffer321/monarchmoney-go`
- **LLM**: OpenAI Go SDK for categorization
- **Database**: SQLite for processing history and audit trail
- **Cache**: In-memory caching for category mappings

### APIs & Services
- Walmart.com (via provider client)
- Costco.com (via provider client)
- Monarch Money API (via SDK)
- OpenAI API (for categorization)

## Data Flow

### 1. Order Collection
```json
{
  "orderNumber": "123456789",
  "orderDate": "2024-01-15",
  "orderTotal": 150.00,
  "items": [
    {
      "name": "Great Value Milk",
      "price": 3.99,
      "quantity": 1,
      "category": null  // To be determined
    }
  ]
}
```

### 2. Categorization Request to LLM
```json
{
  "items": ["Great Value Milk", "Bounty Paper Towels"],
  "availableCategories": ["Groceries", "Home & Garden", "Personal Care"]
}
```

### 3. Monarch Transaction Update
```json
{
  "transactionId": "monarch_123",
  "splits": [
    {
      "amount": 50.00,
      "category": "Groceries",
      "notes": "Milk, Bread, Eggs"
    }
  ]
}
```

## Database Schema (Future)

### cached_categories
```sql
CREATE TABLE cached_categories (
  item_name TEXT PRIMARY KEY,
  category TEXT NOT NULL,
  confidence FLOAT,
  last_updated TIMESTAMP
);
```

### transaction_audit
```sql
CREATE TABLE transaction_audit (
  id SERIAL PRIMARY KEY,
  walmart_order_id TEXT,
  monarch_transaction_id TEXT,
  original_amount DECIMAL,
  splits JSONB,
  processed_at TIMESTAMP
);
```

## Environment Variables
```bash
# Server
PORT=8080
MONARCH_API_KEY=xxx
EXTENSION_SECRET_KEY=shared_secret

# Future
OPENAI_API_KEY=xxx
DATABASE_URL=postgresql://...
REDIS_URL=redis://...
```

## API Endpoints

### Phase 1
- `GET /health` - Health check
- `POST /api/walmart/orders` - Receive orders from extension

### Phase 2
- `GET /api/categories` - List Monarch categories
- `POST /api/categorize` - Categorize items

### Phase 3
- `POST /api/transactions/split` - Split transaction
- `GET /api/transactions/{id}/audit` - Get split history

## Security Considerations
- Extension ↔ Backend: Shared secret key
- Backend → Monarch: OAuth or API key (via SDK)
- Rate limiting on all endpoints
- No PII in logs
- Encrypted storage for sensitive data

## Development Phases

### Current Focus: Phase 1
1. Complete Chrome extension order fetching ✓
2. Create basic Go server with Gin
3. Integrate monarchmoney-go SDK
4. Implement basic matching logic
5. Test end-to-end flow

### Next Steps
- Set up project structure
- Implement health check endpoint
- Create order receive endpoint
- Add Monarch SDK integration
- Build matching algorithm

## Success Metrics
- Phase 1: Successfully match 90% of Walmart transactions
- Phase 2: Accurately categorize 85% of items
- Phase 3: Split transactions with 95% accuracy
- Phase 4: Reduce manual categorization by 80%

## Commands for Development

```bash
# Build the unified CLI
go build -o monarch-sync ./cmd/monarch-sync/

# Use the CLI
./monarch-sync costco sync
./monarch-sync costco dry-run
./monarch-sync walmart sync
./monarch-sync sync -config config.yaml
./monarch-sync api
./monarch-sync audit
./monarch-sync consolidate

# Development workflow
go test ./... -v              # Run all tests
go test ./internal/... -v     # Run specific package tests
go test -run TestCostcoSync   # Run specific test
go test ./... -cover          # Check test coverage
go test ./... -race           # Check for race conditions

# Run with config
./monarch-sync sync -config config.yaml -verbose
```

## TDD Test Structure Example

```go
// handlers/walmart_test.go
func TestProcessWalmartOrder(t *testing.T) {
    // Arrange
    order := models.Order{
        OrderNumber: "123",
        Total: 150.00,
    }
    
    // Act
    result := ProcessOrder(order)
    
    // Assert
    assert.NotNil(t, result)
    assert.Equal(t, "processed", result.Status)
}
```

## Notes for Claude/AI Assistant

### MANDATORY TDD APPROACH
1. **ALWAYS write tests first** - No exceptions
2. **Every feature starts with a failing test**
3. **Bug fixes MUST have a test that reproduces the bug first**
4. **Update `/docs/progress.md` after EVERY work session**
5. **Document test decisions in `/docs/testing.md`**

### Development Guidelines
- Start with Phase 1 - get basic sync working
- Focus on unified CLI application
- Use the monarchmoney-go SDK for all Monarch interactions
- Keep LLM integration simple initially (can be added later)
- Prioritize working MVP over perfect architecture
- Test with real retailer data early and often
- Maintain 80%+ test coverage
- All tests must pass before committing code
- Use single binary deployment model

### Session Handoff Protocol
Before ending any session:
1. Run all tests and document results
2. Update `/docs/progress.md` with:
   - What was completed
   - What tests were written
   - Current blockers
   - Next steps clearly defined
3. Commit with descriptive message including test status
4. Leave breadcrumbs for the next session

## Questions to Resolve
1. How to handle partial matches (order split across multiple Monarch transactions)?
2. Category mapping strategy - user-defined rules vs pure LLM?
3. How to handle returns/refunds?
4. Frequency of sync - real-time vs scheduled?
5. Multi-provider sync coordination strategy?

---

This document will evolve as the project develops. Start with Phase 1 and iterate!
