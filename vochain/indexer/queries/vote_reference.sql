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

-- name: SearchVoteReferences :many
SELECT * FROM vote_references
WHERE (sqlc.arg(process_id) = '' OR process_id = sqlc.arg(process_id))
	AND (sqlc.arg(nullifier_substr) = '' OR (INSTR(LOWER(HEX(nullifier)), sqlc.arg(nullifier_substr)) > 0))
ORDER BY height ASC, nullifier ASC
LIMIT ?
OFFSET ?
;
