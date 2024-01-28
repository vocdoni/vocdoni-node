-- +goose Up
CREATE TABLE blocks (
  height    INTEGER NOT NULL PRIMARY KEY,
  time      DATETIME NOT NULL,
  data_hash BLOB NOT NULL
);

-- +goose Down
DROP TABLE blocks;
