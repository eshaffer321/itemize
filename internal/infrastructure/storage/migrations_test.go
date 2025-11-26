package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrations_FreshDatabase tests running migrations on a fresh database
func TestMigrations_FreshDatabase(t *testing.T) {
	// Create temp database
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	// Create storage (this runs migrations)
	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Verify all migrations were applied
	applied, err := store.getAppliedMigrations()
	require.NoError(t, err)

	assert.True(t, applied[1], "Migration 1 should be applied")
	assert.True(t, applied[2], "Migration 2 should be applied")
	assert.True(t, applied[3], "Migration 3 should be applied")
	assert.Len(t, applied, 3, "Should have exactly 3 migrations applied")

	// Verify schema_migrations table exists
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "Should have 3 migration records")
}

// TestMigrations_Idempotency tests that migrations can be run multiple times
func TestMigrations_Idempotency(t *testing.T) {
	// Create temp database
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	// Run migrations first time
	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	store.Close()

	// Run migrations second time (should be idempotent)
	store, err = NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Verify still have exactly 3 migrations
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "Should still have exactly 3 migration records")
}

// TestMigrations_Schema tests that the correct schema is created
func TestMigrations_Schema(t *testing.T) {
	// Create temp database
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Test processing_records table exists
	err = store.db.QueryRow("SELECT COUNT(*) FROM processing_records").Scan(new(int))
	assert.NoError(t, err, "processing_records table should exist")

	// Test sync_runs table exists
	err = store.db.QueryRow("SELECT COUNT(*) FROM sync_runs").Scan(new(int))
	assert.NoError(t, err, "sync_runs table should exist")

	// Test api_calls table exists
	err = store.db.QueryRow("SELECT COUNT(*) FROM api_calls").Scan(new(int))
	assert.NoError(t, err, "api_calls table should exist")

	// Test schema_migrations table exists
	err = store.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(new(int))
	assert.NoError(t, err, "schema_migrations table should exist")
}

// TestMigrations_ForeignKeyConstraints tests that foreign keys are enforced
func TestMigrations_ForeignKeyConstraints(t *testing.T) {
	// Create temp database
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Verify foreign keys are enabled
	var fkEnabled int
	err = store.db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	require.NoError(t, err)
	assert.Equal(t, 1, fkEnabled, "Foreign keys should be enabled")

	// Test foreign key constraint: try to insert api_call with non-existent run_id
	_, err = store.db.Exec(`
		INSERT INTO api_calls (run_id, order_id, method, request_json, response_json, error, duration_ms)
		VALUES (99999, 'test', 'TestMethod', '{}', '{}', '', 100)
	`)
	assert.Error(t, err, "Should fail to insert api_call with non-existent run_id")
	assert.Contains(t, err.Error(), "FOREIGN KEY constraint failed", "Error should mention foreign key constraint")
}

// TestMigrations_Sequential tests that migrations run in order
func TestMigrations_Sequential(t *testing.T) {
	// Create temp database
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Query migrations in order
	rows, err := store.db.Query("SELECT version, name FROM schema_migrations ORDER BY version")
	require.NoError(t, err)
	defer rows.Close()

	expectedMigrations := []struct {
		version int
		name    string
	}{
		{1, "initial_schema"},
		{2, "add_sync_runs_table"},
		{3, "add_api_calls_table"},
	}

	i := 0
	for rows.Next() {
		var version int
		var name string
		err := rows.Scan(&version, &name)
		require.NoError(t, err)

		require.Less(t, i, len(expectedMigrations), "Too many migrations")
		assert.Equal(t, expectedMigrations[i].version, version)
		assert.Equal(t, expectedMigrations[i].name, name)
		i++
	}

	assert.Equal(t, len(expectedMigrations), i, "Should have all expected migrations")
}

// TestMigrations_APICallsSchema tests the api_calls table schema
func TestMigrations_APICallsSchema(t *testing.T) {
	// Create temp database
	tmpDB := createTempDB(t)
	defer os.Remove(tmpDB)

	store, err := NewStorage(tmpDB)
	require.NoError(t, err)
	defer store.Close()

	// Insert a sync run first (for foreign key)
	runID, err := store.StartSyncRun("test-provider", 30, false)
	require.NoError(t, err)

	// Test inserting an API call
	apiCall := &APICall{
		RunID:        runID,
		OrderID:      "test-order-123",
		Method:       "Transactions.Update",
		RequestJSON:  `{"amount": 100}`,
		ResponseJSON: `{"id": "txn-123"}`,
		Error:        "",
		DurationMs:   500,
	}

	err = store.LogAPICall(apiCall)
	require.NoError(t, err)

	// Query it back
	calls, err := store.GetAPICallsByOrderID("test-order-123")
	require.NoError(t, err)
	require.Len(t, calls, 1)

	// Verify fields
	assert.Equal(t, runID, calls[0].RunID)
	assert.Equal(t, "test-order-123", calls[0].OrderID)
	assert.Equal(t, "Transactions.Update", calls[0].Method)
	assert.Equal(t, `{"amount": 100}`, calls[0].RequestJSON)
	assert.Equal(t, `{"id": "txn-123"}`, calls[0].ResponseJSON)
	assert.Equal(t, "", calls[0].Error)
	assert.Equal(t, int64(500), calls[0].DurationMs)
}

// createTempDB creates a temporary database file for testing
func createTempDB(t *testing.T) string {
	tmpFile, err := os.CreateTemp("", "test_*.db")
	require.NoError(t, err)
	tmpFile.Close()
	return tmpFile.Name()
}
