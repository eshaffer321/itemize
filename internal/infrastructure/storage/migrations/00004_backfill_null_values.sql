-- +goose Up
-- Backfill empty strings for fields that should never be NULL
-- provider should always have a value
-- +goose StatementBegin
UPDATE processing_records SET provider = 'unknown' WHERE provider IS NULL;
-- +goose StatementEnd

-- status should always have a value
-- +goose StatementBegin
UPDATE processing_records SET status = 'unknown' WHERE status IS NULL;
-- +goose StatementEnd

-- JSON fields: empty array is better than NULL for items/splits
-- +goose StatementBegin
UPDATE processing_records SET items_json = '[]' WHERE items_json IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET splits_json = '[]' WHERE splits_json IS NULL;
-- +goose StatementEnd

-- Numeric fields: 0 is better than NULL
-- +goose StatementBegin
UPDATE processing_records SET order_total = 0 WHERE order_total IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET order_subtotal = 0 WHERE order_subtotal IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET order_tax = 0 WHERE order_tax IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET order_tip = 0 WHERE order_tip IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET transaction_amount = 0 WHERE transaction_amount IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET split_count = 0 WHERE split_count IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET item_count = 0 WHERE item_count IS NULL;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET match_confidence = 0 WHERE match_confidence IS NULL;
-- +goose StatementEnd

-- Note: We intentionally leave these as potentially NULL since NULL has meaning:
-- - transaction_id: NULL means no match found
-- - error_message: NULL means no error (success)
-- - multi_delivery_data: NULL means not a multi-delivery order
-- - order_date, processed_at: Could be NULL for very old/corrupt records

-- +goose Down
-- Data migrations are generally not reversible
-- The original NULL values cannot be restored
-- +goose StatementBegin
SELECT 1; -- No-op
-- +goose StatementEnd
