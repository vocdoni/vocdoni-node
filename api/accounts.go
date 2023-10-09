package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	AccountHandler                     = "accounts"
	AccountFetchMetadataTimeoutSeconds = 5
)

func (a *API) enableAccountHandlers() error {
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{address}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.accountHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts",
		"POST",
		apirest.MethodAccessTypePublic,
		a.accountSetHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{organizationID}/elections/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionCountHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{organizationID}/elections/status/{status}/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{organizationID}/elections/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.electionListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{accountID}/transfers/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.tokenTransfersListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{accountID}/fees/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.tokenFeesHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{accountID}/transfers/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.tokenTransfersCountHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.accountCountHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.accountListHandler,
	); err != nil {
		return err
	}

	return nil
}

// accountHandler
//
//	@Summary		Get account
//	@Description	Get account information by its address or public key. The `infoURI` parameter contain where account metadata is uploaded (like avatar, name...). It return also an already parsed "metadata" object from this infoUri.
//	@Description	The `meta` object inside the `metadata` property is left to the user to add random information about the account.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			address	path		string	true	"Account address"
//	@Success		200		{object}	Account
//	@Router			/accounts/{address} [get]
func (a *API) accountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	if len(util.TrimHex(ctx.URLParam("address"))) != common.AddressLength*2 {
		return ErrAddressMalformed
	}
	addr := common.HexToAddress(ctx.URLParam("address"))
	acc, err := a.vocapp.State.GetAccount(addr, true)
	if err != nil || acc == nil {
		return ErrAccountNotFound.With(addr.Hex())
	}

	// Try to retrieve the account info metadata
	accMetadata := &AccountMetadata{}
	if a.storage != nil {
		stgCtx, cancel := context.WithTimeout(context.Background(), AccountFetchMetadataTimeoutSeconds*time.Second)
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

	sik, err := a.vocapp.State.SIKFromAddress(addr)
	if err != nil && !errors.Is(err, state.ErrSIKNotFound) {
		log.Warnf("unknown error getting SIK: %v", err)
		return ErrGettingSIK.WithErr(err)
	}

	var data []byte
	if data, err = json.Marshal(Account{
		Address:       addr.Bytes(),
		Nonce:         acc.GetNonce(),
		Balance:       acc.GetBalance(),
		ElectionIndex: acc.GetProcessIndex(),
		InfoURL:       acc.GetInfoURI(),
		Metadata:      accMetadata,
		SIK:           types.HexBytes(sik),
	}); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// accountSetHandler
//
//	@Summary				Set account
//	@Description.markdown	accountSetHandler
//	@Tags					Accounts
//	@Accept					json
//	@Produce				json
//	@Param					transaction	body		object{txPayload=string,metadata=string}	true	"Transaction payload and metadata object encoded using base64 "
//	@Success				200			{object}	AccountSet
//	@Router					/accounts [post]
func (a *API) accountSetHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
		return ErrMetadataProvidedButNoURI
	}

	var metadataCID string
	if req.Metadata != nil {
		// if election metadata defined, check the format
		metadata := AccountMetadata{}
		if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
			return ErrCantParseMetadataAsJSON.WithErr(err)
		}

		// set metadataCID from metadata bytes
		metadataCID = ipfs.CalculateCIDv1json(req.Metadata)
		// check metadata URI matches metadata content
		if !ipfs.CIDequals(metadataCID, metadataURI) {
			return ErrMetadataURINotMatchContent
		}
	}

	// send the transaction to the blockchain
	res, err := a.sendTx(req.TxPayload)
	if err != nil {
		return err
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
		if !ipfs.CIDequals(cid, metadataCID) {
			log.Errorf("metadata CID does not match metadata content (%s != %s)", cid, metadataCID)
		}
	}

	var data []byte
	if data, err = json.Marshal(resp); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// accountCountHandler
//
//	@Summary		Total number of accounts
//	@Description	Returns the count of total number of existing accounts
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{count=int}
//	@Router			/accounts/count [get]
func (a *API) accountCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count, err := a.indexer.CountTotalAccounts()
	if err != nil {
		return err
	}

	data, err := json.Marshal(
		struct {
			Count uint64 `json:"count"`
		}{Count: count},
	)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionListHandler
