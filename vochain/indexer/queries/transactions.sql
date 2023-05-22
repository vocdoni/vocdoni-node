-- name: CreateTransaction :execresult
INSERT INTO transactions (
	hash, block_height, block_index, type
) VALUES (
	?, ?, ?, ?
);

-- name: GetTransaction :one
SELECT * FROM transactions
WHERE id = ?
LIMIT 1;

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
