-- name: CreateVote :execresult
REPLACE INTO votes (
	nullifier, process_id, block_height, block_index,
	weight, voter_id, overwrite_count,
	encryption_key_indexes, package
) VALUES (
	?, ?, ?, ?,
	?, ?, ?,
	?, ?
);

-- name: GetVote :one
SELECT v.*, t.hash AS tx_hash, b.time AS block_time FROM votes AS v
LEFT JOIN transactions AS t
	ON v.block_height = t.block_height
	AND v.block_index = t.block_index
LEFT JOIN blocks AS b
	ON v.block_height = b.height
WHERE v.nullifier = ?
LIMIT 1;

-- name: CountTotalVotes :one
SELECT COUNT(*)
FROM votes;

-- name: CountVotes :one
SELECT COUNT(*)
FROM votes
WHERE (
	LENGTH(sqlc.arg(process_id_substr)) <= 64 -- if passed arg is longer, then just abort the query
	AND (
		sqlc.arg(process_id_substr) = ''
		OR (LENGTH(sqlc.arg(process_id_substr)) = 64 AND LOWER(HEX(process_id)) = LOWER(sqlc.arg(process_id_substr)))
		OR (LENGTH(sqlc.arg(process_id_substr)) < 64 AND INSTR(LOWER(HEX(process_id)), LOWER(sqlc.arg(process_id_substr))) > 0)
		-- TODO: consider keeping an process_id_hex column for faster searches
	)
	AND LENGTH(sqlc.arg(nullifier_substr)) <= 64 -- if passed arg is longer, then just abort the query
	AND (
		sqlc.arg(nullifier_substr) = ''
		OR (LENGTH(sqlc.arg(nullifier_substr)) = 64 AND LOWER(HEX(nullifier)) = LOWER(sqlc.arg(nullifier_substr)))
		OR (LENGTH(sqlc.arg(nullifier_substr)) < 64 AND INSTR(LOWER(HEX(nullifier)), LOWER(sqlc.arg(nullifier_substr))) > 0)
		-- TODO: consider keeping an nullifier_hex column for faster searches
	)
);

-- name: SearchVotes :many
SELECT v.*, t.hash
FROM votes AS v
LEFT JOIN transactions AS t
	ON v.block_height = t.block_height
	AND v.block_index = t.block_index
WHERE (
	LENGTH(sqlc.arg(process_id_substr)) <= 64 -- if passed arg is longer, then just abort the query
	AND (
		sqlc.arg(process_id_substr) = ''
		OR (LENGTH(sqlc.arg(process_id_substr)) = 64 AND LOWER(HEX(process_id)) = LOWER(sqlc.arg(process_id_substr)))
		OR (LENGTH(sqlc.arg(process_id_substr)) < 64 AND INSTR(LOWER(HEX(process_id)), LOWER(sqlc.arg(process_id_substr))) > 0)
		-- TODO: consider keeping an process_id_hex column for faster searches
	)
	AND LENGTH(sqlc.arg(nullifier_substr)) <= 64 -- if passed arg is longer, then just abort the query
	AND (
		sqlc.arg(nullifier_substr) = ''
		OR (LENGTH(sqlc.arg(nullifier_substr)) = 64 AND LOWER(HEX(nullifier)) = LOWER(sqlc.arg(nullifier_substr)))
		OR (LENGTH(sqlc.arg(nullifier_substr)) < 64 AND INSTR(LOWER(HEX(nullifier)), LOWER(sqlc.arg(nullifier_substr))) > 0)
		-- TODO: consider keeping an nullifier_hex column for faster searches
	)
)
ORDER BY v.block_height DESC, v.nullifier ASC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);
