-- +goose Up
CREATE TABLE token_fees (
  id           INTEGER NOT NULL PRIMARY KEY,
  block_height INTEGER NOT NULL,
  from_account BLOB NOT NULL,
  reference    TEXT NOT NULL,
  cost         INTEGER NOT NULL,
  tx_type      TEXT NOT NULL,
  spend_time   DATETIME NOT NULL
);

CREATE INDEX index_from_account_token_fees
ON token_fees(from_account);

CREATE INDEX index_tx_type_token_fees
ON token_fees(tx_type);

CREATE INDEX index_tx_reference_fees
ON token_fees(reference);

-- +goose Down
DROP TABLE token_fees;

DROP INDEX index_tx_type_token_fees;
DROP INDEX index_from_account_token_fees;
DROP INDEX index_tx_reference_fees;
