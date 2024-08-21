-- name: CreateTransaction :execresult
INSERT INTO transactions (
	hash, block_height, block_index, type
) VALUES (
	?, ?, ?, ?
);

-- name: GetTransactionByHash :one
SELECT * FROM transactions
WHERE hash = ?
LIMIT 1;

-- name: GetLastTransactions :many
SELECT * FROM transactions
ORDER BY id DESC
LIMIT ?
OFFSET ?
;

-- name: CountTransactions :one
SELECT COUNT(*) FROM transactions;

-- name: GetTxReferenceByBlockHeightAndBlockIndex :one
SELECT * FROM transactions
WHERE block_height = ? AND block_index = ?
LIMIT 1;

