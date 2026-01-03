-- +goose Up
-- +goose StatementBegin
ALTER TABLE ledger_charges ADD COLUMN charged_at TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- SQLite doesn't support DROP COLUMN in older versions
-- This is a no-op for safety - column will remain
-- +goose StatementBegin
SELECT 1; -- No-op
-- +goose StatementEnd
