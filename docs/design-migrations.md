# Simple SQLite Migration System

**Status:** Implementation Ready
**Effort:** 2-3 hours

## Problem

Current schema management is ad-hoc:
- Manual column checks (lines 25-48 in storage.go)
- `migrateFromV1()` and `ensureMultiDeliveryColumn()` functions
- No tracking of what migrations have run
- Doesn't scale as we add more tables/columns

**We need migrations for:**
1. `sync_runs` table (currently placeholder)
2. `api_calls` table (for API logging)
3. Future schema changes

## Design Principles

For a **single-user CLI tool**, we want:
- ✅ Simple (no external migration files)
- ✅ In-code migrations (easy to review)
- ✅ Version tracking (don't re-run)
- ✅ Fail-fast (errors stop startup)
- ❌ No complex rollback (just restore backup)
- ❌ No down migrations (forward-only)

## Architecture

### Migration Tracking Table

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Migration Structure

```go
type Migration struct {
    Version int
    Name    string
    Up      func(*sql.DB) error
}
```

### Migration Flow

```
1. NewStorage() is called
2. ensureMigrationsTable() creates schema_migrations
3. getAppliedMigrations() reads which versions ran
4. runMigrations() executes pending migrations in order
5. Each migration is wrapped in a transaction
6. If migration fails, app exits with error
```

## Implementation

### File: `internal/infrastructure/storage/migrations.go`

```go
package storage

import (
    "database/sql"
    "fmt"
    "log"
)

// Migration represents a database schema migration
type Migration struct {
    Version int
    Name    string
    Up      func(*sql.DB) error
}

// allMigrations defines all migrations in order
var allMigrations = []Migration{
    {
        Version: 1,
        Name:    "initial_schema",
        Up:      migration001InitialSchema,
    },
    {
        Version: 2,
        Name:    "add_sync_runs_table",
        Up:      migration002AddSyncRunsTable,
    },
    {
        Version: 3,
        Name:    "add_api_calls_table",
        Up:      migration003AddAPICallsTable,
    },
}

// runMigrations executes all pending migrations
func (s *Storage) runMigrations() error {
    // Ensure migrations table exists
    if err := s.ensureMigrationsTable(); err != nil {
        return fmt.Errorf("failed to create migrations table: %w", err)
    }

    // Get applied migrations
    applied, err := s.getAppliedMigrations()
    if err != nil {
        return fmt.Errorf("failed to get applied migrations: %w", err)
    }

    // Run pending migrations
    for _, migration := range allMigrations {
        if applied[migration.Version] {
            continue // Already applied
        }

        log.Printf("Running migration %d: %s", migration.Version, migration.Name)

        // Run migration in transaction
        tx, err := s.db.Begin()
        if err != nil {
            return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
        }

        // Execute migration
        if err := migration.Up(tx); err != nil {
            tx.Rollback()
            return fmt.Errorf("migration %d (%s) failed: %w", migration.Version, migration.Name, err)
        }

        // Record migration
        _, err = tx.Exec(`
            INSERT INTO schema_migrations (version, name) VALUES (?, ?)
        `, migration.Version, migration.Name)
        if err != nil {
            tx.Rollback()
            return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
        }

        // Commit
        if err := tx.Commit(); err != nil {
            return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
        }

        log.Printf("✅ Migration %d complete", migration.Version)
    }

    return nil
}

// ensureMigrationsTable creates the schema_migrations table
func (s *Storage) ensureMigrationsTable() error {
    query := `
    CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`

    _, err := s.db.Exec(query)
    return err
}

// getAppliedMigrations returns a set of applied migration versions
func (s *Storage) getAppliedMigrations() (map[int]bool, error) {
    applied := make(map[int]bool)

    rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var version int
        if err := rows.Scan(&version); err != nil {
            return nil, err
        }
        applied[version] = true
    }

    return applied, rows.Err()
}

// ================================================================
// MIGRATION FUNCTIONS
// ================================================================

// migration001InitialSchema creates the initial processing_records table
func migration001InitialSchema(db *sql.DB) error {
    queries := []string{
        // Main processing records table
        `CREATE TABLE IF NOT EXISTS processing_records (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            order_id TEXT UNIQUE NOT NULL,
            provider TEXT DEFAULT 'walmart',
            transaction_id TEXT,
            order_date TIMESTAMP,
            processed_at TIMESTAMP,
            order_total REAL,
            order_subtotal REAL,
            order_tax REAL,
            order_tip REAL,
            transaction_amount REAL,
            split_count INTEGER,
            status TEXT,
            error_message TEXT,
            item_count INTEGER,
            match_confidence REAL,
            dry_run BOOLEAN DEFAULT 0,
            items_json TEXT,
            splits_json TEXT,
            multi_delivery_data TEXT
        )`,

        // Indexes for common queries
        `CREATE INDEX IF NOT EXISTS idx_processing_records_provider
         ON processing_records(provider)`,

        `CREATE INDEX IF NOT EXISTS idx_processing_records_order_date
         ON processing_records(order_date)`,

        `CREATE INDEX IF NOT EXISTS idx_processing_records_status
         ON processing_records(status)`,
    }

    for _, query := range queries {
        if _, err := db.Exec(query); err != nil {
            return err
        }
    }

    return nil
}

// migration002AddSyncRunsTable creates the sync_runs table
func migration002AddSyncRunsTable(db *sql.DB) error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS sync_runs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            provider TEXT NOT NULL,
            started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            completed_at TIMESTAMP,
            lookback_days INTEGER,
            dry_run BOOLEAN DEFAULT 0,
            orders_found INTEGER DEFAULT 0,
            orders_processed INTEGER DEFAULT 0,
            orders_skipped INTEGER DEFAULT 0,
            orders_errored INTEGER DEFAULT 0,
            status TEXT DEFAULT 'running'
        )`,

        `CREATE INDEX IF NOT EXISTS idx_sync_runs_provider
         ON sync_runs(provider)`,

        `CREATE INDEX IF NOT EXISTS idx_sync_runs_started
         ON sync_runs(started_at DESC)`,
    }

    for _, query := range queries {
        if _, err := db.Exec(query); err != nil {
            return err
        }
    }

    return nil
}

