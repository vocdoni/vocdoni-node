-- +goose Up
CREATE TABLE tx_references (
  -- The id key auto-increments; see https://www.sqlite.org/autoinc.html.
  -- We don't need AUTOINCREMENT as we don't delete rows.
  id             INTEGER NOT NULL PRIMARY KEY,
  hash           BLOB NOT NULL,
  block_height   INTEGER NOT NULL,
  tx_block_index INTEGER NOT NULL,
  tx_type        TEXT NOT NULL
);

CREATE INDEX tx_references_hash
ON tx_references(hash);

-- +goose Down
DROP TABLE tx_references

DROP INDEX tx_references_hash
