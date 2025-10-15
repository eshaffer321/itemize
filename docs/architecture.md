# Architecture

**Last Updated:** October 2024
**Status:** Current implementation

## Overview

The Monarch Money Sync Backend follows a **layered architecture** pattern with clear separation of concerns. This structure emerged from refactoring a flat directory layout that made it difficult to understand where components belonged.

## Directory Structure

```
internal/
├── application/         # Workflow orchestration
│   └── sync/           # Main sync orchestrator
├── domain/             # Business logic (pure functions)
│   ├── categorizer/    # AI-powered categorization
│   ├── matcher/        # Transaction matching
│   └── splitter/       # Transaction splitting
├── adapters/           # External system integrations
│   ├── providers/      # Retailer APIs
│   │   ├── costco/     # Costco implementation
│   │   ├── walmart/    # Walmart implementation
│   │   └── types.go    # Common interfaces
│   └── clients/        # API client builders
├── infrastructure/     # Technical foundations
│   ├── config/         # Configuration management
│   ├── storage/        # SQLite persistence
│   └── logging/        # Structured logging
└── cli/                # Command-line interface
```

## Layers

### 1. Application Layer (`internal/application/`)

**Purpose:** Orchestrates the sync workflow by coordinating between domain logic, adapters, and infrastructure.

**Key Component:**
- `sync/orchestrator.go` - Main workflow coordinator

**Responsibilities:**
- Coordinate order fetching
- Match orders with transactions
- Categorize items
- Create transaction splits
- Handle errors and logging

**Dependencies:** Domain, Adapters, Infrastructure, CLI

### 2. Domain Layer (`internal/domain/`)

**Purpose:** Contains pure business logic with no external dependencies.

**Components:**

- **`categorizer/`** - AI-powered item categorization
  - `categorizer.go` - Core categorization logic
  - `openai_client.go` - OpenAI API integration
  - `cache.go` - In-memory category cache

- **`matcher/`** - Fuzzy transaction matching
  - `matcher.go` - Matching algorithm
  - Handles amount tolerance, date ranges, confidence scoring

- **`splitter/`** - Transaction split creation
  - `splitter.go` - Split logic with tax distribution
  - Groups items by category

**Key Principle:** Domain layer has **NO external dependencies**. It works with interfaces (like `providers.Order`) but doesn't know about HTTP, databases, or specific retailer implementations.

**Dependencies:** None (pure logic)

### 3. Adapters Layer (`internal/adapters/`)

**Purpose:** Integrations with external systems.

**Components:**

- **`providers/`** - Retailer order fetching
  - `types.go` - `OrderProvider` interface definition
  - `costco/` - Costco API implementation
  - `walmart/` - Walmart API implementation

- **`clients/`** - API client initialization
  - `clients.go` - Builds Monarch and OpenAI clients with config

**Responsibilities:**
- Fetch orders from retailer APIs
- Provide uniform interface to application layer
- Handle provider-specific authentication and data formats

**Dependencies:** Infrastructure (for config, logging)

### 4. Infrastructure Layer (`internal/infrastructure/`)

**Purpose:** Technical foundations needed by the application.

**Components:**

- **`config/`** - Configuration management
  - Loads from YAML or environment variables
  - Supports multiple API key variants (`OPENAI_API_KEY` or `OPENAI_APIKEY`)

- **`storage/`** - SQLite persistence
  - Processing history
  - Duplicate prevention
  - Audit trail
  - Automatic schema migration

- **`logging/`** - Structured logging
  - slog wrapper
  - JSON or text format

**Dependencies:** None (leaf nodes)

### 5. CLI Layer (`internal/cli/`)

**Purpose:** User interface and command parsing.

**Components:**
- `flags.go` - Command-line flag parsing
- `output.go` - User-facing output formatting
- `providers.go` - Provider initialization from config

**Dependencies:** Application layer (entry point)

## Dependency Flow

```
┌──────────────────────────────────────────────────┐
│                  CLI Layer                        │
│           (internal/cli/)                         │
└────────────────────┬─────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────┐
│            Application Layer                      │
│       (internal/application/sync/)                │
└─────┬──────────┬────────────┬──────────┬─────────┘
      │          │            │          │
      ▼          ▼            ▼          ▼
┌─────────┐ ┌────────┐  ┌──────────┐ ┌──────────┐
│ Domain  │ │Adapters│  │Infrastructure│ │   CLI    │
│ (pure)  │ │        │  │          │ │          │
└─────────┘ └────┬───┘  └──────────┘ └──────────┘
                 │
                 └──────> Infrastructure (for config)
```

### Key Principles:

