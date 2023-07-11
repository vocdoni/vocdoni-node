// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.19.0
// source: blocks.sql

package indexerdb

import (
	"context"
	"database/sql"
	"time"
)

const createBlock = `-- name: CreateBlock :execresult
INSERT INTO blocks(
    height, time, data_hash
) VALUES (
	?, ?, ?
)
`

type CreateBlockParams struct {
	Height   int64
	Time     time.Time
	DataHash []byte
}

func (q *Queries) CreateBlock(ctx context.Context, arg CreateBlockParams) (sql.Result, error) {
	return q.db.ExecContext(ctx, createBlock, arg.Height, arg.Time, arg.DataHash)
}

const getBlock = `-- name: GetBlock :one
SELECT height, time, data_hash FROM blocks
WHERE height = ?
LIMIT 1
`

func (q *Queries) GetBlock(ctx context.Context, height int64) (Block, error) {
	row := q.db.QueryRowContext(ctx, getBlock, height)
	var i Block
	err := row.Scan(&i.Height, &i.Time, &i.DataHash)
	return i, err
}
