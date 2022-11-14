package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	AccountHandler = "accounts"
)

func (a *API) enableAccountHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/accounts/{address}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.accountHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/accounts",
		"POST",
		bearerstdapi.MethodAccessTypePublic,
		a.accountSetHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/accounts/{organizationID}/elections",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionListHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/accounts/{organizationID}/elections/count",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionCountHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/accounts/{organizationID}/elections/status/{status}/page/{page}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionListHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/accounts/{organizationID}/elections/status/{status}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionListHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/accounts/{organizationID}/elections/page/{page}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.electionListHandler,
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

// POST /account
// set account information
func (a *API) accountSetHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	req := &AccountSet{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}
	// check if the metadata is of correct type
	metadata := &OrganizationMetadata{}
	if req.Metadata != nil {
		if err := json.Unmarshal(req.Metadata, metadata); err != nil {
			return fmt.Errorf("could not unmarshal metadata: %w", err)
		}
	}
	// check if the transaction is of the correct type
	if ok, err := isTransactionType(req.TxPayload, &models.Tx_SetAccountInfo{}); err != nil {
		return fmt.Errorf("could not check transaction type: %w", err)
	} else if !ok {
		return fmt.Errorf("transaction is not of type SetAccountInfo")
	}
	// send the transaction to the blockchain
	res, err := a.vocapp.SendTx(req.TxPayload)
	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("no reply from vochain")
	}
	if res.Code != 0 {
		return fmt.Errorf("%s", string(res.Data))
	}

	// prepare the reply
	resp := &AccountSet{
		TxHash: res.Hash.Bytes(),
	}

	// if metadata exists, add it to the storage and set he CID in the reply
	if req.Metadata != nil {
		sctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		cid, err := a.storage.Publish(sctx, req.Metadata)
		if err != nil {
			log.Errorf("could not publish to storage: %v", err)
		} else {
			resp.MetadataURL = a.storage.URIprefix() + cid
		}
	}

	var data []byte
	if data, err = json.Marshal(resp); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /accounts/<organizationID>/elections/status/<status>
// list the elections of an organization.
func (a *API) electionListHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	organizationID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("organizationID")))
	if err != nil || organizationID == nil {
		return fmt.Errorf("organizationID (%q) cannot be decoded", ctx.URLParam("organizationID"))
	}

	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return fmt.Errorf("cannot parse page number")
		}
	}
	page = page * MaxPageSize

	var pids [][]byte
	switch ctx.URLParam("status") {
	case "active":
		pids, err = a.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "READY", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	case "paused":
		pids, err = a.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "PAUSED", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	case "ended":
		pids, err = a.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "RESULTS", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
		pids2, err := a.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "ENDED", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
		pids = append(pids, pids2...)
	case "":
		pids, err = a.scrutinizer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	default:
		return fmt.Errorf("missing status parameter or unknown")
	}

	elections, err := a.getProcessSummaryList(pids...)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&Organization{
		OrganizationID: types.HexBytes(organizationID),
		Elections:      elections,
	})
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	if err = ctx.Send(data, bearerstdapi.HTTPstatusCodeOK); err != nil {
		log.Warn(err)
	}
	return ctx.Send(nil, bearerstdapi.HTTPstatusCodeOK)
}

// /accounts/<organizationID>/elections/count
// Returns the number of elections for an organization
func (a *API) electionCountHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	organizationID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("organizationID")))
	if err != nil || organizationID == nil {
		return fmt.Errorf("organizationID (%q) cannot be decoded", ctx.URLParam("organizationID"))
	}
	acc, err := a.vocapp.State.GetAccount(common.BytesToAddress(organizationID), true)
	if acc == nil {
		return fmt.Errorf("organization not found")
	}
	if err != nil {
		return err
	}
	data, err := json.Marshal(
		struct {
			Count uint32 `json:"count"`
		}{Count: acc.GetProcessIndex()},
	)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}
