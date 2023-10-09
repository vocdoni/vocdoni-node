-- +goose Up
CREATE TABLE token_transfers (
  tx_hash       BLOB NOT NULL PRIMARY KEY,
  block_height  INTEGER NOT NULL,
  from_account  BLOB NOT NULL,
  to_account    BLOB NOT NULL,
  amount        INTEGER NOT NULL,
  transfer_time DATETIME NOT NULL,

  FOREIGN KEY(to_account) REFERENCES accounts(account),
  FOREIGN KEY(from_account) REFERENCES accounts(account)
);

CREATE INDEX index_from_account_token_transfers
ON token_transfers(from_account);

-- +goose Down
DROP TABLE token_transfers;

DROP INDEX index_from_account_token_transfers
