package api

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/util"
)

const (
	AccountHandler = "account"
)

func (a *API) enableAccountHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/account/{address}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.accountHandler,
	); err != nil {
		return err
	}
	return nil
}

// /account/{address}
// get the account information
func (a *API) accountHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	if len(util.TrimHex(ctx.URLParam("address"))) != common.AddressLength*2 {
		return fmt.Errorf("address malformed")
	}
	addr := common.HexToAddress(ctx.URLParam("address"))
	acc, err := a.vocapp.State.GetAccount(addr, true)
	if err != nil || acc == nil {
		return fmt.Errorf("account %s does not exist", addr.Hex())
	}

	var data []byte
	if data, err = json.Marshal(Account{
		Address:       addr.Bytes(),
		Nonce:         acc.GetNonce(),
		Balance:       acc.GetBalance(),
		ElectionIndex: acc.GetProcessIndex(),
		InfoURL:       acc.GetInfoURI(),
	}); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
