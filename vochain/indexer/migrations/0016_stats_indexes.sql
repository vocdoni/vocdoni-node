-- +goose Up
-- Indexes backing the grouped counts served by /chain/stats. Without them both
-- aggregates are full table scans; with them sqlite can walk the index instead.
CREATE INDEX index_transactions_type
ON transactions(type);

CREATE INDEX index_processes_status
ON processes(status);

-- +goose Down
DROP INDEX index_processes_status;

DROP INDEX index_transactions_type;
