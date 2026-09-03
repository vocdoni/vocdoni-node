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
	chain_id,

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
	?,

	?, '"0"', 0
);

-- name: GetProcess :one
SELECT * FROM processes
WHERE id = ?
LIMIT 1;

-- name: SearchProcesses :many
WITH results AS (
	SELECT *,
			COUNT(*) OVER() AS total_count
	FROM processes
	WHERE (
		LENGTH(sqlc.arg(entity_id_substr)) <= 40 -- if passed arg is longer, then just abort the query
		AND (
			sqlc.arg(entity_id_substr) = ''
			OR (LENGTH(sqlc.arg(entity_id_substr)) = 40 AND LOWER(HEX(entity_id)) = LOWER(sqlc.arg(entity_id_substr)))
			OR (LENGTH(sqlc.arg(entity_id_substr)) < 40 AND INSTR(LOWER(HEX(entity_id)), LOWER(sqlc.arg(entity_id_substr))) > 0)
			-- TODO: consider keeping an entity_id_hex column for faster searches
		)
		AND (sqlc.arg(namespace) = 0 OR namespace = sqlc.arg(namespace))
		AND (sqlc.arg(status) = 0 OR status = sqlc.arg(status))
		AND (sqlc.arg(source_network_id) = 0 OR source_network_id = sqlc.arg(source_network_id))
		AND LENGTH(sqlc.arg(id_substr)) <= 64 -- if passed arg is longer, then just abort the query
		AND (
			sqlc.arg(id_substr) = ''
			OR (LENGTH(sqlc.arg(id_substr)) = 64 AND LOWER(HEX(id)) = LOWER(sqlc.arg(id_substr)))
			OR (LENGTH(sqlc.arg(id_substr)) < 64 AND INSTR(LOWER(HEX(id)), LOWER(sqlc.arg(id_substr))) > 0)
			-- TODO: consider keeping an id_hex column for faster searches
		)
		AND (
			sqlc.arg(have_results) = -1
			OR (sqlc.arg(have_results) = 1 AND have_results = TRUE)
			OR (sqlc.arg(have_results) = 0 AND have_results = FALSE)
		)
		AND (
			sqlc.arg(final_results) = -1
			OR (sqlc.arg(final_results) = 1 AND final_results = TRUE)
			OR (sqlc.arg(final_results) = 0 AND final_results = FALSE)
		)
		AND (
			sqlc.arg(manually_ended) = -1
			OR (sqlc.arg(manually_ended) = 1 AND manually_ended = TRUE)
			OR (sqlc.arg(manually_ended) = 0 AND manually_ended = FALSE)
		)
		AND (sqlc.arg(start_date_after) IS NULL OR start_date >= sqlc.arg(start_date_after))
		AND (sqlc.arg(start_date_before) IS NULL OR start_date <= sqlc.arg(start_date_before))
		AND (sqlc.arg(end_date_after) IS NULL OR end_date >= sqlc.arg(end_date_after))
		AND (sqlc.arg(end_date_before) IS NULL OR end_date <= sqlc.arg(end_date_before))
	)
)
SELECT id, total_count
FROM results
ORDER BY creation_time DESC, id ASC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

-- name: UpdateProcessFromState :execresult
UPDATE processes
SET census_root         = sqlc.arg(census_root),
	census_uri          = sqlc.arg(census_uri),
	private_keys        = sqlc.arg(private_keys),
	public_keys         = sqlc.arg(public_keys),
	metadata            = sqlc.arg(metadata),
	status              = sqlc.arg(status),
	max_census_size	 	= sqlc.arg(max_census_size),
	end_date 			= sqlc.arg(end_date)
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

-- name: ComputeProcessVoteCount :execresult
UPDATE processes
SET vote_count = (SELECT COUNT(*) FROM votes WHERE process_id = id)
WHERE id = sqlc.arg(id);

-- name: GetProcessCount :one
SELECT COUNT(*) FROM processes;

-- name: SetProcessMetadataTitle :execresult
-- Stores the title resolved from the process off-chain metadata. Only writes when
-- the title actually changed, so the common case of re-resolving the same title
-- costs no write.
UPDATE processes
SET metadata_title = sqlc.arg(metadata_title)
WHERE id = sqlc.arg(id) AND metadata_title != sqlc.arg(metadata_title);

-- name: ListProcessesMissingMetadataTitle :many
-- Lists the processes whose title was never resolved but which do declare a
-- metadata URI, so a backfill knows where to look. Used once per boot. Paged by
-- the process id rather than by an offset, so that a page is never revisited
-- even though rows leave the result set as the backfill fills them.
SELECT id, metadata FROM processes
WHERE metadata_title = '' AND metadata != '' AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(limit);