// migration003AddAPICallsTable creates the api_calls table for logging
func migration003AddAPICallsTable(db *sql.DB) error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS api_calls (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            run_id INTEGER,
            order_id TEXT,
            timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            method TEXT NOT NULL,
            request_json TEXT,
            response_json TEXT,
            error TEXT,
            duration_ms INTEGER,
            FOREIGN KEY (run_id) REFERENCES sync_runs(id)
        )`,

        `CREATE INDEX IF NOT EXISTS idx_api_calls_run_id
         ON api_calls(run_id)`,

        `CREATE INDEX IF NOT EXISTS idx_api_calls_order_id
         ON api_calls(order_id)`,

        `CREATE INDEX IF NOT EXISTS idx_api_calls_timestamp
         ON api_calls(timestamp DESC)`,

        `CREATE INDEX IF NOT EXISTS idx_api_calls_method
         ON api_calls(method)`,
    }

    for _, query := range queries {
        if _, err := db.Exec(query); err != nil {
            return err
        }
    }

    return nil
}
```

### File: `internal/infrastructure/storage/storage.go` (changes)

```go
// NewStorage creates a new storage instance with SQLite database
func NewStorage(dbPath string) (*Storage, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }

    s := &Storage{db: db}

    // Run migrations
    if err := s.runMigrations(); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to run migrations: %w", err)
    }

    return s, nil
}
```

**Remove all the old ad-hoc migration code:**
- Remove lines 25-48 (manual column checks)
- Remove `migrateFromV1()` function
- Remove `ensureMultiDeliveryColumn()` function
- Remove `CreateTables()` function

## Migration Strategy for Existing Databases

**Problem:** Existing users have databases created with the old schema.

**Solution:** Migration 001 is idempotent (uses CREATE TABLE IF NOT EXISTS).

