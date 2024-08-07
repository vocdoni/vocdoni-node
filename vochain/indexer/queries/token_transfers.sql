-- name: CreateTokenTransfer :execresult
INSERT INTO token_transfers (
	tx_hash, block_height, from_account,
	to_account, amount, transfer_time
) VALUES (
	?, ?, ?,
	?, ?, ?
);

-- name: GetTokenTransfer :one
SELECT * FROM token_transfers
WHERE tx_hash = ?
LIMIT 1;

-- name: SearchTokenTransfers :many
WITH results AS (
  SELECT *
  FROM token_transfers
  WHERE (
    (sqlc.arg(from_or_to_account) = '' OR (
		LOWER(HEX(from_account)) = LOWER(sqlc.arg(from_or_to_account))
		OR LOWER(HEX(to_account)) = LOWER(sqlc.arg(from_or_to_account))
	))
    AND (sqlc.arg(from_account) = '' OR LOWER(HEX(from_account)) = LOWER(sqlc.arg(from_account)))
    AND (sqlc.arg(to_account) = '' OR LOWER(HEX(to_account)) = LOWER(sqlc.arg(to_account)))
  )
)
SELECT *, COUNT(*) OVER() AS total_count
FROM results
ORDER BY transfer_time DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: CountTokenTransfersByAccount :one
SELECT COUNT(*) FROM token_transfers
WHERE to_account = sqlc.arg(account) OR
	  from_account = sqlc.arg(account);