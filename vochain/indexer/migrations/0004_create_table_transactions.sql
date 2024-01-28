-- +goose Up
CREATE TABLE transactions (
  -- The id key auto-increments; see https://www.sqlite.org/autoinc.html.
  -- We don't need AUTOINCREMENT as we don't delete rows.
  id           INTEGER NOT NULL PRIMARY KEY,
  hash         BLOB NOT NULL,
  block_height INTEGER NOT NULL,
  block_index  INTEGER NOT NULL,
  type         TEXT NOT NULL
);

CREATE INDEX transactions_hash
ON transactions(hash);

CREATE INDEX transactions_block_height_index
ON transactions(block_height, block_index);

-- +goose Down
DROP TABLE transactions;

DROP INDEX transactions_hash;

DROP INDEX transactions_block_height_index;
