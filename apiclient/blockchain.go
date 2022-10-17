package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	ErrTransactionDoesNotExist = fmt.Errorf("transaction does not exist")
)

func (c *HTTPclient) TransactionReference(txHash types.HexBytes) (*api.TransactionReference, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "transaction", "reference", txHash.String())
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, ErrTransactionDoesNotExist
	}
	txRef := &api.TransactionReference{}
	if err := json.Unmarshal(resp, txRef); err != nil {
		return nil, err
	}
	return txRef, nil
}

func (c *HTTPclient) TransactionByHash(txHash types.HexBytes) (*models.Tx, error) {
	ref, err := c.TransactionReference(txHash)
	if err != nil {
		return nil, err
	}
	resp, code, err := c.Request(HTTPGET, nil,
		"chain", "transaction", fmt.Sprintf("%d", ref.Height), fmt.Sprintf("%d", ref.Index))
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%d:could not get raw transaction: %s", code, resp)
	}
	tx := &models.Tx{}
	return tx, protojson.Unmarshal(resp, tx)
}
