-- name: CreateAccount :execresult
REPLACE INTO accounts (
    account, balance, nonce
) VALUES (?, ?, ?);

-- name: SearchAccounts :many
WITH results AS (
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
)
SELECT *, COUNT(*) OVER() AS total_count
FROM results
ORDER BY balance DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: SetAccountMetadata :execresult
-- Stores the name and avatar resolved from the account off-chain metadata. Only
-- writes when either actually changed, so re-resolving unchanged metadata costs
-- no write. The account row must exist; accounts are created by CreateAccount
-- when the state indexes them.
UPDATE accounts
SET name = sqlc.arg(name),
    avatar = sqlc.arg(avatar)
WHERE account = sqlc.arg(account)
  AND (name != sqlc.arg(name) OR avatar != sqlc.arg(avatar));

-- name: ListAccountsMissingName :many
-- Lists the accounts whose name was never resolved, so a backfill knows which
-- ones to look up. Used once per boot. Paged by the account id rather than by an
-- offset, so that a page is never revisited even though rows leave the result set
-- as the backfill fills them.
SELECT account FROM accounts
WHERE name = '' AND account > sqlc.arg(after_account)
ORDER BY account
LIMIT sqlc.arg(limit);

-- name: CountAccounts :one
SELECT COUNT(*) FROM accounts;