-- name: CreateProcess :execresult
INSERT INTO processes (
	id, entity_id, start_date, end_date, manually_ended,
	vote_count, have_results, final_results, census_root,
	max_census_size, census_uri, metadata,
	census_origin, status, namespace,
	envelope, mode, vote_opts,
	private_keys, public_keys,
	question_index, creation_time,
	source_block_height, source_network_id,
	from_archive, chain_id,

	results_votes, results_weight, results_block_height
) VALUES (
	?, ?, ?, ?, ?,
	?, ?, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?, ?,
	?, ?,
	?, ?,
	?, ?,
	?, ?,

	?, '"0"', 0
);

-- name: GetProcess :one
SELECT * FROM processes
WHERE id = ?
GROUP BY id
LIMIT 1;

-- name: SearchProcesses :many
SELECT id FROM processes
WHERE (LENGTH(sqlc.arg(entity_id)) = 0 OR entity_id = sqlc.arg(entity_id))
	AND (sqlc.arg(namespace) = 0 OR namespace = sqlc.arg(namespace))
	AND (sqlc.arg(status) = 0 OR status = sqlc.arg(status))
	AND (sqlc.arg(source_network_id) = 0 OR source_network_id = sqlc.arg(source_network_id))
	-- TODO(mvdan): consider keeping an id_hex column for faster searches
	AND (sqlc.arg(id_substr) = '' OR (INSTR(LOWER(HEX(id)), sqlc.arg(id_substr)) > 0))
	AND (sqlc.arg(with_results) = FALSE OR have_results)
ORDER BY creation_time DESC, id ASC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;

-- name: UpdateProcessFromState :execresult
UPDATE processes
SET census_root         = sqlc.arg(census_root),
	census_uri          = sqlc.arg(census_uri),
	private_keys        = sqlc.arg(private_keys),
	public_keys         = sqlc.arg(public_keys),
	metadata            = sqlc.arg(metadata),
	status              = sqlc.arg(status)
WHERE id = sqlc.arg(id);

-- name: GetProcessStatus :one
SELECT status FROM processes
WHERE id = ?
LIMIT 1;

-- name: UpdateProcessResults :execresult
UPDATE processes
SET results_votes = sqlc.arg(votes),
	results_weight = sqlc.arg(weight),
	results_block_height = sqlc.arg(block_height)
WHERE id = sqlc.arg(id) AND final_results = FALSE;

-- name: SetProcessResultsReady :execresult
UPDATE processes
SET have_results = TRUE, final_results = TRUE,
	results_votes = sqlc.arg(votes),
	results_weight = sqlc.arg(weight),
	results_block_height = sqlc.arg(block_height),
	end_date = sqlc.arg(end_date)
WHERE id = sqlc.arg(id);

-- name: SetProcessResultsCancelled :execresult
UPDATE processes
SET have_results = FALSE, final_results = TRUE, 
    end_date = sqlc.arg(end_date),
	manually_ended = sqlc.arg(manually_ended)
WHERE id = sqlc.arg(id);

-- name: SetProcessVoteCount :execresult
UPDATE processes
SET vote_count = sqlc.arg(vote_count)
WHERE id = sqlc.arg(id);

-- name: GetProcessCount :one
SELECT COUNT(*) FROM processes;

-- name: GetEntityCount :one
SELECT COUNT(DISTINCT entity_id) FROM processes;

-- name: SearchEntities :many
SELECT entity_id, COUNT(id) AS process_count FROM processes
WHERE (sqlc.arg(entity_id_substr) = '' OR (INSTR(LOWER(HEX(entity_id)), sqlc.arg(entity_id_substr)) > 0))
GROUP BY entity_id
ORDER BY creation_time DESC, id ASC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset)
;

-- name: GetProcessIDsByFinalResults :many
SELECT id FROM processes
WHERE final_results = ?;

-- name: UpdateProcessResultByID :execresult
UPDATE processes
SET results_votes  = sqlc.arg(votes),
    results_weight = sqlc.arg(weight),
    vote_opts = sqlc.arg(vote_opts),
    envelope = sqlc.arg(envelope)
WHERE id = sqlc.arg(id);

-- name: UpdateProcessEndDate :execresult
UPDATE processes
SET end_date = sqlc.arg(end_date),
	manually_ended = sqlc.arg(manually_ended)
WHERE id = sqlc.arg(id);
