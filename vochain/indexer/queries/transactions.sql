-- name: CreateTransaction :execresult
INSERT INTO transactions (
	hash, block_height, block_index, type, subtype, raw_tx, signature, signer
) VALUES (
	?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT(hash) DO UPDATE
SET block_height = excluded.block_height,
    block_index  = excluded.block_index,
    type         = excluded.type,
    subtype      = excluded.subtype,
    raw_tx       = excluded.raw_tx,
    signature    = excluded.signature,
    signer       = excluded.signer;


-- name: GetTransactionByHash :one
SELECT * FROM transactions
WHERE hash = ?
LIMIT 1;

-- name: CountTransactions :one
SELECT COUNT(*) FROM transactions;

-- name: CountTransactionsByType :many
-- Counts the indexed transactions grouped by their type, in one pass over
-- index_transactions_type. Types with no transaction are simply absent.
SELECT type, COUNT(*) AS count
FROM transactions
GROUP BY type
ORDER BY type;

-- name: CountTransactionsByHeight :one
SELECT COUNT(*) FROM transactions
WHERE block_height = ?;

-- name: GetTransactionByHeightAndIndex :one
SELECT * FROM transactions
WHERE block_height = ? AND block_index = ?
LIMIT 1;

-- name: ListTransactionsByTypeAndSubtype :many
-- Returns the stored transactions of a given type and subtype, walking
-- index_transactions_type. Used to backfill process columns derived from a
-- transaction body, which only the raw transaction holds.
SELECT hash, block_height, raw_tx FROM transactions
WHERE type = sqlc.arg(tx_type) AND subtype = sqlc.arg(tx_subtype);

-- name: SearchTransactions :many
SELECT *, COUNT(*) OVER() AS total_count
FROM transactions
WHERE
  (sqlc.arg(block_height) = 0 OR block_height = sqlc.arg(block_height))
  AND (sqlc.arg(tx_type) = '' OR LOWER(type) = LOWER(sqlc.arg(tx_type)))
  AND (sqlc.arg(tx_subtype) = '' OR LOWER(subtype) = LOWER(sqlc.arg(tx_subtype)))
  AND (sqlc.arg(tx_signer) = '' OR LOWER(HEX(signer)) = LOWER(sqlc.arg(tx_signer)))
  AND (
    sqlc.arg(hash_substr) = ''
    OR (LENGTH(sqlc.arg(hash_substr)) = 64 AND LOWER(HEX(hash)) = LOWER(sqlc.arg(hash_substr)))
    OR (LENGTH(sqlc.arg(hash_substr)) < 64 AND INSTR(LOWER(HEX(hash)), LOWER(sqlc.arg(hash_substr))) > 0)
    -- TODO: consider keeping an hash_hex column for faster searches
  )
ORDER BY block_height DESC, block_index DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);
