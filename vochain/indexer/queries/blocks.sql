-- name: CreateBlock :execresult
INSERT INTO blocks(
    chain_id, height, time, hash, proposer_address, last_block_hash
) VALUES (
	?, ?, ?, ?, ?, ?
);

-- name: GetBlock :one
SELECT * FROM blocks
WHERE height = ?
LIMIT 1;