-- name: SetProcessKeyReveal :execresult
-- Records where the encryption keys of a process were revealed. Keeps the
-- earliest such transaction, since the keys of a multi-key election are revealed
-- one transaction at a time and it is the first one that dates the reveal. Also
-- makes reindexing the same transaction a no-op.
UPDATE processes
SET key_reveal_height = sqlc.arg(key_reveal_height),
    key_reveal_tx_hash = sqlc.arg(key_reveal_tx_hash)
WHERE id = sqlc.arg(id)
  AND (key_reveal_height = 0 OR key_reveal_height > sqlc.arg(key_reveal_height));

-- name: CountProcessesByStatus :many
-- Counts the indexed processes grouped by their status, in one pass over
-- index_processes_status. Statuses with no process are simply absent.
SELECT status, COUNT(*) AS count
FROM processes
GROUP BY status
ORDER BY status;

-- name: GetEntityCount :one
SELECT COUNT(DISTINCT entity_id) FROM processes;

-- name: SearchEntities :many
-- The join to accounts is an indexed point lookup on the accounts primary key,
-- and carries the name and avatar resolved from the account off-chain metadata,
-- so a client listing organizations doesn't need one account request per row.
-- The name filter is a case-insensitive substring match; LOWER only folds ASCII
-- in sqlite, so names differing by non-ASCII case or by diacritics do not match.
--
-- sort_by selects the ordering ('createdAt', 'electionCount' or 'name') and
-- sort_order its direction ('asc' or 'desc'). Both are expected to be one of
-- those exact values; the caller validates them. sqlite cannot parameterize an
-- ORDER BY term and sqlc does not even substitute arguments inside one, so the
-- sort key is computed as a column of the grouped subquery and the outer ORDER
-- BY only picks between the ascending and the descending one, exactly one of
-- which is non-NULL. A term that is NULL on every row compares equal on every
-- row, so it is a no-op.
--
-- 'createdAt' is MIN(creation_time), the creation time of the *first* election
-- indexed for an organization, i.e. when the organization first appeared in the
-- index. That is what the previous hardcoded `ORDER BY creation_time DESC, id
-- ASC` resolved to in practice: creation_time and id were bare columns of a
-- grouped query, so sqlite took them from an arbitrary row of each group, and
-- with the scan driven by index_processes_entity_id that row was the group's
-- first, i.e. its oldest process.
--
-- entity_id is always the last tiebreak, so the ordering is total and paging
-- with LIMIT/OFFSET can neither repeat nor skip a row.
WITH results AS (
    SELECT p.*,
        COALESCE(a.name, '') AS account_name,
        COALESCE(a.avatar, '') AS account_avatar
    FROM processes AS p
    LEFT JOIN accounts AS a
        ON a.account = p.entity_id
    WHERE (sqlc.arg(entity_id_substr) = '' OR (INSTR(LOWER(HEX(p.entity_id)), sqlc.arg(entity_id_substr)) > 0))
    AND (sqlc.arg(name_substr) = '' OR (INSTR(LOWER(COALESCE(a.name, '')), LOWER(sqlc.arg(name_substr))) > 0))
), grouped AS (
    SELECT entity_id,
        account_name,
        account_avatar,
        COUNT(id) AS process_count,
        COUNT(entity_id) OVER() AS total_count,
        -- organizations whose account resolves no name sort last when sorting
        -- by name, in either direction
        CASE WHEN sqlc.arg(sort_by) = 'name' AND account_name = '' THEN 1 END AS sort_name_empty,
        CASE WHEN sqlc.arg(sort_order) = 'asc' THEN (CASE
            WHEN sqlc.arg(sort_by) = 'electionCount' THEN COUNT(id)
            WHEN sqlc.arg(sort_by) = 'name' THEN LOWER(account_name)
            WHEN sqlc.arg(sort_by) = 'createdAt' THEN MIN(creation_time)
        END) END AS sort_key_asc,
        CASE WHEN sqlc.arg(sort_order) = 'desc' THEN (CASE
            WHEN sqlc.arg(sort_by) = 'electionCount' THEN COUNT(id)
            WHEN sqlc.arg(sort_by) = 'name' THEN LOWER(account_name)
            WHEN sqlc.arg(sort_by) = 'createdAt' THEN MIN(creation_time)
        END) END AS sort_key_desc
    FROM results
    GROUP BY entity_id
)
SELECT entity_id,
	account_name,
	account_avatar,
	process_count,
	total_count
FROM grouped
ORDER BY sort_name_empty ASC, sort_key_asc ASC, sort_key_desc DESC, entity_id ASC
LIMIT sqlc.arg(limit)
OFFSET sqlc.arg(offset);

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
