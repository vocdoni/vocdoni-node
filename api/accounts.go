package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
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
		"/accounts/treasurer",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		a.treasurerHandler,
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

	// Try to retrieve the account info metadata
	accMetadata := &AccountMetadata{}
	if a.storage != nil {
		stgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		metadataBytes, err := a.storage.Retrieve(stgCtx, acc.InfoURI, MaxOffchainFileSize)
		if err != nil {
			log.Warnf("cannot get account metadata from %s: %v", acc.InfoURI, err)
		} else {
			if err := json.Unmarshal(metadataBytes, &accMetadata); err != nil {
				log.Warnf("cannot unmarshal metadata from %s: %v", acc.InfoURI, err)
			}
		}
	}

	var data []byte
	if data, err = json.Marshal(Account{
		Address:       addr.Bytes(),
		Nonce:         acc.GetNonce(),
		Balance:       acc.GetBalance(),
		ElectionIndex: acc.GetProcessIndex(),
		InfoURL:       acc.GetInfoURI(),
		Metadata:      accMetadata,
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

	// check if the transaction is of the correct type and extract metadata URI
	metadataURI, err := func() (string, error) {
		stx := &models.SignedTx{}
		if err := proto.Unmarshal(req.TxPayload, stx); err != nil {
			return "", err
		}
		tx := &models.Tx{}
		if err := proto.Unmarshal(stx.GetTx(), tx); err != nil {
			return "", err
		}
		if np := tx.GetSetAccount(); np != nil {
			return np.GetInfoURI(), nil
		}
		return "", nil
	}()
	if err != nil {
		return err
	}

	// Check if the tx metadata URI is provided (in case of metadata bytes provided).
	// Note that we enforce the metadata URI to be provided in the tx payload only if
	// req.Metadata is provided, but not in the other direction.
	if req.Metadata != nil && metadataURI == "" {
		return fmt.Errorf("metadata provided but no metadata URI found in transaction")
	}

	var metadataCID string
	if req.Metadata != nil {
		// if election metadata defined, check the format
		metadata := AccountMetadata{}
		if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
			return fmt.Errorf("wrong metadata format: %w", err)
		}

		// set metadataCID from metadata bytes
		metadataCID = data.CalculateIPFSCIDv1json(req.Metadata)
		// check metadata URI matches metadata content
		if !data.IPFSCIDequals(metadataCID, strings.TrimPrefix(metadataURI, "ipfs://")) {
			return fmt.Errorf("metadata URI does not match metadata content")
		}
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

	// if metadata exists, add it to the storage
	if a.storage != nil && req.Metadata != nil {
		sctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		cid, err := a.storage.Publish(sctx, req.Metadata)
		if err != nil {
			log.Errorf("could not publish to storage: %v", err)
		} else {
			resp.MetadataURL = a.storage.URIprefix() + cid
		}
		if cid != metadataCID {
			log.Errorf("metadata CID does not match metadata content (%s != %s)", cid, metadataCID)
		}
	}

	var data []byte
	if data, err = json.Marshal(resp); err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// GET /accounts/treasurer
// get the treasurer address
func (a *API) treasurerHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	acc, err := a.vocapp.State.Treasurer(true)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("treasurer account does not exist")
	}
	data, err := json.Marshal(struct {
		Address types.HexBytes `json:"address"`
	}{Address: acc.GetAddress()})

	if err != nil {
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
	case "ready":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "READY", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	case "paused":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "PAUSED", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	case "canceled":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "CANCELED", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	case "ended", "results":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "RESULTS", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
		pids2, err := a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "ENDED", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
		pids = append(pids, pids2...)
	case "":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, "", "", false)
		if err != nil {
			return fmt.Errorf("cannot fetch election list: %w", err)
		}
	default:
		return fmt.Errorf("missing status parameter or unknown")
	}

	elections, err := a.electionSummaryList(pids...)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&Organization{
		Elections: elections,
	})
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
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
