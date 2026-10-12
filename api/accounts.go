package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	AccountHandler = "accounts"
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
		"/accounts/{organizationId}/elections/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.accountElectionsCountHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{organizationId}/elections/status/{status}/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.accountElectionsListByStatusAndPageHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{organizationId}/elections/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.accountElectionsListByPageHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{accountId}/transfers/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.tokenTransfersListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{accountId}/fees/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.tokenFeesHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts/{accountId}/transfers/count",
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
		a.accountListByPageHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/accounts",
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
//	@Description	Get account information by its address or public key. The `infoURI` parameter contains the URI where the account metadata (avatar, name...) is hosted by its owner.
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
	if err != nil {
		return ErrAccountNotFound.With(addr.Hex())
	}

	if acc == nil {
		return ErrAccountNotFound.With(addr.Hex())
	}

	sik, err := a.vocapp.State.SIKFromAddress(addr)
	if err != nil && !errors.Is(err, state.ErrSIKNotFound) {
		log.Warnf("unknown error getting SIK: %v", err)
		return ErrGettingSIK.WithErr(err)
	}

	_, transfersCount, err := a.indexer.TokenTransfersList(1, 0, hex.EncodeToString(addr.Bytes()), "", "")
	if err != nil {
		return ErrCantFetchTokenTransfers.WithErr(err)
	}

	_, feesCount, err := a.indexer.TokenFeesList(1, 0, "", "", hex.EncodeToString(addr.Bytes()))
	if err != nil {
		return ErrCantFetchTokenFees.WithErr(err)
	}

	var data []byte
	if data, err = json.Marshal(Account{
		Address:        addr.Bytes(),
		Nonce:          acc.GetNonce(),
		Balance:        acc.GetBalance(),
		ElectionIndex:  acc.GetProcessIndex(),
		TransfersCount: transfersCount,
		FeesCount:      feesCount,
		InfoURL:        acc.GetInfoURI(),
		SIK:            types.HexBytes(sik),
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
//	@Param					transaction	body		object{txPayload=string}	true	"Transaction payload encoded using base64"
//	@Success				200			{object}	AccountSet
//	@Router					/accounts [post]
func (a *API) accountSetHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &AccountSet{}
	if msg == nil || msg.Data == nil {
		return ErrCantParseDataAsJSON
	}

	if err := json.Unmarshal(msg.Data, req); err != nil {
		return err
	}

	// check the payload is a well-formed signed transaction
	stx := &models.SignedTx{}
	if req.TxPayload == nil {
		return ErrUnmarshalingServerProto
	}
	if err := proto.Unmarshal(req.TxPayload, stx); err != nil {
		return err
	}
	if stx.GetTx() == nil {
		return ErrUnmarshalingServerProto
	}
	if err := proto.Unmarshal(stx.GetTx(), &models.Tx{}); err != nil {
		return err
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
//	@Deprecated
//	@Description	(deprecated, in favor of /accounts which reports totalItems)
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	CountResult
//	@Router			/accounts/count [get]
func (a *API) accountCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count, err := a.indexer.CountTotalAccounts()
	if err != nil {
		return err
	}
	return marshalAndSend(ctx, &CountResult{Count: count})
}

// accountElectionsListByPageHandler
//
//	@Summary		List organization elections
//	@Description	List the elections of an organization
//	@Deprecated
//	@Description	(deprecated, in favor of /elections?page=xxx&organizationId=xxx)
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			organizationId	path		string	true	"Specific organizationId"
//	@Param			page			path		number	true	"Page"
//	@Success		200				{object}	ElectionsList
//	@Router			/accounts/{organizationId}/elections/page/{page} [get]
func (a *API) accountElectionsListByPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := electionParams(ctx.URLParam,
		ParamPage,
		ParamOrganizationId,
	)
	if err != nil {
		return err
	}

	if params.OrganizationID == "" {
		return ErrMissingParameter
	}

	list, err := a.electionList(params)
	if err != nil {
		// keep the odd legacy behaviour of sending an empty json "{}"" rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return marshalAndSend(ctx, struct{}{})
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// accountElectionsListByStatusAndPageHandler
//
//	@Summary		List organization elections by status
//	@Description	List the elections of an organization by status
//	@Deprecated
//	@Description	(deprecated, in favor of /elections?page=xxx&organizationId=xxx&status=xxx)
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			organizationId	path		string	true	"Specific organizationId"
//	@Param			status			path		string	true	"Election status"	Enums(ready, paused, canceled, ended, results)
//	@Param			page			path		number	true	"Page"
//	@Success		200				{object}	ElectionsList
//	@Router			/accounts/{organizationId}/elections/status/{status}/page/{page} [get]
func (a *API) accountElectionsListByStatusAndPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := electionParams(ctx.URLParam,
		ParamPage,
		ParamStatus,
		ParamOrganizationId,
	)
	if err != nil {
		return err
	}

	if params.OrganizationID == "" || params.Status == "" {
		return ErrMissingParameter
	}

	list, err := a.electionList(params)
	if err != nil {
		// keep the odd legacy behaviour of sending an empty json "{}"" rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return marshalAndSend(ctx, struct{}{})
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// accountElectionsCountHandler
//
//	@Summary		Count organization elections
//	@Description	Returns the number of elections for an organization
//	@Deprecated
//	@Description	(deprecated, in favor of /elections?organizationId=xxx which reports totalItems)
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			organizationId	path		string	true	"Specific organizationId"
//	@Success		200				{object}	CountResult
//	@Router			/accounts/{organizationId}/elections/count [get]
func (a *API) accountElectionsCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	if ctx.URLParam(ParamOrganizationId) == "" {
		return ErrMissingParameter
	}

	organizationID, err := parseHexString(ctx.URLParam(ParamOrganizationId))
	if err != nil {
		return err
	}

	acc, err := a.vocapp.State.GetAccount(common.BytesToAddress(organizationID), true)
	if acc == nil {
		return ErrOrgNotFound
	}
	if err != nil {
		return err
	}
	return marshalAndSend(ctx, &CountResult{Count: uint64(acc.GetProcessIndex())})
}

// tokenTransfersListHandler
//
//	@Summary		List account received and sent token transfers
//	@Description	Returns the token transfers for an account. A transfer is a token transference from one account to other (excepting the burn address).
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/transfers?accountId=xxx&page=xxx)
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			accountId	path		string	true	"Specific accountId that sent or received the tokens"
//	@Param			page		path		number	true	"Page"
//	@Success		200			{object}	TransfersList
//	@Router			/accounts/{accountId}/transfers/page/{page} [get]
func (a *API) tokenTransfersListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseTransfersParams(
		ctx.URLParam(ParamPage),
		"",
		ctx.URLParam(ParamAccountId),
		"",
		"",
	)
	if err != nil {
		return err
	}

	list, err := a.transfersList(params)
	if err != nil {
		// keep legacy behaviour of sending an empty list rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return marshalAndSend(ctx, emptyTransfersList())
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// tokenFeesHandler
//
//	@Summary		List account token fees
//	@Description	Returns the token fees for an account. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/transfers?accountId=xxx&page=xxx)
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			accountId	path		string	true	"Specific accountId"
//	@Param			page		path		number	true	"Page"
//	@Success		200			{object}	FeesList
//	@Router			/accounts/{accountId}/fees/page/{page} [get]
func (a *API) tokenFeesHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseFeesParams(
		ctx.URLParam(ParamPage),
		"",
		"",
		"",
		ctx.URLParam(ParamAccountId),
	)
	if err != nil {
		return err
	}

	if params.AccountID == "" {
		return ErrMissingParameter
	}

	list, err := a.feesList(params)
	if err != nil {
		// keep legacy behaviour of sending an empty list rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return marshalAndSend(ctx, emptyFeesList())
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// tokenTransfersCountHandler
//
//	@Summary		Total number of sent and received transactions
//	@Description	Returns the count of total number of sent and received transactions for an account. A transaction is a token transfer from one account to another existing account
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/transfers?accountId=xxx which reports totalItems)
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			accountId	path		string		true	"Specific accountId"
//	@Success		200			{object}	CountResult	"Number of transaction sent and received for the account"
//	@Router			/accounts/{accountId}/transfers/count [get]
func (a *API) tokenTransfersCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	accountID, err := hex.DecodeString(util.TrimHex(ctx.URLParam(ParamAccountId)))
	if err != nil || accountID == nil {
		return ErrCantParseAccountID.Withf("%q", ctx.URLParam(ParamAccountId))
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
	return marshalAndSend(ctx, &CountResult{Count: count})
}

// accountListByPageHandler
//
//	@Summary		List of the existing accounts
//	@Description	Returns information (address, balance and nonce) of the existing accounts.
//	@Deprecated
//	@Description	(deprecated, in favor of /accounts?page=xxx)
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			page	path		number	true	"Page"
//	@Success		200		{object}	AccountsList
//	@Router			/accounts/page/{page} [get]
func (a *API) accountListByPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseAccountParams(
		ctx.URLParam(ParamPage),
		"",
		"",
	)
	if err != nil {
		return err
	}

	list, err := a.accountList(params)
	if err != nil {
		// keep legacy behaviour of sending an empty list rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return marshalAndSend(ctx, emptyAccountsList())
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// accountListHandler
//
//	@Summary		List of the existing accounts
//	@Description	Returns information (address, balance and nonce) of the existing accounts
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			page		query		number	false	"Page"
//	@Param			limit		query		number	false	"Items per page"
//	@Param			accountId	query		string	false	"Filter by partial accountId"
//	@Success		200			{object}	AccountsList
//	@Router			/accounts [get]
func (a *API) accountListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseAccountParams(
		ctx.QueryParam(ParamPage),
		ctx.QueryParam(ParamLimit),
		ctx.QueryParam(ParamAccountId),
	)
	if err != nil {
		return err
	}

	list, err := a.accountList(params)
	if err != nil {
		return err
	}

	return marshalAndSend(ctx, list)
}

// accountList produces a paginated AccountsList.
//
// Errors returned are always of type APIerror.
func (a *API) accountList(params *AccountParams) (*AccountsList, error) {
	accounts, total, err := a.indexer.AccountList(
		params.Limit,
		params.Page*params.Limit,
		params.AccountID,
	)
	if err != nil {
		return nil, ErrIndexerQueryFailed.WithErr(err)
	}

	pagination, err := calculatePagination(params.Page, params.Limit, total)
	if err != nil {
		return nil, err
	}

	list := &AccountsList{
		Accounts:   accounts,
		Pagination: pagination,
	}
	return list, nil
}

// parseAccountParams returns an AccountParams filled with the passed params
func parseAccountParams(paramPage, paramLimit, paramAccountID string) (*AccountParams, error) {
	pagination, err := parsePaginationParams(paramPage, paramLimit)
	if err != nil {
		return nil, err
	}

	return &AccountParams{
		PaginationParams: pagination,
		AccountID:        util.TrimHex(paramAccountID),
	}, nil
}
