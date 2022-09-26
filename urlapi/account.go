package urlapi

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

func (u *URLAPI) enableAccountHandlers() error {
	if err := u.api.RegisterMethod(
		"/account/{address}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.accountHandler,
	); err != nil {
		return err
	}
	return nil
}

// /account/{address}
// get the account information
func (u *URLAPI) accountHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	if len(util.TrimHex(ctx.URLParam("address"))) != common.AddressLength*2 {
		return fmt.Errorf("address malformed")
	}
	addr := common.HexToAddress(ctx.URLParam("address"))
	acc, err := u.vocapp.State.GetAccount(addr, true)
	if err != nil || acc == nil {
		return fmt.Errorf("account %s does not exist", addr.Hex())
	}
	var data []byte
	if data, err = json.Marshal(Account{
		Address: addr.Bytes(),
		Account: acc,
	}); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
