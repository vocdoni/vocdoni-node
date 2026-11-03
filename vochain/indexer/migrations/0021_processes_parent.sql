-- +goose Up
-- The metadata-only process a process links to through its parentProcessId,
-- whose metadata applies to it too. Empty means no parent. A process cannot
-- change its parent, and processes indexed before this migration have none,
-- since parent links did not exist until now.
ALTER TABLE processes ADD COLUMN parent_process_id BLOB NOT NULL DEFAULT x'';

CREATE INDEX index_processes_parent_process_id
ON processes(parent_process_id);

-- +goose Down
DROP INDEX index_processes_parent_process_id;

ALTER TABLE processes DROP COLUMN parent_process_id;
