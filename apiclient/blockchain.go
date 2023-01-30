package apiclient

import (
	"encoding/json"
	"fmt"

	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	//ErrTransactionDoesNotExist is returned when the transaction does not exist
	ErrTransactionDoesNotExist = fmt.Errorf("transaction does not exist")
)

// TransactionsCost returns the current cost for all transactions
func (c *HTTPclient) TransactionsCost() (*api.TransactionsCost, error) {
	resp, code, err := c.Request(HTTPGET, nil, "chain", "transactions", "cost")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	txscost := &api.TransactionsCost{}
	err = json.Unmarshal(resp, txscost)
	if err != nil {
		return nil, err
	}
	return txscost, nil
}

// TransactionCost returns the current cost for the given transaction type
func (c *HTTPclient) TransactionCost(txType models.TxType) (uint32, error) {
	txscost, err := c.TransactionsCost()
	if err != nil {
		return 0, err
	}
	// TODO: lookup the field somehow dinamically instead of this giant switch case
	switch txType {
	case models.TxType_SET_PROCESS_STATUS:
		return txscost.Costs.SetProcessStatus, nil
	case models.TxType_SET_PROCESS_CENSUS:
		return txscost.Costs.SetProcessCensus, nil
	case models.TxType_SET_PROCESS_RESULTS:
		return txscost.Costs.SetProcessResults, nil
	case models.TxType_SET_PROCESS_QUESTION_INDEX:
		return txscost.Costs.SetProcessQuestionIndex, nil
	case models.TxType_SEND_TOKENS:
		return txscost.Costs.SendTokens, nil
	case models.TxType_SET_ACCOUNT_INFO_URI:
		return txscost.Costs.SetAccountInfoURI, nil
	case models.TxType_CREATE_ACCOUNT:
		return txscost.Costs.CreateAccount, nil
	case models.TxType_REGISTER_VOTER_KEY:
		return txscost.Costs.RegisterKey, nil
	case models.TxType_NEW_PROCESS:
		return txscost.Costs.NewProcess, nil
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		return txscost.Costs.AddDelegateForAccount, nil
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		return txscost.Costs.DelDelegateForAccount, nil
	case models.TxType_COLLECT_FAUCET:
		return txscost.Costs.CollectFaucet, nil
	default:
		return 0, fmt.Errorf("transaction type unknown")
	}
}

// // SetTransactionCost sets the transaction cost of a given transaction
// // and returns the txHash of the transaction, or nil and the error
// func (c *HTTPclient) SetTransactionCost(
// 	signer *ethereum.SignKeys,
// 	txType models.TxType,
// 	cost uint32,
// 	nonce uint32,
// ) (txHash types.HexBytes, err error) {
// 	tx := &models.SetTransactionCostsTx{
// 		Txtype: txType,
// 		Nonce:  nonce,
// 		Value:  uint64(cost),
// 	}
// 	stx := models.SignedTx{}
// 	stx.Tx, err = proto.Marshal(
// 		&models.Tx{Payload: &models.Tx_SetTransactionCosts{SetTransactionCosts: tx}})
// 	if err != nil {
// 		return nil, err
// 	}
// 	_, resp, err := c.SignAndSendTx(signer, &stx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if !resp.Ok {
// 		return nil, fmt.Errorf("submitRawTx failed: %s", resp.Message)
// 	}
// 	return resp.Hash, nil
// }

// SubmitTx POSTs the given tx to the API, and returns txHash and response
func (c *HTTPclient) SubmitTx(stx *models.SignedTx) (types.HexBytes, []byte, error) {
	txData, err := proto.Marshal(stx)
	if err != nil {
		return nil, nil, err
	}

	tx := &api.Transaction{Payload: txData}
	resp, code, err := c.Request(HTTPPOST, tx, "chain", "transactions")
	if err != nil {
		return nil, nil, err
	}
	if code != 200 {
		return nil, nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	if err := json.Unmarshal(resp, tx); err != nil {
		return nil, nil, fmt.Errorf("could not decode response: %w", err)
	}
	return tx.Hash, tx.Response, nil
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
