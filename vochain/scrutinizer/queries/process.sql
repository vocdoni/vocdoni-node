-- name: CreateProcess :execresult
INSERT INTO processes (
	id, entity_id, entity_index, start_block, end_block,
	results_height, have_results, final_results,
	census_root, rolling_census_root, rolling_census_size,
	max_census_size, census_uri, metadata,
	census_origin, status, namespace,
	envelope_pb, mode_pb, vote_opts_pb,
	private_keys, public_keys,
	question_index, creation_time,
	source_block_height, source_network_id,

	results_votes, results_weight, results_envelope_height,
	results_signatures, results_block_height
) VALUES (
	?, ?, ?, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?,
	?, ?,
	?, ?,

	?, "0", 0,
	"", 0
);

-- name: GetProcess :one
SELECT * FROM processes
WHERE id = ?
LIMIT 1;

-- name: SearchProcesses :many
SELECT ID FROM processes
-- TODO(mvdan): drop the hex conversion on entity_id once we can use BLOB for
-- the column and parameter; see sqlc.yaml
WHERE (LENGTH(sqlc.arg(entity_id)) = 0 OR LOWER(HEX(entity_id)) = sqlc.arg(entity_id))
	AND (sqlc.arg(namespace) = 0 OR namespace = sqlc.arg(namespace))
	AND (sqlc.arg(status) = 0 OR status = sqlc.arg(status))
	AND (sqlc.arg(source_network_id) = "" OR source_network_id = sqlc.arg(source_network_id))
	-- TODO(mvdan): consider keeping an id_hex column for faster searches
	AND (sqlc.arg(id_substr) = "" OR (INSTR(LOWER(HEX(id)), sqlc.arg(id_substr)) > 0))
	AND (sqlc.arg(with_results) = FALSE OR have_results)
ORDER BY creation_time ASC, ID ASC
	-- TODO(mvdan): use sqlc.arg once limit/offset support it:
	-- https://github.com/kyleconroy/sqlc/issues/1025
LIMIT ?
OFFSET ?
;

-- name: UpdateProcessFromState :execresult
UPDATE processes
SET end_block           = sqlc.arg(end_block),
	census_root         = sqlc.arg(census_root),
	rolling_census_root = sqlc.arg(rolling_census_root),
	census_uri          = sqlc.arg(census_uri),
	private_keys        = sqlc.arg(private_keys),
	public_keys         = sqlc.arg(public_keys),
	metadata            = sqlc.arg(metadata),
	rolling_census_size = sqlc.arg(rolling_census_size),
	status              = sqlc.arg(status)
WHERE id = sqlc.arg(id);

-- name: GetProcessStatus :one
SELECT status FROM processes
WHERE id = ?
LIMIT 1;

-- name: SetProcessResultsHeight :execresult
UPDATE processes
SET results_height = sqlc.arg(results_height)
WHERE id = sqlc.arg(id);

-- name: UpdateProcessResults :execresult
UPDATE processes
SET results_votes = sqlc.arg(votes),
	results_weight = sqlc.arg(weight),
	results_envelope_height = sqlc.arg(envelope_height),
	results_block_height = sqlc.arg(block_height)
WHERE id = sqlc.arg(id) AND final_results = FALSE;

-- name: SetProcessResultsReady :execresult
UPDATE processes
SET have_results = TRUE, final_results = TRUE,
	results_votes = sqlc.arg(votes),
	results_weight = sqlc.arg(weight),
	results_envelope_height = sqlc.arg(envelope_height),
	results_signatures = sqlc.arg(signatures),
	results_block_height = sqlc.arg(block_height)
WHERE id = sqlc.arg(id);

-- name: SetProcessResultsCancelled :execresult
UPDATE processes
SET have_results = FALSE, final_results = TRUE
WHERE id = sqlc.arg(id);
