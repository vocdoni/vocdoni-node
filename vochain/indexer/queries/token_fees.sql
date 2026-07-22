-- name: CreateTokenFee :execresult
INSERT INTO token_fees (
	from_account, block_height, reference,
	cost, tx_type, spend_time
) VALUES (
	?, ?, ?,
	?, ?, ?
);

-- name: CountTokenFees :one
SELECT COUNT(*)
FROM token_fees
WHERE (
  (sqlc.arg(from_account) = '' OR LOWER(HEX(from_account)) = LOWER(sqlc.arg(from_account)))
  AND (sqlc.arg(tx_type) = '' OR LOWER(tx_type) = LOWER(sqlc.arg(tx_type)))
  AND (sqlc.arg(reference) = '' OR LOWER(reference) = LOWER(sqlc.arg(reference)))
);

-- name: SearchTokenFees :many
SELECT *
FROM token_fees
WHERE (
  (sqlc.arg(from_account) = '' OR LOWER(HEX(from_account)) = LOWER(sqlc.arg(from_account)))
  AND (sqlc.arg(tx_type) = '' OR LOWER(tx_type) = LOWER(sqlc.arg(tx_type)))
  AND (sqlc.arg(reference) = '' OR LOWER(reference) = LOWER(sqlc.arg(reference)))
)
ORDER BY spend_time DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);