//
//	@Summary		List organization elections
//	@Description	List the elections of an organization
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			organizationID	path		string	true	"Specific organizationID"
//	@Param			page			path		number	true	"Define de page number"
//	@Success		200				{object}	object{elections=[]ElectionSummary}
//	@Router			/accounts/{organizationID}/elections/page/{page} [get]
//	/accounts/{organizationID}/elections/status/{status}/page/{page} [post] Endpoint docs generated on docs/models/model.go
func (a *API) electionListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	organizationID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("organizationID")))
	if err != nil || organizationID == nil {
		return ErrCantParseOrgID.Withf("%q", ctx.URLParam("organizationID"))
	}

	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize

	var pids [][]byte
	switch ctx.URLParam("status") {
	case "ready":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, 0, "READY", false)
		if err != nil {
			return ErrCantFetchElectionList.WithErr(err)
		}
	case "paused":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, 0, "PAUSED", false)
		if err != nil {
			return ErrCantFetchElectionList.WithErr(err)
		}
	case "canceled":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, 0, "CANCELED", false)
		if err != nil {
			return ErrCantFetchElectionList.WithErr(err)
		}
	case "ended", "results":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, 0, "RESULTS", false)
		if err != nil {
			return ErrCantFetchElectionList.WithErr(err)
		}
		pids2, err := a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, 0, "ENDED", false)
		if err != nil {
			return ErrCantFetchElectionList.WithErr(err)
		}
		pids = append(pids, pids2...)
	case "":
		pids, err = a.indexer.ProcessList(organizationID, page, MaxPageSize, "", 0, 0, "", false)
		if err != nil {
			return ErrCantFetchElectionList.WithErr(err)
		}
	default:
		return ErrParamStatusMissing
	}

	elections := []*ElectionSummary{}
	for _, pid := range pids {
		procInfo, err := a.indexer.ProcessInfo(pid)
		if err != nil {
			return ErrCantFetchElection.WithErr(err)
		}
		summary := a.electionSummary(procInfo)
		elections = append(elections, &summary)
	}
	data, err := json.Marshal(&Organization{
		Elections: elections,
	})
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// electionCountHandler
//
//	@Summary		Count organization elections
//	@Description	Returns the number of elections for an organization
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			organizationID	path		string	true	"Specific organizationID"
//	@Success		200				{object}	object{count=number}
//	@Router			/accounts/{organizationID}/elections/count [get]
func (a *API) electionCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	organizationID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("organizationID")))
	if err != nil || organizationID == nil {
		return ErrCantParseOrgID.Withf("%q", ctx.URLParam("organizationID"))
	}
	acc, err := a.vocapp.State.GetAccount(common.BytesToAddress(organizationID), true)
	if acc == nil {
		return ErrOrgNotFound
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
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// tokenTransfersListHandler
//
//	@Summary		List account received and sent token transfers
//	@Description	Returns the token transfers for an account. A transfer is a token transference from one account to other (excepting the burn address).
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			accountID	path		string	true	"Specific accountID"
//	@Param			page		path		string	true	"Paginator page"
//	@Success		200			{object}	object{transfers=indexertypes.TokenTransfersAccount}
//	@Router			/accounts/{accountID}/transfers/page/{page} [get]
func (a *API) tokenTransfersListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	accountID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("accountID")))
	if err != nil || accountID == nil {
		return ErrCantParseAccountID.Withf("%q", ctx.URLParam("accountID"))
	}
	acc, err := a.vocapp.State.GetAccount(common.BytesToAddress(accountID), true)
	if acc == nil {
		return ErrAccountNotFound
	}
	if err != nil {
		return err
	}
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize
	transfers, err := a.indexer.GetTokenTransfersByAccount(accountID, int32(page), MaxPageSize)
	if err != nil {
		return ErrCantFetchTokenTransfers.WithErr(err)
	}
	data, err := json.Marshal(
		struct {
			Transfers indexertypes.TokenTransfersAccount `json:"transfers"`
		}{Transfers: transfers},
	)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// tokenFeesHandler
//
//	@Summary		List account token fees
//	@Description	Returns the token fees for an account. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			accountID	path		string	true	"Specific accountID"
//	@Param			page		path		string	true	"Paginator page"
//	@Success		200			{object}	object{fees=[]indexertypes.TokenFeeMeta}
//	@Router			/accounts/{accountID}/fees/page/{page} [get]
func (a *API) tokenFeesHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	accountID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("accountID")))
	if err != nil || accountID == nil {
		return ErrCantParseAccountID.Withf("%q", ctx.URLParam("accountID"))
	}
	acc, err := a.vocapp.State.GetAccount(common.BytesToAddress(accountID), true)
	if acc == nil {
		return ErrAccountNotFound
	}
	if err != nil {
		return err
	}
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize

	fees, err := a.indexer.GetTokenFeesByFromAccount(accountID, int32(page), MaxPageSize)
	if err != nil {
		return ErrCantFetchTokenTransfers.WithErr(err)
	}
	data, err := json.Marshal(
		struct {
			Fees []*indexertypes.TokenFeeMeta `json:"fees"`
		}{Fees: fees},
	)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// tokenTransfersCountHandler
//
//	@Summary		Total number of sent and received transactions
//	@Description	Returns the count of total number of sent and received transactions for an account. A transaction is a token transfer from one account to another existing account
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			accountID	path		string				true	"Specific accountID"
//	@Success		200			{object}	object{count=int}	"Number of transaction sent and received for the account"
//	@Router			/accounts/{accountID}/transfers/count [get]
func (a *API) tokenTransfersCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	accountID, err := hex.DecodeString(util.TrimHex(ctx.URLParam("accountID")))
	if err != nil || accountID == nil {
		return ErrCantParseAccountID.Withf("%q", ctx.URLParam("accountID"))
	}
	acc, err := a.vocapp.State.GetAccount(common.BytesToAddress(accountID), true)
	if acc == nil {
		return ErrAccountNotFound
	}
	if err != nil {
		return err
	}

	count, err := a.indexer.CountTokenTransfersByAccount(accountID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(
		struct {
			Count uint64 `json:"count"`
		}{Count: count},
	)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// accountListHandler
//
//	@Summary		List of the existing accounts
//	@Description	Returns information (address, balance and nonce) of the existing accounts
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			page	path		string	true	"Paginator page"
//	@Success		200		{object}	object{accounts=[]indexertypes.Account}
//	@Router			/accounts/page/{page} [get]
func (a *API) accountListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize
	accounts, err := a.indexer.GetListAccounts(int32(page), MaxPageSize)
	if err != nil {
		return ErrCantFetchTokenTransfers.WithErr(err)
	}
	data, err := json.Marshal(
		struct {
			Accounts []indexertypes.Account `json:"accounts"`
		}{Accounts: accounts},
	)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