1. **Domain has no dependencies** - Pure business logic
2. **Application coordinates** - Orchestrates workflow
3. **Adapters talk to external systems** - Can use infrastructure
4. **Infrastructure is foundational** - No dependencies
5. **CLI is the entry point** - Uses application layer

## Data Flow

### Example: Syncing a Walmart Order

```
1. CLI parses flags
   └─> internal/cli/flags.go

2. CLI creates provider
   └─> internal/cli/providers.go
       └─> adapters/providers/walmart/

3. CLI calls orchestrator
   └─> application/sync/orchestrator.go

4. Orchestrator fetches orders
   └─> adapters/providers/walmart/provider.go
       └─> walmart-api client (external package)

5. Orchestrator matches orders
   └─> domain/matcher/matcher.go
       └─> Uses Monarch transactions (via clients)

6. Orchestrator categorizes items
   └─> domain/categorizer/categorizer.go
       └─> Calls OpenAI API (via clients)
       └─> Checks cache first

7. Orchestrator creates splits
   └─> domain/splitter/splitter.go
       └─> Groups items by category
       └─> Distributes tax proportionally

8. Orchestrator updates Monarch
   └─> adapters/clients/clients.go (Monarch client)
       └─> monarchmoney-go SDK (external package)

9. Orchestrator saves to database
   └─> infrastructure/storage/storage.go
       └─> SQLite persistence
```

## Adding a New Provider

See [adding-providers.md](adding-providers.md) for detailed guide.

Quick steps:
1. Create `internal/adapters/providers/newprovider/`
2. Implement `OrderProvider` interface from `types.go`
3. Add config in `internal/infrastructure/config/config.go`
4. Register in `internal/cli/providers.go`

## Design Decisions

### Why Layered Architecture?

The original flat structure (`internal/categorizer/`, `internal/matcher/`, `internal/sync/`, etc.) made it unclear:
- Where new features should go
- What depends on what
- Which components can be tested in isolation

The layered structure provides:
- **Clear mental model** - "This is business logic" vs "This talks to external APIs"
- **Intuitive placement** - Easy to know where new code belongs
- **Testability** - Domain layer has no dependencies, easy to test
- **Maintainability** - Clear boundaries prevent tangled dependencies

### Why Not Hexagonal/Ports & Adapters?

We considered full hexagonal architecture but chose simpler layered approach because:
- Smaller codebase doesn't need full hexagonal complexity
- Layered architecture is more familiar to most developers
- Still achieves main goals (testability, clear boundaries)
- Can evolve to hexagonal later if needed

### Interface-Driven Design

Even though we don't use full hexagonal, we extensively use interfaces:
- `OrderProvider` - Allows any retailer implementation
- `Order` / `OrderItem` - Uniform order representation
- `OpenAIClient` - Allows mocking in tests
- `Cache` - Allows different cache implementations

This provides flexibility without full hexagonal overhead.

## Migration History

**October 2024:** Refactored from flat structure to layered architecture.
- Moved `internal/categorizer/` → `internal/domain/categorizer/`
- Moved `internal/matcher/` → `internal/domain/matcher/`
- Moved `internal/splitter/` → `internal/domain/splitter/`
- Moved `internal/sync/` → `internal/application/sync/`
- Moved `internal/observability/` → `internal/infrastructure/logging/`
- Moved `internal/storage/` → `internal/infrastructure/storage/`
- Moved `internal/config/` → `internal/infrastructure/config/`
- Created `internal/adapters/` for external integrations
- Moved `internal/providers/` → `internal/adapters/providers/`
- Moved `internal/clients/` → `internal/adapters/clients/`
- Moved CLI from `cmd/monarch-sync/cli/` → `internal/cli/`

**Key Improvements:**
- Fixed broken module paths and imports
- Added automatic database schema migration
- Fixed API client initialization to support `OPENAI_APIKEY` variant
- All tests passing after refactor
- Both Walmart and Costco providers verified working end-to-end

## Testing Strategy

See [testing.md](testing.md) for detailed testing guidelines.

**Key Principles:**
- **Domain layer:** Unit tests with no mocks (pure functions)
- **Application layer:** Integration tests with mock adapters
- **Adapters:** Can skip real API tests in CI, test interfaces
- **TDD approach:** Write test first, watch it fail, implement, watch it pass

## Further Reading

- [CLAUDE.md](../CLAUDE.md) - Development methodology and TDD practices
- [adding-providers.md](adding-providers.md) - Guide to adding new retailers
- [testing.md](testing.md) - Testing strategy and guidelines
- [deduplication-safety.md](deduplication-safety.md) - How duplicate prevention works
