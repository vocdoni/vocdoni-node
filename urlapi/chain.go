package urlapi

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/vochain"
)

const (
	ChainHandler = "chain"
)

func (u *URLAPI) enableChainHandlers() error {
	if err := u.api.RegisterMethod(
		"/chain/organization/list",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.organizationListHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/chain/organization/list/{page}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.organizationListHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/chain/organization/count",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.organizationCountHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/chain/info",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.chainInfoHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/chain/transaction/cost",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.chainTxCostHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/chain/transaction/submit",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		u.chainSendTxHandler,
	); err != nil {
		return err
	}

	return nil
}

// /chain/organization/list/<page>
// list the existing organizations
func (u *URLAPI) organizationListHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return fmt.Errorf("cannot parse page number")
		}
	}
	page = page * MaxPageSize
	organization := &Organization{}
	list := u.scrutinizer.EntityList(MaxPageSize, page, "")
	for _, orgIDstr := range list {
		orgList := &OrganizationList{}
		orgList.OrganizationID, err = hex.DecodeString(orgIDstr)
		if err != nil {
			panic(err)
		}
		orgList.ElectionCount = u.scrutinizer.ProcessCount(orgList.OrganizationID)
		organization.Organizations = append(organization.Organizations, orgList)
	}

	var data []byte
	if data, err = json.Marshal(organization); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /chain/organization/count
// return the number of organizations
func (u *URLAPI) organizationCountHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	count := u.scrutinizer.EntityCount()
	organization := &Organization{Count: &count}
	data, err := json.Marshal(organization)
	if err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)

}

// /chain/info
// returns the chain ID, blocktimes, timestamp and height of the blockchain
func (u *URLAPI) chainInfoHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	blockTimes := u.vocinfo.BlockTimes()
	height := u.vocapp.Height()
	timestamp := u.vocapp.Timestamp()
	data, err := json.Marshal(ChainInfo{
		ID:        u.vocapp.ChainID(),
		BlockTime: blockTimes,
		Height:    &height,
		Timestamp: &timestamp,
	})
	if err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /chain/transaction/submit
// submits a blockchain transaction
func (u *URLAPI) chainSendTxHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	req := &Transaction{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}
	res, err := u.vocapp.SendTx(req.Payload)
	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("no reply from vochain")
	}
	if res.Code != 0 {
		return fmt.Errorf("%s", string(res.Data))
	}
	var data []byte
	if data, err = json.Marshal(Transaction{
		Response: res.Data.Bytes(),
		Code:     &res.Code,
		Hash:     res.Hash.Bytes(),
	}); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /chain/transaction/cost
// returns de list of transactions and its cost
func (u *URLAPI) chainTxCostHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	txCosts := &Transaction{
		Costs: make(map[string]uint64),
	}
	var err error
	for k, v := range vochain.TxCostNameToTxTypeMap {
		txCosts.Costs[k], err = u.vocapp.State.TxCost(v, true)
		if err != nil {
			return err
		}
	}
	var data []byte
	if data, err = json.Marshal(txCosts); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
