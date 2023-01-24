-- name: CreateVoteReference :execresult
REPLACE INTO vote_references (
	nullifier, process_id, height, weight,
	tx_index, voter_id, overwrite_count, creation_time
) VALUES (
	?, ?, ?, ?,
	?, ?, ?, ?
);

-- name: GetVoteReference :one
SELECT * FROM vote_references
WHERE nullifier = ?
LIMIT 1;

-- name: GetVoteReferencesByProcessID :many
SELECT * FROM vote_references
WHERE process_id = ?;
