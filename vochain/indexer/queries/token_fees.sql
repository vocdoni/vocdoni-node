-- name: CreateTokenFee :execresult
INSERT INTO token_fees (
	from_account, block_height, reference,
	cost, tx_type, spend_time
) VALUES (
	?, ?, ?,
	?, ?, ?
);

-- name: GetTokenFees :many
SELECT * FROM token_fees
ORDER BY spend_time DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;

-- name: GetTokenFeesByFromAccount :many
SELECT * FROM token_fees
WHERE from_account = sqlc.arg(from_account)
ORDER BY spend_time DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;

-- name: GetTokenFeesByTxType :many
SELECT * FROM token_fees
WHERE tx_type = sqlc.arg(tx_type)
ORDER BY spend_time DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;

-- name: GetTokenFeesByReference :many
SELECT * FROM token_fees
WHERE reference = sqlc.arg(reference)
ORDER BY spend_time DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;
