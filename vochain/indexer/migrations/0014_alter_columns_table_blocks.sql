-- +goose Up
ALTER TABLE blocks DROP COLUMN data_hash;
ALTER TABLE blocks ADD COLUMN chain_id TEXT NOT NULL DEFAULT '';
ALTER TABLE blocks ADD COLUMN hash BLOB NOT NULL DEFAULT x'';
ALTER TABLE blocks ADD COLUMN proposer_address BLOB NOT NULL DEFAULT x'';
ALTER TABLE blocks ADD COLUMN last_block_hash BLOB NOT NULL DEFAULT x'';

-- +goose Down
ALTER TABLE blocks ADD COLUMN data_hash BLOB NOT NULL;
ALTER TABLE blocks DROP COLUMN chain_id;
ALTER TABLE blocks DROP COLUMN hash;
ALTER TABLE blocks DROP COLUMN proposer_address;
ALTER TABLE blocks DROP COLUMN last_block_hash;
