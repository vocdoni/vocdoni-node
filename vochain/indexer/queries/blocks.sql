-- name: CreateBlock :execresult
INSERT INTO blocks(
    chain_id, height, time, hash, proposer_address, last_block_hash
) VALUES (
	?, ?, ?, ?, ?, ?
)
ON CONFLICT(height) DO UPDATE
SET chain_id         = excluded.chain_id,
    time             = excluded.time,
    hash             = excluded.hash,
    proposer_address = excluded.proposer_address,
    last_block_hash  = excluded.last_block_hash;

-- name: GetBlockByHeight :one
SELECT * FROM blocks
WHERE height = ?
LIMIT 1;

-- name: GetBlockByHash :one
SELECT * FROM blocks
WHERE hash = ?
LIMIT 1;

-- name: LastBlockHeight :one
SELECT height FROM blocks
ORDER BY height DESC
LIMIT 1;

-- name: SearchBlocks :many
SELECT
    b.*,
    COUNT(t.block_index) AS tx_count
FROM blocks AS b
LEFT JOIN transactions AS t
    ON b.height = t.block_height
WHERE (
    (sqlc.arg(chain_id) = '' OR b.chain_id = sqlc.arg(chain_id))
    AND LENGTH(sqlc.arg(hash_substr)) <= 64 -- if passed arg is longer, then just abort the query
    AND (
        sqlc.arg(hash_substr) = ''
        OR (LENGTH(sqlc.arg(hash_substr)) = 64 AND LOWER(HEX(b.hash)) = LOWER(sqlc.arg(hash_substr)))
        OR (LENGTH(sqlc.arg(hash_substr)) < 64 AND INSTR(LOWER(HEX(b.hash)), LOWER(sqlc.arg(hash_substr))) > 0)
        -- TODO: consider keeping an hash_hex column for faster searches
    )
    AND (sqlc.arg(proposer_address) = '' OR LOWER(HEX(b.proposer_address)) = LOWER(sqlc.arg(proposer_address)))
)
GROUP BY b.height
ORDER BY b.height DESC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: CountBlocks :one
SELECT COUNT(*)
FROM blocks AS b
WHERE (
    (sqlc.arg(chain_id) = '' OR b.chain_id = sqlc.arg(chain_id))
    AND LENGTH(sqlc.arg(hash_substr)) <= 64 -- if passed arg is longer, then just abort the query
    AND (
        sqlc.arg(hash_substr) = ''
        OR (LENGTH(sqlc.arg(hash_substr)) = 64 AND LOWER(HEX(b.hash)) = LOWER(sqlc.arg(hash_substr)))
        OR (LENGTH(sqlc.arg(hash_substr)) < 64 AND INSTR(LOWER(HEX(b.hash)), LOWER(sqlc.arg(hash_substr))) > 0)
        -- TODO: consider keeping an hash_hex column for faster searches
    )
    AND (sqlc.arg(proposer_address) = '' OR LOWER(HEX(b.proposer_address)) = LOWER(sqlc.arg(proposer_address)))
);
