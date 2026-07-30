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

-- name: CountVotes :one
SELECT COUNT(*) FROM votes;

-- name: VoteActivity :many
-- Aggregates the votes of a process into time buckets, using the block timestamp.
-- The bucket_format argument is a strftime format string which truncates the
-- timestamp to the desired granularity (e.g. '%Y-%m-%dT%H:00:00Z' for hourly).
SELECT CAST(strftime(CAST(sqlc.arg(bucket_format) AS TEXT), b.time) AS TEXT) AS period,
	COUNT(*) AS count
FROM votes AS v
JOIN blocks AS b
	ON v.block_height = b.height
WHERE v.process_id = sqlc.arg(process_id)
GROUP BY period
ORDER BY period;

-- name: HasVotesMissingBlockTime :one
-- Reports whether any indexed vote references a block which is not indexed, and
-- thus cannot be dated. Used once at startup as a completeness check: the scan
-- stops at the first such vote, so it is cheap when the data is complete.
SELECT CAST(EXISTS (
	SELECT 1 FROM votes AS v
	LEFT JOIN blocks AS b
		ON v.block_height = b.height
	WHERE b.height IS NULL
) AS INTEGER) AS incomplete;

-- name: SearchVotes :many
WITH results AS (
	SELECT v.*, t.hash, b.time AS block_time
	FROM votes AS v
	LEFT JOIN transactions AS t
		ON v.block_height = t.block_height
		AND v.block_index = t.block_index
	-- dates every listed vote by its block, the same indexed point lookup on the
	-- blocks primary key that GetVote and VoteActivity already do
	LEFT JOIN blocks AS b
		ON v.block_height = b.height
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
)
SELECT *, COUNT(*) OVER() AS total_count
FROM results
ORDER BY block_height DESC, nullifier ASC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);
