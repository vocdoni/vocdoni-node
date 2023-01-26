// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.16.0
// source: process.sql

package indexerdb

import (
	"context"
	"database/sql"
	"time"

	"go.vocdoni.io/dvote/types"
)

const createProcess = `-- name: CreateProcess :execresult
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

	?, '0', 0,
	'', 0
)
`

type CreateProcessParams struct {
	ID                types.ProcessID
	EntityID          types.EntityID
	EntityIndex       int64
	StartBlock        int64
	EndBlock          int64
	ResultsHeight     int64
	HaveResults       bool
	FinalResults      bool
	CensusRoot        types.CensusRoot
	RollingCensusRoot types.CensusRoot
	RollingCensusSize int64
	MaxCensusSize     int64
	CensusUri         string
	Metadata          string
	CensusOrigin      int64
	Status            int64
	Namespace         int64
	EnvelopePb        types.EncodedProtoBuf
	ModePb            types.EncodedProtoBuf
	VoteOptsPb        types.EncodedProtoBuf
	PrivateKeys       string
	PublicKeys        string
	QuestionIndex     int64
	CreationTime      time.Time
	SourceBlockHeight int64
	SourceNetworkID   string
	ResultsVotes      string
}

func (q *Queries) CreateProcess(ctx context.Context, arg CreateProcessParams) (sql.Result, error) {
	return q.db.ExecContext(ctx, createProcess,
		arg.ID,
		arg.EntityID,
		arg.EntityIndex,
		arg.StartBlock,
		arg.EndBlock,
		arg.ResultsHeight,
		arg.HaveResults,
		arg.FinalResults,
		arg.CensusRoot,
		arg.RollingCensusRoot,
		arg.RollingCensusSize,
		arg.MaxCensusSize,
		arg.CensusUri,
		arg.Metadata,
		arg.CensusOrigin,
		arg.Status,
		arg.Namespace,
		arg.EnvelopePb,
		arg.ModePb,
		arg.VoteOptsPb,
		arg.PrivateKeys,
		arg.PublicKeys,
		arg.QuestionIndex,
		arg.CreationTime,
		arg.SourceBlockHeight,
		arg.SourceNetworkID,
		arg.ResultsVotes,
	)
}

const getEntityCount = `-- name: GetEntityCount :one
SELECT COUNT(DISTINCT entity_id) FROM processes
`

