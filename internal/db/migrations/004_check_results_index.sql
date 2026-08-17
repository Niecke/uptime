-- +goose Up
-- Covering index for both read paths on check_results:
--   ListEndpoints    — MAX(checked_at) GROUP BY endpoint_id, plus the per-endpoint counts
--   HistoryEndpoints — endpoint_id = ? AND checked_at > ?
-- status_code and duration_ms are in the index so neither query has to touch the
-- table itself; rows are fat (headers JSON) and the rowid lookups dominated.
-- IF NOT EXISTS because the index was created by hand on the running instance.
CREATE INDEX IF NOT EXISTS idx_check_results_history
    ON check_results(endpoint_id, checked_at, status_code, duration_ms);

-- Superseded: (endpoint_id, checked_at) is a prefix of the index above.
DROP INDEX IF EXISTS idx_check_results_endpoint_checked;

-- +goose Down
DROP INDEX IF EXISTS idx_check_results_history;
