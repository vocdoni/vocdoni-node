package apiclient

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

var (
	// ErrAccountNotConfigured is returned when the client has not been configured with an account.
	ErrAccountNotConfigured = fmt.Errorf("account not configured")
)

// Account returns the information about a Vocdoni account. If address is empty, it returns the information
// about the account associated with the client.
func (c *HTTPclient) Account(address string) (*api.Account, error) {
	if address == "" {
		if c.account == nil {
			return nil, ErrAccountNotConfigured
		}
		address = c.account.AddressString()
	}
	resp, code, err := c.Request(HTTPGET, nil, "accounts", address)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	acc := &api.Account{}
	err = json.Unmarshal(resp, acc)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

// Transfer sends tokens from the account associated with the client to the given address.
// Returns the transaction hash.
func (c *HTTPclient) Transfer(to common.Address, amount uint64) (types.HexBytes, error) {
	acc, err := c.Account("")
	if err != nil {
		return nil, err
	}
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SendTokens{
			SendTokens: &models.SendTokensTx{
				Txtype: models.TxType_SET_ACCOUNT_INFO_URI,
				Nonce:  acc.Nonce,
				From:   c.account.Address().Bytes(),
				To:     to.Bytes(),
				Value:  amount,
			},
		}})
	if err != nil {
		return nil, err
	}
	txHash, _, err := c.SignAndSendTx(&stx)
	return txHash, err
}

// AccountBootstrap initializes the account in the Vocdoni blockchain. A faucet package is required in order
// to pay for the costs of the transaction.  Returns the transaction hash.
func (c *HTTPclient) AccountBootstrap(faucetPkg []byte) (types.HexBytes, error) {
	var err error
	var faucetPackageProto *models.FaucetPackage
	if faucetPkg != nil {
		faucetPackageProto = new(models.FaucetPackage)
		if err := proto.Unmarshal(faucetPkg, faucetPackageProto); err != nil {
			return nil, fmt.Errorf("could not unmarshal faucet package: %w", err)
		}
	}
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetAccount{
			SetAccount: &models.SetAccountTx{
				Txtype:        models.TxType_CREATE_ACCOUNT,
				Nonce:         new(uint32),
				Account:       c.account.Address().Bytes(),
				FaucetPackage: faucetPackageProto,
			},
		}})
	if err != nil {
		return nil, err
	}
	txHash, _, err := c.SignAndSendTx(&stx)
	return txHash, err
}