**Flow:**
1. User upgrades to new version
2. `schema_migrations` table doesn't exist
3. Migration system creates it
4. Runs Migration 001 (safe, uses IF NOT EXISTS)
5. Runs Migration 002 (new sync_runs table)
6. Runs Migration 003 (new api_calls table)

**Old ad-hoc migrations still there?**
- If processing_records already has provider column → Migration 001 is no-op
- If multi_delivery_data column exists → Migration 001 is no-op
- New tables get created cleanly

## Testing Migrations

### Test 1: Fresh Database

```bash
rm monarch_sync.db
./monarch-sync walmart -dry-run -days 7
sqlite3 monarch_sync.db "SELECT * FROM schema_migrations"
# Should show: 1, 2, 3
```

### Test 2: Existing Database

```bash
# Use existing monarch_sync.db
./monarch-sync walmart -dry-run -days 7
sqlite3 monarch_sync.db "SELECT * FROM schema_migrations"
# Should show: 1, 2, 3
# processing_records should be unchanged
```

### Test 3: Partial Migration

```bash
# Manually create schema_migrations and add version 1 and 2
sqlite3 monarch_sync.db "INSERT INTO schema_migrations (version, name) VALUES (1, 'initial'), (2, 'sync_runs')"
./monarch-sync walmart -dry-run -days 7
sqlite3 monarch_sync.db "SELECT * FROM schema_migrations"
# Should show: 1, 2, 3 (only ran migration 3)
```

## Adding New Migrations (Future)

**Example: Adding a new column to api_calls**

```go
// In migrations.go, add to allMigrations:
{
    Version: 4,
    Name:    "add_api_calls_user_agent",
    Up:      migration004AddUserAgent,
},

func migration004AddUserAgent(db *sql.DB) error {
    _, err := db.Exec(`
        ALTER TABLE api_calls
        ADD COLUMN user_agent TEXT
    `)
    return err
}
```

## Rollback Strategy

**We don't implement down migrations because:**
1. This is a single-user CLI tool
2. Database is local (easy to backup/restore)
3. Rollback complexity isn't worth it

**If migration fails:**
1. App exits with error message
2. User can restore from backup
3. Or delete database and start fresh (sync will rebuild)

**Recommended in docs:**
```bash
# Before upgrading
cp monarch_sync.db monarch_sync.db.backup
```

## Comparison: Before vs After

### Before (Ad-Hoc)

```go
// Check if provider column exists
_, err = db.Exec("SELECT provider FROM processing_records LIMIT 1")
if err != nil {
    migrateFromV1()
}

// Check if multi_delivery_data exists
if err := ensureMultiDeliveryColumn(); err != nil {
    return nil, err
}

// Maybe create tables?
CreateTables()
```

**Problems:**
- No tracking of what ran
- Order-dependent
- Hard to add new migrations
- Brittle

### After (Migration System)

```go
func NewStorage(dbPath string) (*Storage, error) {
    db, _ := sql.Open("sqlite3", dbPath)
    s := &Storage{db: db}

    // Run all pending migrations
    if err := s.runMigrations(); err != nil {
        return nil, err
    }

    return s, nil
}
```

**Benefits:**
- ✅ Tracks what ran in schema_migrations
- ✅ Runs in order automatically
- ✅ Easy to add new migrations
- ✅ Idempotent and safe

## Implementation Checklist

- [ ] Create `internal/infrastructure/storage/migrations.go`
- [ ] Define Migration struct and allMigrations slice
- [ ] Implement runMigrations() function
- [ ] Implement migration001InitialSchema()
- [ ] Implement migration002AddSyncRunsTable()
- [ ] Implement migration003AddAPICallsTable()
- [ ] Update NewStorage() in storage.go to call runMigrations()
- [ ] Remove old migration code (migrateFromV1, ensureMultiDeliveryColumn, CreateTables)
- [ ] Test with fresh database
- [ ] Test with existing database
- [ ] Update documentation

**Estimated Time:** 2-3 hours
