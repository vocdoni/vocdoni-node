-- +goose Up
CREATE TABLE vote_references (
  nullifier       BLOB NOT NULL PRIMARY KEY,
  process_id      BLOB NOT NULL,
  height          INTEGER NOT NULL,
  weight          TEXT NOT NULL, -- a bigint in Go, so we store via MarshalText
  tx_index        INTEGER NOT NULL,
  creation_time   DATETIME NOT NULL,
  voter_id BLOB   NOT NULL,
  overwrite_count INTEGER NOT NULL,

  FOREIGN KEY(process_id) REFERENCES processes(id)
);

CREATE INDEX index_vote_references_height_tx_index
ON vote_references(height, tx_index);

-- +goose Down
DROP TABLE vote_references

DROP INDEX index_vote_references_height_tx_index
