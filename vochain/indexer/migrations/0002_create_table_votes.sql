-- +goose Up
CREATE TABLE votes (
  nullifier       BLOB NOT NULL PRIMARY KEY,
  process_id      BLOB NOT NULL,
  block_height    INTEGER NOT NULL,
  block_index     INTEGER NOT NULL,
  weight          TEXT NOT NULL, -- a bigint in Go, so we store via MarshalText
  creation_time   DATETIME NOT NULL, -- TODO: unused, remove
  voter_id        BLOB NOT NULL,
  overwrite_count INTEGER NOT NULL,

  FOREIGN KEY(process_id) REFERENCES processes(id)
);

CREATE INDEX index_votes_block_height_index
ON votes(block_height, block_index);

-- +goose Down
DROP TABLE votes

DROP INDEX index_votes_block_height_index
