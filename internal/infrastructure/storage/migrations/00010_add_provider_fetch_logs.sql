-- +goose Up
-- Store provider fetch snapshots and distinguish API-call intent from completion.

-- +goose StatementBegin
ALTER TABLE api_calls ADD COLUMN phase TEXT DEFAULT 'completed';
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS provider_fetches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER,
    provider TEXT NOT NULL,
    fetch_type TEXT NOT NULL,
    request_json TEXT,
    response_json TEXT,
    error TEXT,
    duration_ms INTEGER,
    order_count INTEGER DEFAULT 0,
    transaction_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (run_id) REFERENCES sync_runs(id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_provider_fetches_run_id
    ON provider_fetches(run_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_provider_fetches_provider
    ON provider_fetches(provider);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_provider_fetches_created_at
    ON provider_fetches(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- SQLite cannot drop columns directly. Leave nullable columns/tables in place.
