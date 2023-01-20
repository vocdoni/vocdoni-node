-- +goose Up
CREATE TABLE vote_references (
  nullifier       BLOB NOT NULL PRIMARY KEY,
  process_id      BLOB NOT NULL,
  height          INTEGER NOT NULL,
  weight          TEXT NOT NULL, -- a bigint in Go, so we store via MarshalText
  tx_index        INTEGER NOT NULL,
  creation_time   DATETIME NOT NULL,
  voter_id BLOB   NOT NULL DEFAULT X'',
  overwrite_count INTEGER NOT NULL DEFAULT 0,


  FOREIGN KEY(process_id) REFERENCES processes(id)
);

-- +goose Down
DROP TABLE vote_references
