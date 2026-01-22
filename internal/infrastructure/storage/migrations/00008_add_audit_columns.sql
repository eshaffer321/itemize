-- +goose Up
-- Add audit trail columns to processing_records for better debugging
-- These columns capture the category and notes sent to Monarch, raw order data, and fees

ALTER TABLE processing_records ADD COLUMN monarch_notes TEXT;
ALTER TABLE processing_records ADD COLUMN category_id TEXT;
ALTER TABLE processing_records ADD COLUMN category_name TEXT;
ALTER TABLE processing_records ADD COLUMN order_fees_json TEXT;
ALTER TABLE processing_records ADD COLUMN raw_order_json TEXT;

-- +goose Down
-- SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
-- For simplicity, we'll leave these columns in place on downgrade since they're nullable
-- and won't affect existing functionality
