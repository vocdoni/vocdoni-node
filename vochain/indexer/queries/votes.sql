-- name: CreateVote :execresult
-- REPLACE is DELETE + INSERT, so every column the votes table owns must be listed
-- here: one left out reverts to its default whenever a vote is overwritten.
REPLACE INTO votes (
	nullifier, process_id, block_height, block_index,
	weight, voter_id, overwrite_count,
	encryption_key_indexes, package, memo
) VALUES (
	?, ?, ?, ?,
	?, ?, ?,
	?, ?, ?
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
-- The join on blocks is a LEFT JOIN on purpose: a vote whose block is not indexed
-- cannot be dated, and is reported under the empty period rather than dropped,
-- so that the caller can tell the aggregation apart from the vote count.
-- The from_time and to_time arguments bound the dated votes; undated votes belong
-- to no window, so they are always counted.
SELECT CAST(COALESCE(strftime(CAST(sqlc.arg(bucket_format) AS TEXT), b.time), '') AS TEXT) AS period,
	COUNT(*) AS count
FROM votes AS v
LEFT JOIN blocks AS b
	ON v.block_height = b.height
WHERE v.process_id = sqlc.arg(process_id)
	AND (
		b.height IS NULL
		OR (
			-- datetime() normalizes both sides to UTC, so that a stored timestamp
			-- and a bound written with different zone offsets still compare right.
			(sqlc.arg(from_time) IS NULL OR datetime(b.time) >= datetime(sqlc.arg(from_time)))
			AND (sqlc.arg(to_time) IS NULL OR datetime(b.time) <= datetime(sqlc.arg(to_time)))
		)
	)
GROUP BY period
ORDER BY period;

-- name: VoteBlockHeightBounds :one
-- Returns the height of the oldest and newest indexed vote, and the height of the
-- oldest indexed block. An indexer db recreated or restored later than the chain
-- it indexes holds votes older than its first block, which therefore cannot be
-- dated. Every value is a MIN/MAX over an indexed column, so this is O(log n) and
-- cheap to run on every boot, unlike a scan looking for votes with no block row.
-- A zero means the corresponding table is empty.
SELECT
	CAST(COALESCE((SELECT MIN(block_height) FROM votes), 0) AS INTEGER) AS min_vote_height,
	CAST(COALESCE((SELECT MAX(block_height) FROM votes), 0) AS INTEGER) AS max_vote_height,
	CAST(COALESCE((SELECT MIN(height) FROM blocks), 0) AS INTEGER) AS min_block_height;

-- name: CountUndatedVotesBelowHeight :one
-- Counts the votes below the given height whose block is not indexed. Only used to
-- report how many votes can never be dated, when their blocks are already pruned
-- from the block store, so it only ever scans that (bounded) height range.
SELECT COUNT(*) FROM votes AS v
LEFT JOIN blocks AS b
	ON v.block_height = b.height
WHERE v.block_height < sqlc.arg(height)
	AND b.height IS NULL;

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
