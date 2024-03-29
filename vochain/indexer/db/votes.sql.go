// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: votes.sql

package indexerdb

import (
	"context"
	"database/sql"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/state"
)

const countVotes = `-- name: CountVotes :one
SELECT COUNT(*) FROM votes
`

func (q *Queries) CountVotes(ctx context.Context) (int64, error) {
	row := q.queryRow(ctx, q.countVotesStmt, countVotes)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const createVote = `-- name: CreateVote :execresult
REPLACE INTO votes (
	nullifier, process_id, block_height, block_index,
	weight, voter_id, overwrite_count,
	encryption_key_indexes, package
) VALUES (
	?, ?, ?, ?,
	?, ?, ?,
	?, ?
)
`

type CreateVoteParams struct {
	Nullifier            types.Nullifier
	ProcessID            types.ProcessID
	BlockHeight          int64
	BlockIndex           int64
	Weight               string
	VoterID              state.VoterID
	OverwriteCount       int64
	EncryptionKeyIndexes string
	Package              string
}

func (q *Queries) CreateVote(ctx context.Context, arg CreateVoteParams) (sql.Result, error) {
	return q.exec(ctx, q.createVoteStmt, createVote,
		arg.Nullifier,
		arg.ProcessID,
		arg.BlockHeight,
		arg.BlockIndex,
		arg.Weight,
		arg.VoterID,
		arg.OverwriteCount,
		arg.EncryptionKeyIndexes,
		arg.Package,
	)
}

const getVote = `-- name: GetVote :one
SELECT v.nullifier, v.process_id, v.block_height, v.block_index, v.weight, v.voter_id, v.overwrite_count, v.encryption_key_indexes, v.package, t.hash AS tx_hash, b.time AS block_time FROM votes AS v
LEFT JOIN transactions AS t
	ON v.block_height = t.block_height
	AND v.block_index = t.block_index
LEFT JOIN blocks AS b
	ON v.block_height = b.height
WHERE v.nullifier = ?
LIMIT 1
`

type GetVoteRow struct {
	Nullifier            types.Nullifier
	ProcessID            types.ProcessID
	BlockHeight          int64
	BlockIndex           int64
	Weight               string
	VoterID              state.VoterID
	OverwriteCount       int64
	EncryptionKeyIndexes string
	Package              string
	TxHash               types.Hash
	BlockTime            sql.NullTime
}

func (q *Queries) GetVote(ctx context.Context, nullifier types.Nullifier) (GetVoteRow, error) {
	row := q.queryRow(ctx, q.getVoteStmt, getVote, nullifier)
	var i GetVoteRow
	err := row.Scan(
		&i.Nullifier,
		&i.ProcessID,
		&i.BlockHeight,
		&i.BlockIndex,
		&i.Weight,
		&i.VoterID,
		&i.OverwriteCount,
		&i.EncryptionKeyIndexes,
		&i.Package,
		&i.TxHash,
		&i.BlockTime,
	)
	return i, err
}

const searchVotes = `-- name: SearchVotes :many
SELECT v.nullifier, v.process_id, v.block_height, v.block_index, v.weight, v.voter_id, v.overwrite_count, v.encryption_key_indexes, v.package, t.hash FROM votes AS v
LEFT JOIN transactions AS t
	ON  v.block_height = t.block_height
	AND v.block_index  = t.block_index
WHERE (?1 = '' OR process_id = ?1)
	AND (?2 = '' OR (INSTR(LOWER(HEX(nullifier)), ?2) > 0))
ORDER BY v.block_height DESC, v.nullifier ASC
LIMIT ?4
OFFSET ?3
`

type SearchVotesParams struct {
	ProcessID       interface{}
	NullifierSubstr interface{}
	Offset          int64
	Limit           int64
}

type SearchVotesRow struct {
	Nullifier            types.Nullifier
	ProcessID            types.ProcessID
	BlockHeight          int64
	BlockIndex           int64
	Weight               string
	VoterID              state.VoterID
	OverwriteCount       int64
	EncryptionKeyIndexes string
	Package              string
	Hash                 types.Hash
}

func (q *Queries) SearchVotes(ctx context.Context, arg SearchVotesParams) ([]SearchVotesRow, error) {
	rows, err := q.query(ctx, q.searchVotesStmt, searchVotes,
		arg.ProcessID,
		arg.NullifierSubstr,
		arg.Offset,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SearchVotesRow
	for rows.Next() {
		var i SearchVotesRow
		if err := rows.Scan(
			&i.Nullifier,
			&i.ProcessID,
			&i.BlockHeight,
			&i.BlockIndex,
			&i.Weight,
			&i.VoterID,
			&i.OverwriteCount,
			&i.EncryptionKeyIndexes,
			&i.Package,
			&i.Hash,
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
