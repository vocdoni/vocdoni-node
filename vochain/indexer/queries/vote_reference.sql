-- name: CreateVoteReference :execresult
REPLACE INTO vote_references (
	nullifier, process_id, height, weight,
	tx_index, voter_id, overwrite_count, creation_time
) VALUES (
	?, ?, ?, ?,
	?, ?, ?, ?
);

-- name: GetVoteReference :one
SELECT vote_references.*, tx_references.hash FROM vote_references
LEFT JOIN tx_references
	ON vote_references.height = tx_references.block_height
	AND vote_references.tx_index = tx_references.tx_block_index
WHERE nullifier = ?
LIMIT 1;

-- name: SearchVoteReferences :many
SELECT vote_references.*, tx_references.hash FROM vote_references
LEFT JOIN tx_references
	ON vote_references.height = tx_references.block_height
	AND vote_references.tx_index = tx_references.tx_block_index
WHERE (sqlc.arg(process_id) = '' OR process_id = sqlc.arg(process_id))
	AND (sqlc.arg(nullifier_substr) = '' OR (INSTR(LOWER(HEX(nullifier)), sqlc.arg(nullifier_substr)) > 0))
ORDER BY height DESC, nullifier ASC
LIMIT ?
OFFSET ?
;
