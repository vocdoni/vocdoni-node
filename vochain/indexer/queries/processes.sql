-- name: CreateProcess :execresult
INSERT INTO processes (
	id, entity_id, start_block, end_block,
	results_height, have_results, final_results,
	census_root, rolling_census_root, rolling_census_size,
	max_census_size, census_uri, metadata,
	census_origin, status, namespace,
	envelope_pb, mode_pb, vote_opts_pb,
	private_keys, public_keys,
	question_index, creation_time,
	source_block_height, source_network_id,

	results_votes, results_weight, results_envelope_height,
	results_block_height
) VALUES (
	?, ?, ?, ?,
	0, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?,
	?, ?,
	?, ?,

	?, '0', 0,
	0
);

-- name: GetProcess :one
SELECT * FROM processes
WHERE id = ?
LIMIT 1;

-- name: SearchProcesses :many
SELECT id FROM processes
-- TODO(mvdan): when sqlc's parser is better, and does not get confused with
-- string types, use:
-- WHERE (LENGTH(sqlc.arg(entity_id)) = 0 OR entity_id = sqlc.arg(entity_id))
WHERE (sqlc.arg(entity_id_len) = 0 OR entity_id = sqlc.arg(entity_id))
	AND (sqlc.arg(namespace) = 0 OR namespace = sqlc.arg(namespace))
	AND (sqlc.arg(status) = 0 OR status = sqlc.arg(status))
	AND (sqlc.arg(source_network_id) = 0 OR source_network_id = sqlc.arg(source_network_id))
	-- TODO(mvdan): consider keeping an id_hex column for faster searches
	AND (sqlc.arg(id_substr) = '' OR (INSTR(LOWER(HEX(id)), sqlc.arg(id_substr)) > 0))
	AND (sqlc.arg(with_results) = FALSE OR have_results)
ORDER BY creation_time DESC, id ASC
-- TODO(mvdan): use sqlc.arg once limit/offset support it:
-- https://github.com/kyleconroy/sqlc/issues/1025
-- LIMIT sqlc.arg(limit)
-- OFFSET sqlc.arg(offset)
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

-- name: GetProcessEnvelopeHeight :one
SELECT results_envelope_height FROM processes
WHERE id = ?
LIMIT 1;

-- name: GetTotalProcessEnvelopeHeight :one
SELECT SUM(results_envelope_height) FROM processes;

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
	results_block_height = sqlc.arg(block_height)
WHERE id = sqlc.arg(id);

-- name: SetProcessResultsCancelled :execresult
UPDATE processes
SET have_results = FALSE, final_results = TRUE
WHERE id = sqlc.arg(id);

-- name: GetProcessCount :one
SELECT COUNT(*) FROM processes;

-- name: GetEntityProcessCount :one
SELECT COUNT(*) FROM processes
WHERE entity_id = sqlc.arg(entity_id);

-- name: GetEntityCount :one
SELECT COUNT(DISTINCT entity_id) FROM processes;

-- name: SearchEntities :many
SELECT DISTINCT entity_id FROM processes
WHERE (sqlc.arg(entity_id_substr) = '' OR (INSTR(LOWER(HEX(entity_id)), sqlc.arg(entity_id_substr)) > 0))
ORDER BY creation_time DESC, id ASC
LIMIT ?
OFFSET ?
;

-- name: GetProcessIDsByFinalResults :many
SELECT id FROM processes
WHERE final_results = ?;

-- name: UpdateProcessResultByID :execresult
UPDATE processes
SET results_votes  = sqlc.arg(votes),
    results_weight = sqlc.arg(weight),
    vote_opts_pb = sqlc.arg(vote_opts_pb),
    envelope_pb = sqlc.arg(envelope_pb)
WHERE id = sqlc.arg(id);
