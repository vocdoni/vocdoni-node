package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	//ErrTransactionDoesNotExist is returned when the transaction does not exist
	ErrTransactionDoesNotExist = fmt.Errorf("transaction does not exist")
)

// TransactionsCost returns a map with the current cost for all transactions
func (c *HTTPclient) TransactionsCost() (map[string]uint64, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "transactions", "cost")
	if err != nil {
		return nil, err
	}
	if code != 200 {
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
	txcost, found := txscost[vochain.TxTypeToCostName(txType)]
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
	if code != 200 {
		return nil, ErrTransactionDoesNotExist
	}
	txRef := &api.TransactionReference{}
	if err := json.Unmarshal(resp, txRef); err != nil {
		return nil, err
	}
	return txRef, nil
}

// TransactionByHash returns the full transaction given its hash.  For querying if a transaction is included in a block,
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
	if code != 200 {
		return nil, fmt.Errorf("%d: could not get raw transaction: %s", code, resp)
	}
	tx := &models.Tx{}
	return tx, protojson.Unmarshal(resp, tx)
}

// OrganizationsBySearchTermPaginated returns a paginated list of organizations that match the given search term.
func (c *HTTPclient) OrganizationsBySearchTermPaginated(organizationID types.HexBytes, page int) ([]types.HexBytes, error) {
	// make a post request to /chain/organizations/filter/page/<page> with the organizationID as the body and page as the url parameter
	resp, code, err := c.Request(HTTPPOST, organizationID, "chain", "organizations", "filter", "page", fmt.Sprintf("%d", page))
	if err != nil {
		return nil, err
	}
	if code != 200 {
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
