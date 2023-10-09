-- name: CreateAccount :execresult
REPLACE INTO accounts (
    account, balance, nonce
) VALUES (?, ?, ?)
;

-- name: GetListAccounts :many
SELECT *
FROM accounts
ORDER BY balance DESC
LIMIT ? OFFSET ?
;

-- name: CountAccounts :one
SELECT COUNT(*) FROM accounts;