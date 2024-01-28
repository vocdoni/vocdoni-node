-- +goose Up
CREATE INDEX index_votes_process_id
ON votes(process_id);
-- +goose Down
DROP INDEX index_votes_process_id;
