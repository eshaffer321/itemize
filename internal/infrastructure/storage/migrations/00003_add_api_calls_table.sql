-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS api_calls (
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
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_api_calls_run_id
    ON api_calls(run_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_api_calls_order_id
    ON api_calls(order_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_api_calls_timestamp
    ON api_calls(timestamp DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_api_calls_method
    ON api_calls(method);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_api_calls_method;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_api_calls_timestamp;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_api_calls_order_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_api_calls_run_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS api_calls;
-- +goose StatementEnd
