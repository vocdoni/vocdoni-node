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
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// DefaultFaucetURLs is a map of default faucet URLs for each network.
var DefaultFaucetURLs = map[string]string{
	"dev":     "https://api-dev.faucet.vocdoni.net/v2/open/claim/",
	"develop": "https://api-dev.faucet.vocdoni.net/v2/open/claim/",
	"stg":     "https://api-stg.faucet.vocdoni.net/v2/open/claim",
	"stage":   "https://api-stg.faucet.vocdoni.net/v2/open/claim",
	"lts":     "https://api-faucet.vocdoni.io/v2/open/claim/",
	"prod":    "https://api-faucet.vocdoni.io/v2/open/claim/",
}

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

// AccountMetadata returns the metadata associated with a Vocdoni account. If address is empty, it returns the information
// about the account associated with the client.
func (c *HTTPclient) AccountMetadata(address string) (*api.AccountMetadata, error) {
	if address == "" {
		if c.account == nil {
			return nil, ErrAccountNotConfigured
		}
		address = c.account.AddressString()
	}
	resp, code, err := c.Request(HTTPGET, nil, "accounts", address, "metadata")
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	acc := &api.AccountMetadata{}
	err = json.Unmarshal(resp, acc)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

// Transfer sends tokens from the account associated with the client to the given address.
// The nonce is automatically calculated from the account information.
// Returns the transaction hash.
func (c *HTTPclient) Transfer(to common.Address, amount uint64) (types.HexBytes, error) {
	acc, err := c.Account("")
	if err != nil {
		return nil, err
	}
	return c.TransferWithNonce(to, amount, acc.Nonce)
}

// TransferWithNonce sends tokens from the account associated with the client to the given address.
// Returns the transaction hash.
func (c *HTTPclient) TransferWithNonce(to common.Address, amount uint64, nonce uint32) (types.HexBytes, error) {
	var err error
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SendTokens{
			SendTokens: &models.SendTokensTx{
				Txtype: models.TxType_SET_ACCOUNT_INFO_URI,
				Nonce:  nonce,
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
		if sik, err = c.account.AccountSIK(nil); err != nil {
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
				SIK:           sik,
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

// AccountSetValidator upgrades to validator status the account associated with the public key provided.
// If the public key is nil, the public key associated with the account client is used.
// Returns the transaction hash.
func (c *HTTPclient) AccountSetValidator(pubKey []byte, name string) (types.HexBytes, error) {
	acc, err := c.Account("")
	if err != nil {
		return nil, fmt.Errorf("account not configured: %w", err)
	}
	if pubKey == nil {
		pubKey = c.account.PublicKey()
	}
	// Build the transaction
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetAccount{
			SetAccount: &models.SetAccountTx{
				Txtype:    models.TxType_SET_ACCOUNT_VALIDATOR,
				Nonce:     &acc.Nonce,
				PublicKey: pubKey,
				Name:      &name,
			},
		},
	},
	)
	if err != nil {
		return nil, fmt.Errorf("could not marshal transaction: %w", err)
	}
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
	}, "chain", "transactions")
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
		},
	},
	)
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

// ListTokenTransfers returns the list of sent and received token transfers associated with an account
func (c *HTTPclient) ListTokenTransfers(account common.Address, page int) (indexertypes.TokenTransfersAccount, error) {
	resp, code, err := c.Request(HTTPGET, nil, "accounts", account.Hex(), "transfers", "page", strconv.Itoa(page))
	if err != nil {
		return indexertypes.TokenTransfersAccount{}, err
	}
	if code != apirest.HTTPstatusOK {
		return indexertypes.TokenTransfersAccount{}, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	tokenTxs := new(struct {
		Transfers indexertypes.TokenTransfersAccount `json:"transfers"`
	})
	if err := json.Unmarshal(resp, &tokenTxs); err != nil {
		return indexertypes.TokenTransfersAccount{}, err
	}
	return tokenTxs.Transfers, nil
}

// SetSIK function allows to update the Secret Identity Key for the current
// HTTPClient account. To do that, the function requires a secret user input.
func (c *HTTPclient) SetSIK(secret []byte) (types.HexBytes, error) {
	sik, err := c.account.AccountSIK(secret)
	if err != nil {
		return nil, fmt.Errorf("could not generate the sik: %w", err)
	}
	// Build the transaction
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetSIK{
			SetSIK: &models.SIKTx{
				Txtype: models.TxType_SET_ACCOUNT_SIK,
				SIK:    sik,
			},
		},
	},
	)
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
	}, "chain", "transactions")
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

