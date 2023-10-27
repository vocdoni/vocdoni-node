// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.23.0
// source: token_transfers.sql

package indexerdb

import (
	"context"
	"database/sql"
	"time"

	"go.vocdoni.io/dvote/types"
)

const createTokenTransfer = `-- name: CreateTokenTransfer :execresult
INSERT INTO token_transfers (
	tx_hash, block_height, from_account,
	to_account, amount, transfer_time
) VALUES (
	?, ?, ?,
	?, ?, ?
)
`

type CreateTokenTransferParams struct {
	TxHash       types.Hash
	BlockHeight  int64
	FromAccount  types.AccountID
	ToAccount    types.AccountID
	Amount       int64
	TransferTime time.Time
}

func (q *Queries) CreateTokenTransfer(ctx context.Context, arg CreateTokenTransferParams) (sql.Result, error) {
	return q.exec(ctx, q.createTokenTransferStmt, createTokenTransfer,
		arg.TxHash,
		arg.BlockHeight,
		arg.FromAccount,
		arg.ToAccount,
		arg.Amount,
		arg.TransferTime,
	)
}

const getTokenTransfer = `-- name: GetTokenTransfer :one
SELECT tx_hash, block_height, from_account, to_account, amount, transfer_time FROM token_transfers
WHERE tx_hash = ?
LIMIT 1
`

func (q *Queries) GetTokenTransfer(ctx context.Context, txHash types.Hash) (TokenTransfer, error) {
	row := q.queryRow(ctx, q.getTokenTransferStmt, getTokenTransfer, txHash)
	var i TokenTransfer
	err := row.Scan(
		&i.TxHash,
		&i.BlockHeight,
		&i.FromAccount,
		&i.ToAccount,
		&i.Amount,
		&i.TransferTime,
	)
	return i, err
}

const getTokenTransfersByFromAccount = `-- name: GetTokenTransfersByFromAccount :many
SELECT tx_hash, block_height, from_account, to_account, amount, transfer_time FROM token_transfers
WHERE from_account = ?1
ORDER BY transfer_time DESC
LIMIT ?3
OFFSET ?2
`

type GetTokenTransfersByFromAccountParams struct {
	FromAccount types.AccountID
	Offset      int64
	Limit       int64
}

func (q *Queries) GetTokenTransfersByFromAccount(ctx context.Context, arg GetTokenTransfersByFromAccountParams) ([]TokenTransfer, error) {
	rows, err := q.query(ctx, q.getTokenTransfersByFromAccountStmt, getTokenTransfersByFromAccount, arg.FromAccount, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TokenTransfer
	for rows.Next() {
		var i TokenTransfer
		if err := rows.Scan(
			&i.TxHash,
			&i.BlockHeight,
			&i.FromAccount,
			&i.ToAccount,
			&i.Amount,
			&i.TransferTime,
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
