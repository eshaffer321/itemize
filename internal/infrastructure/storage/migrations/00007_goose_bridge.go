package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upGooseBridge, downGooseBridge)
}

// upGooseBridge handles the transition from the custom migration system to goose.
// It checks if the old schema_migrations table exists and if so, copies the
// migration history to goose_db_version so goose won't re-run already-applied migrations.
func upGooseBridge(ctx context.Context, tx *sql.Tx) error {
	// Check if schema_migrations table exists (indicates migration from old system)
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='schema_migrations'
	`).Scan(&exists)
	if err != nil {
		return err
	}

	if exists == 0 {
		// Fresh install - old migration table doesn't exist
		// Nothing to migrate, goose will handle everything
		return nil
	}

	// Old system exists - we need to mark migrations 1-6 as applied in goose's table
	// Goose has already created goose_db_version table at this point
	// We just need to ensure migrations 1-6 are marked as applied

	// Get already applied migrations from old system
	rows, err := tx.QueryContext(ctx, `
		SELECT version, applied_at FROM schema_migrations
		WHERE version < 7
		ORDER BY version
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		var appliedAt sql.NullTime
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return err
		}

		// Check if this version already exists in goose table
		var count int
		err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM goose_db_version WHERE version_id = ?
		`, version).Scan(&count)
		if err != nil {
			return err
		}

		// Only insert if not already present
		if count == 0 {
			if appliedAt.Valid {
				_, err = tx.ExecContext(ctx, `
					INSERT INTO goose_db_version (version_id, is_applied, tstamp)
					VALUES (?, 1, ?)
				`, version, appliedAt.Time)
			} else {
				_, err = tx.ExecContext(ctx, `
					INSERT INTO goose_db_version (version_id, is_applied)
					VALUES (?, 1)
				`, version)
			}
			if err != nil {
				return err
			}
		}
	}

	return rows.Err()
}

// downGooseBridge is a no-op - we don't want to undo the bridge migration
func downGooseBridge(ctx context.Context, tx *sql.Tx) error {
	// No-op for bridge migration - can't really undo switching migration systems
	return nil
}
