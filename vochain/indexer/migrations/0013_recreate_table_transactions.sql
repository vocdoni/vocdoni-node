-- +goose Up
PRAGMA foreign_keys = OFF;

-- Create a new table with hash as primary key
CREATE TABLE transactions_new (
  hash         BLOB NOT NULL PRIMARY KEY,
  block_height INTEGER NOT NULL,
  block_index  INTEGER NOT NULL,
  type         TEXT NOT NULL
);

-- Copy data from the old table to the new table
INSERT OR REPLACE INTO transactions_new (hash, block_height, block_index, type)
SELECT hash, block_height, block_index, type
FROM transactions;

-- Drop the old table
DROP TABLE transactions;

-- Rename the new table to the old table name
ALTER TABLE transactions_new RENAME TO transactions;

-- Recreate necessary indexes
CREATE INDEX transactions_block_height_index
ON transactions(block_height, block_index);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

-- Recreate the old table structure
CREATE TABLE transactions (
  id           INTEGER NOT NULL PRIMARY KEY,
  hash         BLOB NOT NULL,
  block_height INTEGER NOT NULL,
  block_index  INTEGER NOT NULL,
  type         TEXT NOT NULL
);

-- Copy data back from the new table to the old table
INSERT INTO transactions (hash, block_height, block_index, type)
SELECT hash, block_height, block_index, type
FROM transactions_new;

-- Drop the new table
DROP TABLE transactions_new;

-- Recreate the old indexes
CREATE INDEX transactions_hash
ON transactions(hash);

CREATE INDEX transactions_block_height_index
ON transactions(block_height, block_index);

PRAGMA foreign_keys = ON;
