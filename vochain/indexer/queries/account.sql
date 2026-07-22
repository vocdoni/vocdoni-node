-- name: CreateAccount :execresult
REPLACE INTO accounts (
    account, balance, nonce
) VALUES (?, ?, ?);

-- name: SearchAccounts :many
SELECT *
FROM accounts
WHERE (
  (
  sqlc.arg(account_id_substr) = ''
  OR (LENGTH(sqlc.arg(account_id_substr)) = 40 AND LOWER(HEX(account)) = LOWER(sqlc.arg(account_id_substr)))
  OR (LENGTH(sqlc.arg(account_id_substr)) < 40 AND INSTR(LOWER(HEX(account)), LOWER(sqlc.arg(account_id_substr))) > 0)
  -- TODO: consider keeping an account_hex column for faster searches
  )
)
ORDER BY balance DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: CountTotalAccounts :one
SELECT COUNT(*)
FROM accounts;

-- name: CountAccounts :one
SELECT COUNT(*)
FROM accounts
WHERE (
  (
  sqlc.arg(account_id_substr) = ''
  OR (LENGTH(sqlc.arg(account_id_substr)) = 40 AND LOWER(HEX(account)) = LOWER(sqlc.arg(account_id_substr)))
  OR (LENGTH(sqlc.arg(account_id_substr)) < 40 AND INSTR(LOWER(HEX(account)), LOWER(sqlc.arg(account_id_substr))) > 0)
  -- TODO: consider keeping an account_hex column for faster searches
  )
);
