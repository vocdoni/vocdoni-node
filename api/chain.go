package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/vochain"
)

const (
	ChainHandler = "chain"
)

func (a *API) enableChainHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/chain/organization/list",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.organizationListHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/organization/list/{page}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.organizationListHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/organization/count",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.organizationCountHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/info",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.chainInfoHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/blockdate/<timestamp>",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.chainEstimateHeightHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transaction/cost",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.chainTxCostHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transaction/submit",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		a.chainSendTxHandler,
	); err != nil {
		return err
	}

	return nil
}

// /chain/organization/list/<page>
// list the existing organizations
func (a *API) organizationListHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
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

	list := a.scrutinizer.EntityList(MaxPageSize, page, "")
	for _, orgID := range list {
		organization.Organizations = append(organization.Organizations, &OrganizationList{
			OrganizationID: orgID,
			ElectionCount:  a.scrutinizer.ProcessCount(orgID),
		})
	}

	var data []byte
	if data, err = json.Marshal(organization); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /chain/organization/count
// return the number of organizations
func (a *API) organizationCountHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	count := a.scrutinizer.EntityCount()
	organization := &Organization{Count: &count}
	data, err := json.Marshal(organization)
	if err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)

}

// /chain/info
// returns the chain ID, blocktimes, timestamp and height of the blockchain
func (a *API) chainInfoHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	blockTimes := a.vocinfo.BlockTimes()
	height := a.vocapp.Height()
	timestamp := a.vocapp.Timestamp()
	data, err := json.Marshal(ChainInfo{
		ID:        a.vocapp.ChainID(),
		BlockTime: blockTimes,
		Height:    &height,
		Timestamp: &timestamp,
	})
	if err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /chain/blockdate/<timestamp>
// returns the estimated block height for the timestamp provided
func (a *API) chainEstimateHeightHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	timestamp, err := strconv.ParseInt(ctx.URLParam("timestamp"), 10, 64)
	if err != nil {
		return err
	}
	height, err := a.vocinfo.EstimateBlockHeight(time.Unix(timestamp, 0))
	if err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Height uint32 `json:"height"`
	}{Height: height},
	)
	if err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /chain/transaction/submit
// submits a blockchain transaction
func (a *API) chainSendTxHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	req := &Transaction{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}
	res, err := a.vocapp.SendTx(req.Payload)
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
func (a *API) chainTxCostHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	txCosts := &Transaction{
		Costs: make(map[string]uint64),
	}
	var err error
	for k, v := range vochain.TxCostNameToTxTypeMap {
		txCosts.Costs[k], err = a.vocapp.State.TxCost(v, true)
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
