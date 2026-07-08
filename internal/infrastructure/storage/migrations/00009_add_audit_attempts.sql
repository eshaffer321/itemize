-- +goose Up
-- Durable debugging/audit trail for order processing.
-- processing_records remains the latest summary row. processing_attempts is append-only.

-- +goose StatementBegin
ALTER TABLE processing_records ADD COLUMN match_diagnostics_json TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS processing_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER,
    order_id TEXT NOT NULL,
    provider TEXT,
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
    multi_delivery_data TEXT,
    monarch_notes TEXT,
    category_id TEXT,
    category_name TEXT,
    order_fees_json TEXT,
    raw_order_json TEXT,
    match_diagnostics_json TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (run_id) REFERENCES sync_runs(id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_processing_attempts_order_id
    ON processing_attempts(order_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_processing_attempts_run_id
    ON processing_attempts(run_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_processing_attempts_created_at
    ON processing_attempts(created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS order_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER,
    order_id TEXT NOT NULL,
    transaction_id TEXT NOT NULL,
    role TEXT NOT NULL,
    amount REAL,
    category_id TEXT,
    category_name TEXT,
    notes TEXT,
    observed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (run_id) REFERENCES sync_runs(id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_order_transactions_order_id
    ON order_transactions(order_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_order_transactions_transaction_id
    ON order_transactions(transaction_id);
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE api_calls ADD COLUMN transaction_id TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE api_calls ADD COLUMN dry_run BOOLEAN DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- SQLite cannot drop columns directly. Leave nullable columns/tables in place.
