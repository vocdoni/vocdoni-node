-- name: CreateVoteReference :execresult
INSERT INTO vote_references (
	nullifier, process_id, height, weight,
	tx_index, creation_time
) VALUES (
	?, ?, ?, ?,
	?, ?
);

-- name: GetVoteReference :one
SELECT * FROM vote_references
WHERE nullifier = ?
LIMIT 1;
