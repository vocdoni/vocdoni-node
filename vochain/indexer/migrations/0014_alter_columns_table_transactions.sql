-- +goose Up
ALTER TABLE transactions ADD COLUMN raw_tx BLOB NOT NULL DEFAULT x'';

-- +goose Down
ALTER TABLE transactions DROP COLUMN raw_tx;
