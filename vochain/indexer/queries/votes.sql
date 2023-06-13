-- name: CreateVote :execresult
REPLACE INTO votes (
	nullifier, process_id, block_height, block_index,
	weight, voter_id, overwrite_count, creation_time
) VALUES (
	?, ?, ?, ?,
	?, ?, ?, ?
);

-- name: GetVote :one
SELECT v.*, t.hash FROM votes AS v
LEFT JOIN transactions AS t
	ON v.block_height = t.block_height
	AND v.block_index = t.block_index
WHERE v.nullifier = ?
LIMIT 1;

-- name: CountVotes :one
SELECT COUNT(*) FROM votes;

-- name: CountVotesByProcessID :one
SELECT COUNT(*) FROM votes
WHERE process_id = ?;

-- name: SearchVotes :many
SELECT v.*, t.hash FROM votes AS v
LEFT JOIN transactions AS t
	ON  v.block_height = t.block_height
	AND v.block_index  = t.block_index
WHERE (sqlc.arg(process_id) = '' OR process_id = sqlc.arg(process_id))
	AND (sqlc.arg(nullifier_substr) = '' OR (INSTR(LOWER(HEX(nullifier)), sqlc.arg(nullifier_substr)) > 0))
ORDER BY v.block_height DESC, v.nullifier ASC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;
