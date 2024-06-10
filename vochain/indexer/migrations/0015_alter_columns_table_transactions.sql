-- +goose Up
ALTER TABLE transactions ADD COLUMN subtype TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN raw_tx BLOB NOT NULL DEFAULT x'';
ALTER TABLE transactions ADD COLUMN signature BLOB NOT NULL DEFAULT x'';
ALTER TABLE transactions ADD COLUMN signer BLOB NOT NULL DEFAULT x'';

-- +goose Down
ALTER TABLE transactions DROP COLUMN signer;
ALTER TABLE transactions DROP COLUMN signature;
ALTER TABLE transactions DROP COLUMN raw_tx;
ALTER TABLE transactions DROP COLUMN subtype;
