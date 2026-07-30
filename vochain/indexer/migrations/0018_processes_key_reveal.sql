-- +goose Up
-- Where an encrypted election had its encryption keys revealed. Zero means the
-- keys were never revealed, or the revealing transaction is not indexed. Clients
-- otherwise had to scan blocks around the end of the election looking for it.
ALTER TABLE processes ADD COLUMN key_reveal_height INTEGER NOT NULL DEFAULT 0;

ALTER TABLE processes ADD COLUMN key_reveal_tx_hash BLOB NOT NULL DEFAULT x'';

-- +goose Down
ALTER TABLE processes DROP COLUMN key_reveal_tx_hash;

ALTER TABLE processes DROP COLUMN key_reveal_height;
