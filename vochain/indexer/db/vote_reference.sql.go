// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.17.2
// source: vote_reference.sql

package indexerdb

import (
	"context"
	"database/sql"
	"time"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/state"
)

const createVoteReference = `-- name: CreateVoteReference :execresult
REPLACE INTO vote_references (
	nullifier, process_id, height, weight,
	tx_index, voter_id, overwrite_count, creation_time
) VALUES (
	?, ?, ?, ?,
	?, ?, ?, ?
)
`

type CreateVoteReferenceParams struct {
	Nullifier      types.Nullifier
	ProcessID      types.ProcessID
	Height         int64
	Weight         string
	TxIndex        int64
	VoterID        state.VoterID
	OverwriteCount int64
	CreationTime   time.Time
}

func (q *Queries) CreateVoteReference(ctx context.Context, arg CreateVoteReferenceParams) (sql.Result, error) {
	return q.db.ExecContext(ctx, createVoteReference,
		arg.Nullifier,
		arg.ProcessID,
		arg.Height,
		arg.Weight,
		arg.TxIndex,
		arg.VoterID,
		arg.OverwriteCount,
		arg.CreationTime,
	)
}

const getVoteReference = `-- name: GetVoteReference :one
SELECT nullifier, process_id, height, weight, tx_index, creation_time, voter_id, overwrite_count FROM vote_references
WHERE nullifier = ?
LIMIT 1
`

func (q *Queries) GetVoteReference(ctx context.Context, nullifier types.Nullifier) (VoteReference, error) {
	row := q.db.QueryRowContext(ctx, getVoteReference, nullifier)
	var i VoteReference
	err := row.Scan(
		&i.Nullifier,
		&i.ProcessID,
		&i.Height,
		&i.Weight,
		&i.TxIndex,
		&i.CreationTime,
		&i.VoterID,
		&i.OverwriteCount,
	)
	return i, err
}

const searchVoteReferences = `-- name: SearchVoteReferences :many
SELECT nullifier, process_id, height, weight, tx_index, creation_time, voter_id, overwrite_count FROM vote_references
WHERE (? = '' OR process_id = ?)
	AND (? = '' OR (INSTR(LOWER(HEX(nullifier)), ?) > 0))
ORDER BY height ASC, nullifier ASC
LIMIT ?
OFFSET ?
`

type SearchVoteReferencesParams struct {
	ProcessID       types.ProcessID
	NullifierSubstr string
	Limit           int32
	Offset          int32
}

func (q *Queries) SearchVoteReferences(ctx context.Context, arg SearchVoteReferencesParams) ([]VoteReference, error) {
	rows, err := q.db.QueryContext(ctx, searchVoteReferences,
		arg.ProcessID,
		arg.ProcessID,
		arg.NullifierSubstr,
		arg.NullifierSubstr,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []VoteReference
	for rows.Next() {
		var i VoteReference
		if err := rows.Scan(
			&i.Nullifier,
			&i.ProcessID,
			&i.Height,
			&i.Weight,
			&i.TxIndex,
			&i.CreationTime,
			&i.VoterID,
			&i.OverwriteCount,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
