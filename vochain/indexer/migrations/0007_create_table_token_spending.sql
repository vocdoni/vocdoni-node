-- +goose Up
CREATE TABLE token_spendings (
  id           INTEGER NOT NULL PRIMARY KEY,
  block_height INTEGER NOT NULL,
  from_account BLOB NOT NULL,
  reference    BLOB NOT NULL,
  cost         INTEGER NOT NULL,
  txType       TEXT NOT NULL,
  spend_time   DATETIME NOT NULL
);

CREATE INDEX index_from_account_token_spendings
ON token_spendings(from_account);

CREATE INDEX index_txType_token_spendings
ON token_spendings(txType);

-- +goose Down
DROP TABLE token_spendings

DROP INDEX index_txType_token_spendings
DROP INDEX index_from_account_token_spendings
