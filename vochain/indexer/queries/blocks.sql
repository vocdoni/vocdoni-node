-- name: CreateBlock :execresult
INSERT INTO blocks(
    chain_id, height, time, hash, proposer_address, last_block_hash
) VALUES (
	?, ?, ?, ?, ?, ?
);

-- name: GetBlockByHeight :one
SELECT * FROM blocks
WHERE height = ?
LIMIT 1;

-- name: GetBlockByHash :one
SELECT * FROM blocks
WHERE hash = ?
LIMIT 1;
