package apiclient

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
	indexertypes "go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	// DefaultDevelopmentFaucetURL is the default URL for the development faucet which can be used freely.
	DefaultDevelopmentFaucetURL = "https://faucet-azeno.vocdoni.net/faucet/vocdoni/dev/"
	// DefaultDevelopmentFaucetToken is the default token for the development faucet which can be used freely.
	DefaultDevelopmentFaucetToken = "158a58ba-bd3e-479e-b230-2814a34fae8f"
)

var (
	// ErrAccountNotConfigured is returned when the client has not been configured with an account.
	ErrAccountNotConfigured = fmt.Errorf("account not configured")
)

// Treasurer returns the treasurer address.
func (c *HTTPclient) Treasurer() (*api.Account, error) {
	resp, code, err := c.Request(HTTPGET, nil, "accounts", "treasurer")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	acc := &api.Account{}
	err = json.Unmarshal(resp, acc)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

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
	if code != apirest.HTTPstatusOK {
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
// to pay for the costs of the transaction if the blockchain requires it.  Returns the transaction hash.
func (c *HTTPclient) AccountBootstrap(faucetPkg *models.FaucetPackage, metadata *api.AccountMetadata, sik []byte) (types.HexBytes, error) {
	var err error
	var metadataBytes []byte
	var metadataURI string
	if metadata != nil {
		metadataBytes, err = json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("could not marshal metadata: %w", err)
		}
		metadataURI = "ipfs://" + ipfs.CalculateCIDv1json(metadataBytes)
	}

	if sik == nil {
		if sik, err = c.account.Sik(nil); err != nil {
			return nil, fmt.Errorf("could not generate the sik: %w", err)
		}
	}

	// Build the transaction
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetAccount{
			SetAccount: &models.SetAccountTx{
				Txtype:        models.TxType_CREATE_ACCOUNT,
				Nonce:         new(uint32),
				Account:       c.account.Address().Bytes(),
				FaucetPackage: faucetPkg,
				InfoURI:       &metadataURI,
				Sik:           sik,
			},
		}})
	if err != nil {
		return nil, err
	}

	// Sign and send the transaction
	stx.Signature, err = c.account.SignVocdoniTx(stx.Tx, c.ChainID())
	if err != nil {
		return nil, err
	}
	stxb, err := proto.Marshal(&stx)
	if err != nil {
		return nil, err
	}
	resp, code, err := c.Request(HTTPPOST, &api.AccountSet{
		TxPayload: stxb,
		Metadata:  metadataBytes,
	}, "accounts")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	acc := &api.AccountSet{}
	err = json.Unmarshal(resp, acc)
	if err != nil {
		return nil, err
	}

	return acc.TxHash, nil
}

// AccountSetMetadata updates the metadata associated with the account associated with the client.
func (c *HTTPclient) AccountSetMetadata(metadata *api.AccountMetadata) (types.HexBytes, error) {
	var err error
	var metadataBytes []byte
	var metadataURI string
	if metadata != nil {
		metadataBytes, err = json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("could not marshal metadata: %w", err)
		}
		metadataURI = "ipfs://" + ipfs.CalculateCIDv1json(metadataBytes)
	}

	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("account not configured: %w", err)
	}

	// Build the transaction
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetAccount{
			SetAccount: &models.SetAccountTx{
				Txtype:  models.TxType_SET_ACCOUNT_INFO_URI,
				Nonce:   &acc.Nonce,
				Account: c.account.Address().Bytes(),
				InfoURI: &metadataURI,
			},
		}})
	if err != nil {
		return nil, err
	}

	// Sign and send the transaction
	stx.Signature, err = c.account.SignVocdoniTx(stx.Tx, c.ChainID())
	if err != nil {
		return nil, err
	}
	stxb, err := proto.Marshal(&stx)
	if err != nil {
		return nil, err
	}
	resp, code, err := c.Request(HTTPPOST, &api.AccountSet{
		TxPayload: stxb,
		Metadata:  metadataBytes,
	}, "accounts")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	accv := &api.AccountSet{}
	err = json.Unmarshal(resp, accv)
	if err != nil {
		return nil, err
	}

	return accv.TxHash, nil
}

// GetTransfers returns the list of token transfers associated with an account
func (c *HTTPclient) GetTransfers(from common.Address, page, pageSize int) ([]*indexertypes.TokenTransferMeta, error) {
	resp, code, err := c.Request(HTTPGET, nil, "accounts", from.Hex(), "transfers", "page", strconv.Itoa(page))
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	var transfers []*indexertypes.TokenTransferMeta
	if err := json.Unmarshal(resp, &transfers); err != nil {
		return nil, err
	}
	return transfers, nil
}

// SetSik function allows to update the Secret Identity Key for the current
// HTTPClient account. To do that, the function requires a secret user input.
func (c *HTTPclient) SetSik(secret []byte) (types.HexBytes, error) {
	sik, err := c.account.Sik(secret)
	if err != nil {
		return nil, fmt.Errorf("could not generate the sik: %w", err)
	}
	// Build the transaction
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetSik{
			SetSik: &models.SikTx{
				Txtype:  models.TxType_SET_ACCOUNT_SIK,
				Address: c.account.Address().Bytes(),
				Sik:     sik,
			},
		}})
	if err != nil {
		return nil, err
	}
	// Sign and send the transaction
	stx.Signature, err = c.account.SignVocdoniTx(stx.Tx, c.ChainID())
	if err != nil {
		return nil, err
	}
	stxb, err := proto.Marshal(&stx)
	if err != nil {
		return nil, err
	}
	resp, code, err := c.Request(HTTPPOST, &api.Transaction{
		Payload: stxb,
	}, "chain", "transaction")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	accv := &api.Transaction{}
	err = json.Unmarshal(resp, accv)
	if err != nil {
		return nil, err
	}
	return accv.Hash, nil
}

// DelSik function allows to delete the Secret Identity Key for the current
// HTTPClient account if it already has a valid one.
func (c *HTTPclient) DelSik() (types.HexBytes, error) {
	// Build the transaction
	var err error
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_DelSik{
			DelSik: &models.SikTx{
				Txtype:  models.TxType_DEL_ACCOUNT_SIK,
				Address: c.account.Address().Bytes(),
			},
		}})
	if err != nil {
		return nil, err
	}
	// Sign and send the transaction
	stx.Signature, err = c.account.SignVocdoniTx(stx.Tx, c.ChainID())
	if err != nil {
		return nil, err
	}
	stxb, err := proto.Marshal(&stx)
	if err != nil {
		return nil, err
	}
	resp, code, err := c.Request(HTTPDELETE, &api.Transaction{
		Payload: stxb,
	}, "accounts", "sik")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	accv := &api.Transaction{}
	err = json.Unmarshal(resp, accv)
	if err != nil {
		return nil, err
	}
	return accv.Hash, nil
}
