-- +goose Up
CREATE TABLE accounts (
    account BLOB NOT NULL PRIMARY KEY,
    balance INTEGER NOT NULL,
    nonce   INTEGER NOT NULL
);

-- +goose Down
DROP TABLE accounts