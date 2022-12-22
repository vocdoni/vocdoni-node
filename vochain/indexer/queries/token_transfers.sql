-- name: CreateTokenTransfer :execresult
INSERT INTO token_transfers (
	tx_hash, height, from_account, to_account, amount, transfer_time
) VALUES (
	?, ?, ?, ?, ?, ?
);

-- name: GetTokenTransfer :one
SELECT * FROM token_transfers
WHERE tx_hash = ?
LIMIT 1;

-- name: GetTokenTransfersByFromAccount :many
SELECT * FROM token_transfers
-- the column and parameter; see sqlc.yaml
WHERE from_account = sqlc.arg(from_account)
ORDER BY transfer_time ASC
-- TODO(jordipainan): use sqlc.arg once limit/offset support it:
-- https://github.com/kyleconroy/sqlc/issues/1025
LIMIT ?
OFFSET ?
;