-- +goose Up
-- order_ledgers: Store ledger snapshots with history
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS order_ledgers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id TEXT NOT NULL,
    sync_run_id INTEGER,
    provider TEXT NOT NULL,
    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ledger_state TEXT NOT NULL,
    ledger_version INTEGER DEFAULT 1,
    ledger_json TEXT NOT NULL,
    total_charged REAL,
    charge_count INTEGER,
    payment_method_types TEXT,
    has_refunds BOOLEAN DEFAULT 0,
    is_valid BOOLEAN DEFAULT 1,
    validation_notes TEXT,
    FOREIGN KEY (sync_run_id) REFERENCES sync_runs(id)
);
-- +goose StatementEnd

-- Indexes for order_ledgers
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_order_ledgers_order_id
    ON order_ledgers(order_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_order_ledgers_provider
    ON order_ledgers(provider);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_order_ledgers_state
    ON order_ledgers(ledger_state);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_order_ledgers_fetched
    ON order_ledgers(fetched_at DESC);
-- +goose StatementEnd

-- ledger_charges: Normalized charge entries for querying
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ledger_charges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_ledger_id INTEGER NOT NULL,
    order_id TEXT NOT NULL,
    sync_run_id INTEGER,
    charge_sequence INTEGER NOT NULL,
    charge_amount REAL NOT NULL,
    charge_type TEXT,
    payment_method TEXT,
    card_type TEXT,
    card_last_four TEXT,
    monarch_transaction_id TEXT,
    is_matched BOOLEAN DEFAULT 0,
    match_confidence REAL,
    matched_at TIMESTAMP,
    split_count INTEGER,
    FOREIGN KEY (order_ledger_id) REFERENCES order_ledgers(id),
    FOREIGN KEY (sync_run_id) REFERENCES sync_runs(id)
);
-- +goose StatementEnd

-- Indexes for ledger_charges
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_ledger_charges_order_id
    ON ledger_charges(order_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_ledger_charges_ledger_id
    ON ledger_charges(order_ledger_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_ledger_charges_monarch_tx
    ON ledger_charges(monarch_transaction_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_ledger_charges_unmatched
    ON ledger_charges(is_matched) WHERE is_matched = 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_ledger_charges_unmatched;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_ledger_charges_monarch_tx;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_ledger_charges_ledger_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_ledger_charges_order_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS ledger_charges;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_order_ledgers_fetched;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_order_ledgers_state;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_order_ledgers_provider;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_order_ledgers_order_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS order_ledgers;
-- +goose StatementEnd
