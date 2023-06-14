-- name: CreateBlock :execresult
INSERT INTO blocks(
    height, time, data_hash
) VALUES (
	?, ?, ?
);

-- name: GetBlock :one
SELECT * FROM blocks
WHERE height = ?
LIMIT 1;
