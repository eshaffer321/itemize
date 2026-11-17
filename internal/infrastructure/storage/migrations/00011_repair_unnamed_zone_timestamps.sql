-- +goose Up
-- Repair timestamps the driver wrote with time.Time.String for an unnamed
-- fixed zone, e.g. '2026-10-31 19:11:53 -0700 -0700'. The driver cannot parse
-- that back into a time.Time, so loading those rows failed. Rewrite them to
-- SQLite's '2026-10-31 19:11:53-07:00' form, which keeps the same instant.
-- Any trailing monotonic clock reading ('m=+...') is dropped.

-- +goose StatementBegin
UPDATE processing_records SET order_date = substr(order_date, 1, (CASE WHEN instr(order_date, ' -') > 0 THEN instr(order_date, ' -') ELSE instr(order_date, ' +') END) - 1) || substr(order_date, (CASE WHEN instr(order_date, ' -') > 0 THEN instr(order_date, ' -') ELSE instr(order_date, ' +') END) + 1, 3) || ':' || substr(order_date, (CASE WHEN instr(order_date, ' -') > 0 THEN instr(order_date, ' -') ELSE instr(order_date, ' +') END) + 4, 2)
WHERE order_date GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_records SET processed_at = substr(processed_at, 1, (CASE WHEN instr(processed_at, ' -') > 0 THEN instr(processed_at, ' -') ELSE instr(processed_at, ' +') END) - 1) || substr(processed_at, (CASE WHEN instr(processed_at, ' -') > 0 THEN instr(processed_at, ' -') ELSE instr(processed_at, ' +') END) + 1, 3) || ':' || substr(processed_at, (CASE WHEN instr(processed_at, ' -') > 0 THEN instr(processed_at, ' -') ELSE instr(processed_at, ' +') END) + 4, 2)
WHERE processed_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_attempts SET order_date = substr(order_date, 1, (CASE WHEN instr(order_date, ' -') > 0 THEN instr(order_date, ' -') ELSE instr(order_date, ' +') END) - 1) || substr(order_date, (CASE WHEN instr(order_date, ' -') > 0 THEN instr(order_date, ' -') ELSE instr(order_date, ' +') END) + 1, 3) || ':' || substr(order_date, (CASE WHEN instr(order_date, ' -') > 0 THEN instr(order_date, ' -') ELSE instr(order_date, ' +') END) + 4, 2)
WHERE order_date GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_attempts SET processed_at = substr(processed_at, 1, (CASE WHEN instr(processed_at, ' -') > 0 THEN instr(processed_at, ' -') ELSE instr(processed_at, ' +') END) - 1) || substr(processed_at, (CASE WHEN instr(processed_at, ' -') > 0 THEN instr(processed_at, ' -') ELSE instr(processed_at, ' +') END) + 1, 3) || ':' || substr(processed_at, (CASE WHEN instr(processed_at, ' -') > 0 THEN instr(processed_at, ' -') ELSE instr(processed_at, ' +') END) + 4, 2)
WHERE processed_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE processing_attempts SET created_at = substr(created_at, 1, (CASE WHEN instr(created_at, ' -') > 0 THEN instr(created_at, ' -') ELSE instr(created_at, ' +') END) - 1) || substr(created_at, (CASE WHEN instr(created_at, ' -') > 0 THEN instr(created_at, ' -') ELSE instr(created_at, ' +') END) + 1, 3) || ':' || substr(created_at, (CASE WHEN instr(created_at, ' -') > 0 THEN instr(created_at, ' -') ELSE instr(created_at, ' +') END) + 4, 2)
WHERE created_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE order_ledgers SET fetched_at = substr(fetched_at, 1, (CASE WHEN instr(fetched_at, ' -') > 0 THEN instr(fetched_at, ' -') ELSE instr(fetched_at, ' +') END) - 1) || substr(fetched_at, (CASE WHEN instr(fetched_at, ' -') > 0 THEN instr(fetched_at, ' -') ELSE instr(fetched_at, ' +') END) + 1, 3) || ':' || substr(fetched_at, (CASE WHEN instr(fetched_at, ' -') > 0 THEN instr(fetched_at, ' -') ELSE instr(fetched_at, ' +') END) + 4, 2)
WHERE fetched_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE ledger_charges SET charged_at = substr(charged_at, 1, (CASE WHEN instr(charged_at, ' -') > 0 THEN instr(charged_at, ' -') ELSE instr(charged_at, ' +') END) - 1) || substr(charged_at, (CASE WHEN instr(charged_at, ' -') > 0 THEN instr(charged_at, ' -') ELSE instr(charged_at, ' +') END) + 1, 3) || ':' || substr(charged_at, (CASE WHEN instr(charged_at, ' -') > 0 THEN instr(charged_at, ' -') ELSE instr(charged_at, ' +') END) + 4, 2)
WHERE charged_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE ledger_charges SET matched_at = substr(matched_at, 1, (CASE WHEN instr(matched_at, ' -') > 0 THEN instr(matched_at, ' -') ELSE instr(matched_at, ' +') END) - 1) || substr(matched_at, (CASE WHEN instr(matched_at, ' -') > 0 THEN instr(matched_at, ' -') ELSE instr(matched_at, ' +') END) + 1, 3) || ':' || substr(matched_at, (CASE WHEN instr(matched_at, ' -') > 0 THEN instr(matched_at, ' -') ELSE instr(matched_at, ' +') END) + 4, 2)
WHERE matched_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE order_transactions SET observed_at = substr(observed_at, 1, (CASE WHEN instr(observed_at, ' -') > 0 THEN instr(observed_at, ' -') ELSE instr(observed_at, ' +') END) - 1) || substr(observed_at, (CASE WHEN instr(observed_at, ' -') > 0 THEN instr(observed_at, ' -') ELSE instr(observed_at, ' +') END) + 1, 3) || ':' || substr(observed_at, (CASE WHEN instr(observed_at, ' -') > 0 THEN instr(observed_at, ' -') ELSE instr(observed_at, ' +') END) + 4, 2)
WHERE observed_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE provider_fetches SET created_at = substr(created_at, 1, (CASE WHEN instr(created_at, ' -') > 0 THEN instr(created_at, ' -') ELSE instr(created_at, ' +') END) - 1) || substr(created_at, (CASE WHEN instr(created_at, ' -') > 0 THEN instr(created_at, ' -') ELSE instr(created_at, ' +') END) + 1, 3) || ':' || substr(created_at, (CASE WHEN instr(created_at, ' -') > 0 THEN instr(created_at, ' -') ELSE instr(created_at, ' +') END) + 4, 2)
WHERE created_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE api_calls SET timestamp = substr(timestamp, 1, (CASE WHEN instr(timestamp, ' -') > 0 THEN instr(timestamp, ' -') ELSE instr(timestamp, ' +') END) - 1) || substr(timestamp, (CASE WHEN instr(timestamp, ' -') > 0 THEN instr(timestamp, ' -') ELSE instr(timestamp, ' +') END) + 1, 3) || ':' || substr(timestamp, (CASE WHEN instr(timestamp, ' -') > 0 THEN instr(timestamp, ' -') ELSE instr(timestamp, ' +') END) + 4, 2)
WHERE timestamp GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE sync_runs SET started_at = substr(started_at, 1, (CASE WHEN instr(started_at, ' -') > 0 THEN instr(started_at, ' -') ELSE instr(started_at, ' +') END) - 1) || substr(started_at, (CASE WHEN instr(started_at, ' -') > 0 THEN instr(started_at, ' -') ELSE instr(started_at, ' +') END) + 1, 3) || ':' || substr(started_at, (CASE WHEN instr(started_at, ' -') > 0 THEN instr(started_at, ' -') ELSE instr(started_at, ' +') END) + 4, 2)
WHERE started_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE sync_runs SET completed_at = substr(completed_at, 1, (CASE WHEN instr(completed_at, ' -') > 0 THEN instr(completed_at, ' -') ELSE instr(completed_at, ' +') END) - 1) || substr(completed_at, (CASE WHEN instr(completed_at, ' -') > 0 THEN instr(completed_at, ' -') ELSE instr(completed_at, ' +') END) + 1, 3) || ':' || substr(completed_at, (CASE WHEN instr(completed_at, ' -') > 0 THEN instr(completed_at, ' -') ELSE instr(completed_at, ' +') END) + 4, 2)
WHERE completed_at GLOB '????-??-?? ??:??:??* [+-][0-9][0-9][0-9][0-9] [+-][0-9][0-9][0-9][0-9]*';
-- +goose StatementEnd

-- +goose Down
-- Irreversible: the original strings carried no information the new form lacks.
