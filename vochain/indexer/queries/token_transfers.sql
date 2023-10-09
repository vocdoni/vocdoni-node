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

-- name: GetTokenTransfersByFromAccount :many
SELECT * FROM token_transfers
WHERE from_account = sqlc.arg(from_account)
ORDER BY transfer_time DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;

-- name: GetTokenTransfersByToAccount :many
SELECT * FROM token_transfers
WHERE to_account = sqlc.arg(to_account)
ORDER BY transfer_time DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;


-- name: CountTokenTransfersByAccount :one
SELECT COUNT(*) FROM token_transfers
WHERE to_account = sqlc.arg(account) OR
	  from_account = sqlc.arg(account);