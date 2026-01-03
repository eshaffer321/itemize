-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS sync_runs;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE sync_runs (
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
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_sync_runs_provider
    ON sync_runs(provider);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_sync_runs_started
    ON sync_runs(started_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sync_runs_started;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sync_runs_provider;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS sync_runs;
-- +goose StatementEnd
