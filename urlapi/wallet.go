package urlapi

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	WalletHandler = "wallet"
)

func (u *URLAPI) enableWalletHandlers() error {
	if err := u.api.RegisterMethod(
		"/wallet/create/{privateKey}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.walletCreateHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/wallet/transfer/{privateKey}/{dstAddress}/{amount}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.walletTransferHandler,
	); err != nil {
		return err
	}

	return nil
}

// /wallet/create/{privKey}
// set a new account
func (u *URLAPI) walletCreateHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	wallet := ethereum.SignKeys{}
	if err := wallet.AddHexKey(ctx.URLParam("privateKey")); err != nil {
		return err
	}
	if acc, _ := u.vocapp.State.GetAccount(wallet.Address(), true); acc != nil {
		return fmt.Errorf("account %s already exist", wallet.AddressString())
	}
	tx := &models.SetAccountInfoTx{
		Txtype:        models.TxType_SET_ACCOUNT_INFO,
		Nonce:         0,
		InfoURI:       "none",
		Account:       wallet.Address().Bytes(),
		FaucetPackage: nil,
	}

	stx := models.SignedTx{}
	var err error
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetAccountInfo{SetAccountInfo: tx}})
	if err != nil {
		return err
	}

	if stx.Signature, err = wallet.SignVocdoniTx(stx.Tx, u.vocapp.ChainID()); err != nil {
		return err
	}
	txData, err := proto.Marshal(&stx)
	if err != nil {
		return err
	}

	resp, err := u.vocapp.SendTx(txData)
	if err != nil {
		return err
	}

	if resp == nil {
		return fmt.Errorf("no reply from vochain")
	}
	if resp.Code != 0 {
		return fmt.Errorf("%s", string(resp.Data))
	}
	var data []byte
	if data, err = json.Marshal(Transaction{
		Address:  wallet.Address().Bytes(),
		Response: resp.Data.Bytes(),
		Hash:     resp.Hash.Bytes(),
		Code:     &resp.Code,
	}); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /wallet/transfer/{privateKey}/{dstAddress}/{amount}
// set a new account
func (u *URLAPI) walletTransferHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	wallet := ethereum.SignKeys{}
	if err := wallet.AddHexKey(ctx.URLParam("privateKey")); err != nil {
		return err
	}
	acc, err := u.vocapp.State.GetAccount(wallet.Address(), true)
	if err != nil {
		return err
	}
	if len(util.TrimHex(ctx.URLParam("dstAddress"))) != common.AddressLength*2 {
		return fmt.Errorf("destination address malformed")
	}
	dst := common.HexToAddress(ctx.URLParam("dstAddress"))
	if dstAcc, err := u.vocapp.State.GetAccount(dst, true); dstAcc == nil || err != nil {
		return fmt.Errorf("destination account is unknown")
	}
	amount, err := strconv.ParseUint(ctx.URLParam("amount"), 10, 64)
	if err != nil {
		return err
	}

	tx := &models.SendTokensTx{
		Txtype: models.TxType_SET_ACCOUNT_INFO,
		Nonce:  acc.GetNonce(),
		From:   wallet.Address().Bytes(),
		To:     dst.Bytes(),
		Value:  amount,
	}

	stx := models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SendTokens{SendTokens: tx}})
	if err != nil {
		return err
	}

	if stx.Signature, err = wallet.SignVocdoniTx(stx.Tx, u.vocapp.ChainID()); err != nil {
		return err
	}
	txData, err := proto.Marshal(&stx)
	if err != nil {
		return err
	}

	resp, err := u.vocapp.SendTx(txData)
	if err != nil {
		return err
	}

	if resp == nil {
		return fmt.Errorf("no reply from vochain")
	}
	if resp.Code != 0 {
		return fmt.Errorf("%s", string(resp.Data))
	}
	var data []byte
	if data, err = json.Marshal(Transaction{
		Response: resp.Data.Bytes(),
		Hash:     resp.Hash.Bytes(),
		Code:     &resp.Code,
	}); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
