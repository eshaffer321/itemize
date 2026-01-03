-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS processing_records (
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
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_processing_records_provider
    ON processing_records(provider);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_processing_records_order_date
    ON processing_records(order_date);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_processing_records_status
    ON processing_records(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_processing_records_status;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_processing_records_order_date;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_processing_records_provider;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS processing_records;
-- +goose StatementEnd
