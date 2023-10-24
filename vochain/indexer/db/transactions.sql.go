// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.23.0
// source: transactions.sql

package indexerdb

import (
	"context"
	"database/sql"

	"go.vocdoni.io/dvote/types"
)

const countTransactions = `-- name: CountTransactions :one
;

SELECT COUNT(*) FROM transactions
`

func (q *Queries) CountTransactions(ctx context.Context) (int64, error) {
	row := q.queryRow(ctx, q.countTransactionsStmt, countTransactions)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const createTransaction = `-- name: CreateTransaction :execresult
INSERT INTO transactions (
	hash, block_height, block_index, type
) VALUES (
	?, ?, ?, ?
)
`

type CreateTransactionParams struct {
	Hash        types.Hash
	BlockHeight int64
	BlockIndex  int64
	Type        string
}

func (q *Queries) CreateTransaction(ctx context.Context, arg CreateTransactionParams) (sql.Result, error) {
	return q.exec(ctx, q.createTransactionStmt, createTransaction,
		arg.Hash,
		arg.BlockHeight,
		arg.BlockIndex,
		arg.Type,
	)
}

const getLastTransactions = `-- name: GetLastTransactions :many
SELECT id, hash, block_height, block_index, type FROM transactions
ORDER BY id DESC
LIMIT ?
OFFSET ?
`

type GetLastTransactionsParams struct {
	Limit  int64
	Offset int64
}

func (q *Queries) GetLastTransactions(ctx context.Context, arg GetLastTransactionsParams) ([]Transaction, error) {
	rows, err := q.query(ctx, q.getLastTransactionsStmt, getLastTransactions, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Transaction
	for rows.Next() {
		var i Transaction
		if err := rows.Scan(
			&i.ID,
			&i.Hash,
			&i.BlockHeight,
			&i.BlockIndex,
			&i.Type,
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

const getTransaction = `-- name: GetTransaction :one
SELECT id, hash, block_height, block_index, type FROM transactions
WHERE id = ?
LIMIT 1
`

func (q *Queries) GetTransaction(ctx context.Context, id int64) (Transaction, error) {
	row := q.queryRow(ctx, q.getTransactionStmt, getTransaction, id)
	var i Transaction
	err := row.Scan(
		&i.ID,
		&i.Hash,
		&i.BlockHeight,
		&i.BlockIndex,
		&i.Type,
	)
	return i, err
}

const getTransactionByHash = `-- name: GetTransactionByHash :one
SELECT id, hash, block_height, block_index, type FROM transactions
WHERE hash = ?
LIMIT 1
`

func (q *Queries) GetTransactionByHash(ctx context.Context, hash types.Hash) (Transaction, error) {
	row := q.queryRow(ctx, q.getTransactionByHashStmt, getTransactionByHash, hash)
	var i Transaction
	err := row.Scan(
		&i.ID,
		&i.Hash,
		&i.BlockHeight,
		&i.BlockIndex,
		&i.Type,
	)
	return i, err
}

const getTxReferenceByBlockHeightAndBlockIndex = `-- name: GetTxReferenceByBlockHeightAndBlockIndex :one
SELECT id, hash, block_height, block_index, type FROM transactions
WHERE block_height = ? AND block_index = ?
LIMIT 1
`

type GetTxReferenceByBlockHeightAndBlockIndexParams struct {
	BlockHeight int64
	BlockIndex  int64
}

func (q *Queries) GetTxReferenceByBlockHeightAndBlockIndex(ctx context.Context, arg GetTxReferenceByBlockHeightAndBlockIndexParams) (Transaction, error) {
	row := q.queryRow(ctx, q.getTxReferenceByBlockHeightAndBlockIndexStmt, getTxReferenceByBlockHeightAndBlockIndex, arg.BlockHeight, arg.BlockIndex)
	var i Transaction
	err := row.Scan(
		&i.ID,
		&i.Hash,
		&i.BlockHeight,
		&i.BlockIndex,
		&i.Type,
	)
	return i, err
}
