// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: token_fees.sql

package indexerdb

import (
	"context"
	"database/sql"
	"time"
)

const createTokenFee = `-- name: CreateTokenFee :execresult
INSERT INTO token_fees (
	from_account, block_height, reference,
	cost, tx_type, spend_time
) VALUES (
	?, ?, ?,
	?, ?, ?
)
`

type CreateTokenFeeParams struct {
	FromAccount []byte
	BlockHeight int64
	Reference   string
	Cost        int64
	TxType      string
	SpendTime   time.Time
}

func (q *Queries) CreateTokenFee(ctx context.Context, arg CreateTokenFeeParams) (sql.Result, error) {
	return q.exec(ctx, q.createTokenFeeStmt, createTokenFee,
		arg.FromAccount,
		arg.BlockHeight,
		arg.Reference,
		arg.Cost,
		arg.TxType,
		arg.SpendTime,
	)
}

const searchTokenFees = `-- name: SearchTokenFees :many
WITH results AS (
  SELECT id, block_height, from_account, reference, cost, tx_type, spend_time
  FROM token_fees
  WHERE (
    (?3 = '' OR LOWER(HEX(from_account)) = LOWER(?3))
    AND (?4 = '' OR LOWER(tx_type) = LOWER(?4))
    AND (?5 = '' OR LOWER(reference) = LOWER(?5))
  )
)
SELECT id, block_height, from_account, reference, cost, tx_type, spend_time, COUNT(*) OVER() AS total_count
FROM results
ORDER BY spend_time DESC
LIMIT ?2
OFFSET ?1
`

type SearchTokenFeesParams struct {
	Offset      int64
	Limit       int64
	FromAccount interface{}
	TxType      interface{}
	Reference   interface{}
}

type SearchTokenFeesRow struct {
	ID          int64
	BlockHeight int64
	FromAccount []byte
	Reference   string
	Cost        int64
	TxType      string
	SpendTime   time.Time
	TotalCount  int64
}

func (q *Queries) SearchTokenFees(ctx context.Context, arg SearchTokenFeesParams) ([]SearchTokenFeesRow, error) {
	rows, err := q.query(ctx, q.searchTokenFeesStmt, searchTokenFees,
		arg.Offset,
		arg.Limit,
		arg.FromAccount,
		arg.TxType,
		arg.Reference,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SearchTokenFeesRow
	for rows.Next() {
		var i SearchTokenFeesRow
		if err := rows.Scan(
			&i.ID,
			&i.BlockHeight,
			&i.FromAccount,
			&i.Reference,
			&i.Cost,
			&i.TxType,
			&i.SpendTime,
			&i.TotalCount,
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
