-- name: CreateVote :execresult
REPLACE INTO votes (
	nullifier, process_id, block_height, block_index,
	weight, voter_id, overwrite_count, creation_time
) VALUES (
	?, ?, ?, ?,
	?, ?, ?, ?
);

-- name: GetVote :one
SELECT votes.*, transactions.hash FROM votes
LEFT JOIN transactions
	ON votes.block_height = transactions.block_height
	AND votes.block_index = transactions.block_index
WHERE nullifier = ?
LIMIT 1;

-- name: CountVotes :one
SELECT COUNT(*) FROM votes;

-- name: CountVotesByProcessID :one
SELECT COUNT(*) FROM votes
WHERE process_id = ?;

-- name: SearchVotes :many
SELECT votes.*, transactions.hash FROM votes
LEFT JOIN transactions
	ON votes.block_height = transactions.block_height
	AND votes.block_index = transactions.block_index
WHERE (sqlc.arg(process_id) = '' OR process_id = sqlc.arg(process_id))
	AND (sqlc.arg(nullifier_substr) = '' OR (INSTR(LOWER(HEX(nullifier)), sqlc.arg(nullifier_substr)) > 0))
ORDER BY votes.block_height DESC, votes.nullifier ASC
LIMIT ?
OFFSET ?
;
