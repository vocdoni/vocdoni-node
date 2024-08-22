package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
)

// ErrTransactionDoesNotExist is returned when the transaction does not exist
var ErrTransactionDoesNotExist = fmt.Errorf("transaction does not exist")

// ChainInfo returns some information about the chain, such as block height.
func (c *HTTPclient) ChainInfo() (*api.ChainInfo, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "info")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	chainInfo := &api.ChainInfo{}
	err = json.Unmarshal(resp, chainInfo)
	if err != nil {
		return nil, err
	}
	return chainInfo, nil
}

// Block returns information about a block, given a height.
func (c *HTTPclient) Block(height uint32) (*api.Block, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "blocks", fmt.Sprintf("%d", height))
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	block := &api.Block{}
	err = json.Unmarshal(resp, block)
	if err != nil {
		return nil, err
	}
	return block, nil
}

// TransactionsCost returns a map with the current cost for all transactions
func (c *HTTPclient) TransactionsCost() (map[string]uint64, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "transactions", "cost")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	txscost := &api.Transaction{
		Costs: make(map[string]uint64),
	}
	err = json.Unmarshal(resp, txscost)
	if err != nil {
		return nil, err
	}
	return txscost.Costs, nil
}

// TransactionCost returns the current cost for the given transaction type
func (c *HTTPclient) TransactionCost(txType models.TxType) (uint64, error) {
	txscost, err := c.TransactionsCost()
	if err != nil {
		return 0, err
	}
	txcost, found := txscost[genesis.TxTypeToCostName(txType)]
	if !found {
		return 0, fmt.Errorf("transaction type not found")
	}
	return txcost, nil
}

// TransactionReference returns the reference of a transaction given its hash.
func (c *HTTPclient) TransactionReference(txHash types.HexBytes) (*api.TransactionReference, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "transactions", "reference", txHash.String())
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, ErrTransactionDoesNotExist
	}
	txRef := &api.TransactionReference{}
	if err := json.Unmarshal(resp, txRef); err != nil {
		return nil, err
	}
	return txRef, nil
}

// TransactionByHash returns the full transaction given its hash.
// For querying if a transaction is included in a block, it is recommended to
// use TransactionReference which is much faster.
func (c *HTTPclient) TransactionByHash(txHash types.HexBytes) (*models.Tx, error) {
	ref, err := c.TransactionReference(txHash)
	if err != nil {
		return nil, err
	}
	resp, code, err := c.Request(HTTPGET, nil,
		"chain", "transactions", fmt.Sprintf("%d", ref.Height), fmt.Sprintf("%d", ref.Index))
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, ErrTransactionDoesNotExist
	}
	tx := &models.Tx{}
	return tx, protojson.Unmarshal(resp, tx)
}

// OrganizationsBySearchTermPaginated returns a paginated list of organizations
// that match the given search term.
func (c *HTTPclient) OrganizationsBySearchTermPaginated(
	organizationID types.HexBytes, page int,
) ([]types.HexBytes, error) {
	// make a post request to /chain/organizations/filter/page/<page> with the organizationID
	// as the body and page as the url parameter
	resp, code, err := c.Request(HTTPPOST,
		organizationID,
		"chain",
		"organizations",
		"filter",
		"page",
		fmt.Sprintf("%d", page))
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	orgs := new(struct {
		Organizations []types.HexBytes `json:"organizations"`
	})
	if err := json.Unmarshal(resp, orgs); err != nil {
		return nil, err
	}
	return orgs.Organizations, nil
}

// TransactionCount returns the count of transactions
func (c *HTTPclient) TransactionCount() (uint64, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "transactions", "count")
	if err != nil {
		return 0, err
	}
	if code != apirest.HTTPstatusOK {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	txsCount := new(struct {
		Count uint64 `json:"count"`
	})

	return txsCount.Count, json.Unmarshal(resp, txsCount)
}