func (q *Queries) GetEntityCount(ctx context.Context) (int64, error) {
	row := q.db.QueryRowContext(ctx, getEntityCount)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getEntityProcessCount = `-- name: GetEntityProcessCount :one
SELECT COUNT(*) FROM processes
WHERE entity_id = ?
`

func (q *Queries) GetEntityProcessCount(ctx context.Context, entityID types.EntityID) (int64, error) {
	row := q.db.QueryRowContext(ctx, getEntityProcessCount, entityID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getProcess = `-- name: GetProcess :one
SELECT id, entity_id, entity_index, start_block, end_block, results_height, have_results, final_results, results_votes, results_weight, results_envelope_height, results_signatures, results_block_height, census_root, rolling_census_root, rolling_census_size, max_census_size, census_uri, metadata, census_origin, status, namespace, envelope_pb, mode_pb, vote_opts_pb, private_keys, public_keys, question_index, creation_time, source_block_height, source_network_id FROM processes
WHERE id = ?
LIMIT 1
`

func (q *Queries) GetProcess(ctx context.Context, id types.ProcessID) (Process, error) {
	row := q.db.QueryRowContext(ctx, getProcess, id)
	var i Process
	err := row.Scan(
		&i.ID,
		&i.EntityID,
		&i.EntityIndex,
		&i.StartBlock,
		&i.EndBlock,
		&i.ResultsHeight,
		&i.HaveResults,
		&i.FinalResults,
		&i.ResultsVotes,
		&i.ResultsWeight,
		&i.ResultsEnvelopeHeight,
		&i.ResultsSignatures,
		&i.ResultsBlockHeight,
		&i.CensusRoot,
		&i.RollingCensusRoot,
		&i.RollingCensusSize,
		&i.MaxCensusSize,
		&i.CensusUri,
		&i.Metadata,
		&i.CensusOrigin,
		&i.Status,
		&i.Namespace,
		&i.EnvelopePb,
		&i.ModePb,
		&i.VoteOptsPb,
		&i.PrivateKeys,
		&i.PublicKeys,
		&i.QuestionIndex,
		&i.CreationTime,
		&i.SourceBlockHeight,
		&i.SourceNetworkID,
	)
	return i, err
}

const getProcessCount = `-- name: GetProcessCount :one
SELECT COUNT(*) FROM processes
`

func (q *Queries) GetProcessCount(ctx context.Context) (int64, error) {
	row := q.db.QueryRowContext(ctx, getProcessCount)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getProcessEnvelopeHeight = `-- name: GetProcessEnvelopeHeight :one
SELECT results_envelope_height FROM processes
WHERE id = ?
LIMIT 1
`

func (q *Queries) GetProcessEnvelopeHeight(ctx context.Context, id types.ProcessID) (int64, error) {
	row := q.db.QueryRowContext(ctx, getProcessEnvelopeHeight, id)
	var results_envelope_height int64
	err := row.Scan(&results_envelope_height)
	return results_envelope_height, err
}

const getProcessStatus = `-- name: GetProcessStatus :one
SELECT status FROM processes
WHERE id = ?
LIMIT 1
`

func (q *Queries) GetProcessStatus(ctx context.Context, id types.ProcessID) (int64, error) {
	row := q.db.QueryRowContext(ctx, getProcessStatus, id)
	var status int64
	err := row.Scan(&status)
	return status, err
}

const getTotalProcessEnvelopeHeight = `-- name: GetTotalProcessEnvelopeHeight :one
SELECT SUM(results_envelope_height) FROM processes
`

func (q *Queries) GetTotalProcessEnvelopeHeight(ctx context.Context) (interface{}, error) {
	row := q.db.QueryRowContext(ctx, getTotalProcessEnvelopeHeight)
	var sum interface{}
	err := row.Scan(&sum)
	return sum, err
}

const getProcessByFinalResults = `-- name: GetProcessByFinalResults :many
SELECT id FROM processes
WHERE final_results = ?
`

func (q *Queries) GetProcessByFinalResults(ctx context.Context, finalResults bool) ([][]byte, error) {
	rows, err := q.db.QueryContext(ctx, getProcessByFinalResults, finalResults)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prcs []types.ProcessID
	for rows.Next() {
		var id types.ProcessID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		prcs = append(prcs, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return prcs, nil
}

const searchEntities = `-- name: SearchEntities :many
SELECT entity_id FROM processes
WHERE (? = '' OR (INSTR(LOWER(HEX(entity_id)), ?) > 0))
ORDER BY creation_time ASC, ID ASC
LIMIT ?
OFFSET ?
`

type SearchEntitiesParams struct {
	EntityIDSubstr string
	Limit          int32
	Offset         int32
}

func (q *Queries) SearchEntities(ctx context.Context, arg SearchEntitiesParams) ([]types.EntityID, error) {
	rows, err := q.db.QueryContext(ctx, searchEntities,
		arg.EntityIDSubstr,
		arg.EntityIDSubstr,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []types.EntityID
	for rows.Next() {
		var entity_id types.EntityID
		if err := rows.Scan(&entity_id); err != nil {
			return nil, err
		}
		items = append(items, entity_id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const searchProcesses = `-- name: SearchProcesses :many
SELECT ID FROM processes
WHERE (? = 0 OR entity_id = ?)
	AND (? = 0 OR namespace = ?)
	AND (? = 0 OR status = ?)
	AND (? = '' OR source_network_id = ?)
	-- TODO(mvdan): consider keeping an id_hex column for faster searches
	AND (? = '' OR (INSTR(LOWER(HEX(id)), ?) > 0))
	AND (? = FALSE OR have_results)
ORDER BY creation_time ASC, ID ASC
LIMIT ?
OFFSET ?
`

type SearchProcessesParams struct {
	EntityIDLen     interface{}
	EntityID        types.EntityID
	Namespace       int64
	Status          int64
	SourceNetworkID string
	IDSubstr        string
	WithResults     interface{}
	Limit           int32
	Offset          int32
}

// TODO(mvdan): when sqlc's parser is better, and does not get confused with
// string types, use:
// WHERE (LENGTH(sqlc.arg(entity_id)) = 0 OR entity_id = sqlc.arg(entity_id))
// TODO(mvdan): use sqlc.arg once limit/offset support it:
// https://github.com/kyleconroy/sqlc/issues/1025
// LIMIT sqlc.arg(limit)
// OFFSET sqlc.arg(offset)
func (q *Queries) SearchProcesses(ctx context.Context, arg SearchProcessesParams) ([]types.ProcessID, error) {
	rows, err := q.db.QueryContext(ctx, searchProcesses,
		arg.EntityIDLen,
		arg.EntityID,
		arg.Namespace,
		arg.Namespace,
		arg.Status,
		arg.Status,
		arg.SourceNetworkID,
		arg.SourceNetworkID,
		arg.IDSubstr,
		arg.IDSubstr,
		arg.WithResults,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []types.ProcessID
	for rows.Next() {
		var id types.ProcessID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const setProcessResultsCancelled = `-- name: SetProcessResultsCancelled :execresult
UPDATE processes
SET have_results = FALSE, final_results = TRUE
WHERE id = ?
`

func (q *Queries) SetProcessResultsCancelled(ctx context.Context, id types.ProcessID) (sql.Result, error) {
	return q.db.ExecContext(ctx, setProcessResultsCancelled, id)
}

const setProcessResultsHeight = `-- name: SetProcessResultsHeight :execresult
UPDATE processes
SET results_height = ?
WHERE id = ?
`

type SetProcessResultsHeightParams struct {
	ResultsHeight int64
	ID            types.ProcessID
}

func (q *Queries) SetProcessResultsHeight(ctx context.Context, arg SetProcessResultsHeightParams) (sql.Result, error) {
	return q.db.ExecContext(ctx, setProcessResultsHeight, arg.ResultsHeight, arg.ID)
}

const setProcessResultsReady = `-- name: SetProcessResultsReady :execresult
UPDATE processes
SET have_results = TRUE, final_results = TRUE,
	results_votes = ?,
	results_weight = ?,
	results_envelope_height = ?,
	results_signatures = ?,
	results_block_height = ?
WHERE id = ?
`

type SetProcessResultsReadyParams struct {
	Votes          string
	Weight         string
	EnvelopeHeight int64
	Signatures     string
	BlockHeight    int64
	ID             types.ProcessID
}

func (q *Queries) SetProcessResultsReady(ctx context.Context, arg SetProcessResultsReadyParams) (sql.Result, error) {
	return q.db.ExecContext(ctx, setProcessResultsReady,
		arg.Votes,
		arg.Weight,
		arg.EnvelopeHeight,
		arg.Signatures,
		arg.BlockHeight,
		arg.ID,
	)
}

const updateProcessFromState = `-- name: UpdateProcessFromState :execresult
UPDATE processes
SET end_block           = ?,
	census_root         = ?,
	rolling_census_root = ?,
	census_uri          = ?,
	private_keys        = ?,
	public_keys         = ?,
	metadata            = ?,
	rolling_census_size = ?,
	status              = ?
WHERE id = ?
`

type UpdateProcessFromStateParams struct {
	EndBlock          int64
	CensusRoot        types.CensusRoot
	RollingCensusRoot types.CensusRoot
	CensusUri         string
	PrivateKeys       string
	PublicKeys        string
	Metadata          string
	RollingCensusSize int64
	Status            int64
	ID                types.ProcessID
}

func (q *Queries) UpdateProcessFromState(ctx context.Context, arg UpdateProcessFromStateParams) (sql.Result, error) {
	return q.db.ExecContext(ctx, updateProcessFromState,
		arg.EndBlock,
		arg.CensusRoot,
		arg.RollingCensusRoot,
		arg.CensusUri,
		arg.PrivateKeys,
		arg.PublicKeys,
		arg.Metadata,
		arg.RollingCensusSize,
		arg.Status,
		arg.ID,
	)
}

const updateProcessResults = `-- name: UpdateProcessResults :execresult
UPDATE processes
SET results_votes = ?,
	results_weight = ?,
	results_envelope_height = ?,
	results_block_height = ?
WHERE id = ? AND final_results = FALSE
`

type UpdateProcessResultsParams struct {
	Votes          string
	Weight         string
	EnvelopeHeight int64
	BlockHeight    int64
	ID             types.ProcessID
}

func (q *Queries) UpdateProcessResults(ctx context.Context, arg UpdateProcessResultsParams) (sql.Result, error) {
	return q.db.ExecContext(ctx, updateProcessResults,
		arg.Votes,
		arg.Weight,
		arg.EnvelopeHeight,
		arg.BlockHeight,
		arg.ID,
	)
}
