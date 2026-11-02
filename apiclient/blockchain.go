package apiclient

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

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

// TransactionByHash returns the transaction given its hash, decoded from the
// tx field of GET /chain/transactions/{hash}.
//
// Known limitation, shared with vochain-sequencer: the API encodes the bytes
// fields of that tx as hex while protojson decodes them as base64, so bytes
// fields (hashes, signatures, payloads) do not round-trip; scalar fields are
// correct.
func (c *HTTPclient) TransactionByHash(txHash types.HexBytes) (*models.Tx, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "transactions", txHash.String())
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, ErrTransactionDoesNotExist
	}
	txInfo := &api.GenericTransactionWithInfo{}
	if err := json.Unmarshal(resp, txInfo); err != nil {
		return nil, err
	}
	tx := &models.Tx{}
	return tx, protojson.Unmarshal(txInfo.TxContent, tx)
}

// OrganizationsBySearchTermPaginated returns a paginated list of organizations
// that match the given search term.
func (c *HTTPclient) OrganizationsBySearchTermPaginated(
	organizationID types.HexBytes, page int,
) ([]types.HexBytes, error) {
	query := url.Values{
		api.ParamOrganizationId: {organizationID.String()},
		api.ParamPage:           {strconv.Itoa(page)},
	}
	resp, code, err := c.RequestWithQuery(HTTPGET, nil, query, "chain", "organizations")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	orgs := &api.OrganizationsList{}
	if err := json.Unmarshal(resp, orgs); err != nil {
		return nil, err
	}
	ids := make([]types.HexBytes, 0, len(orgs.Organizations))
	for _, org := range orgs.Organizations {
		ids = append(ids, org.OrganizationID)
	}
	return ids, nil
}

// TransactionCount returns the count of transactions, read from
// transactionCount of GET /chain/info.
func (c *HTTPclient) TransactionCount() (uint64, error) {
	info, err := c.ChainInfo()
	if err != nil {
		return 0, err
	}
	return info.TransactionCount, nil
}
