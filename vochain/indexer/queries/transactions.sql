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

-- name: CountTransactions :one
SELECT COUNT(*) FROM transactions;

-- name: GetTxReferenceByBlockHeightAndBlockIndex :one
SELECT * FROM transactions
WHERE block_height = ? AND block_index = ?
LIMIT 1;

-- name: SearchTransactions :many
WITH results AS (
  SELECT *
  FROM transactions
  WHERE (
    (sqlc.arg(block_height) = 0 OR block_height = sqlc.arg(block_height))
    AND (sqlc.arg(tx_type) = '' OR LOWER(type) = LOWER(sqlc.arg(tx_type)))
  )
)
SELECT *, COUNT(*) OVER() AS total_count
FROM results
ORDER BY block_height DESC, block_index DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);
