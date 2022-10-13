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

func (c *HTTPclient) Account(address string) (*api.Account, error) {
	if address == "" {
		address = c.account.AddressString()
	}
	resp, code, err := c.Request(HTTPGET, nil, "account", address)
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

func (c *HTTPclient) Transfer(to common.Address, amount uint64) (types.HexBytes, error) {
	acc, err := c.Account("")
	if err != nil {
		return nil, err
	}
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SendTokens{
			SendTokens: &models.SendTokensTx{
				Txtype: models.TxType_SET_ACCOUNT_INFO,
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

func (c *HTTPclient) AccountBootstrap(faucetPkg []byte) (types.HexBytes, error) {
	var err error
	var faucet *models.FaucetPackage
	if faucetPkg != nil {
		if err := proto.Unmarshal(faucetPkg, faucet); err != nil {
			return nil, fmt.Errorf("could not unmarshal faucet package: %w", err)
		}
	}
	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetAccountInfo{
			SetAccountInfo: &models.SetAccountInfoTx{
				Txtype:        models.TxType_SET_ACCOUNT_INFO,
				Nonce:         0,
				InfoURI:       "none", // TODO: support infoURI
				Account:       c.account.Address().Bytes(),
				FaucetPackage: faucet,
			},
		}})
	if err != nil {
		return nil, err
	}
	txHash, _, err := c.SignAndSendTx(&stx)
	return txHash, err
}
