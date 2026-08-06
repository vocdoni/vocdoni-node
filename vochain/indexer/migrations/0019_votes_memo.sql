-- +goose Up
-- The optional free-text note attached by the voter. Opaque bytes: the chain
-- stores and returns them verbatim and does not interpret them. Empty means the
-- voter attached none. Indexing it keeps the vote endpoints served entirely from
-- SQLite; reading it back from the state tree cost one merkle subtree open per
-- row. Votes indexed before this migration read as empty until their db is
-- rebuilt, since a migration cannot backfill from state.
ALTER TABLE votes ADD COLUMN memo BLOB NOT NULL DEFAULT x'';

-- +goose Down
ALTER TABLE votes DROP COLUMN memo;