// DelSIK function allows to delete the Secret Identity Key for the current
// HTTPClient account if it already has a valid one.
func (c *HTTPclient) DelSIK() (types.HexBytes, error) {
	// Build the transaction
	var err error
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_DelSIK{
			DelSIK: &models.SIKTx{
				Txtype: models.TxType_DEL_ACCOUNT_SIK,
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

// RegisterSIKForVote function performs the free RegisterSIKTx to the vochain
// helping to non registered accounts to vote in an ongoing election, but only
// if the account is in the election census. The function returns the hash of
// the sent transaction, and requires the election ID. The census proof and the
// secret are optional. If no proof is provided, it will be generated.
func (c *HTTPclient) RegisterSIKForVote(electionId types.HexBytes, proof *CensusProof, secret []byte) (types.HexBytes, error) {
	// get process info
	process, err := c.Election(electionId)
	if err != nil {
		return nil, fmt.Errorf("error getting election info: %w", err)
	}
	// if any proof has been provided, get it from the API
	if proof == nil {
		proof, err = c.CensusGenProof(process.Census.CensusRoot, c.account.Address().Bytes())
		if err != nil {
			return nil, fmt.Errorf("error generating census proof: %w", err)
		}
	}
	// get the account SIK using the secret provided
	sik, err := c.account.AccountSIK(secret)
	if err != nil {
		return nil, fmt.Errorf("error generating SIK: %w", err)
	}
	// compose and encode the transaction
	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_RegisterSIK{
			RegisterSIK: &models.RegisterSIKTx{
				SIK:        sik,
				ElectionId: electionId,
				CensusProof: &models.Proof{
					Payload: &models.Proof_Arbo{
						Arbo: &models.ProofArbo{
							Type:            models.ProofArbo_POSEIDON,
							Siblings:        proof.Proof,
							KeyType:         proof.KeyType,
							AvailableWeight: proof.LeafValue,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error encoding RegisterSIKTx: %w", err)
	}
	// sign it and send it
	hash, _, err := c.SignAndSendTx(stx)
	if err != nil {
		return nil, fmt.Errorf("error signing or sending the Tx: %w", err)
	}
	return hash, nil
}

// ListAccounts return the account list information (address, balance and nonce)
func (c *HTTPclient) ListAccounts(page int) ([]indexertypes.Account, error) {
	resp, code, err := c.Request(HTTPGET, nil, "accounts", "page", strconv.Itoa(page))
	if err != nil {
		return nil, err
	}
	if code != apirest.HTTPstatusOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	accts := new(struct {
		Accounts []indexertypes.Account `json:"accounts"`
	})

	if err := json.Unmarshal(resp, &accts); err != nil {
		return nil, err
	}

	return accts.Accounts, nil
}

// CountAccounts returns the total count of exiting accounts
func (c *HTTPclient) CountAccounts() (uint64, error) {
	resp, code, err := c.Request(HTTPGET, nil, "accounts", "count")
	if err != nil {
		return 0, err
	}
	if code != apirest.HTTPstatusOK {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	accts := new(struct {
		Count uint64 `json:"count"`
	})

	if err := json.Unmarshal(resp, accts); err != nil {
		return 0, err
	}
	return accts.Count, nil
}

// CountTokenTransfers returns the total count of transfers sent and received for an account
func (c *HTTPclient) CountTokenTransfers(accountID common.Address) (uint64, error) {
	resp, code, err := c.Request(HTTPGET, nil, "accounts", accountID.Hex(), "transfers", "count")
	if err != nil {
		return 0, err
	}
	if code != apirest.HTTPstatusOK {
		return 0, fmt.Errorf("%s: %d (%s)", errCodeNot200, code, resp)
	}
	tokentxs := new(struct {
		Count uint64 `json:"count"`
	})

	if err := json.Unmarshal(resp, tokentxs); err != nil {
		return 0, err
	}
	return tokentxs.Count, nil
}
