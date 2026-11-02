package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/data/ipfs"
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
		"/accounts/{address}/metadata",
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
//	@Description	Get account information by its address or public key. The `infoURI` parameter contain where account metadata is uploaded (like avatar, name...). It return also an already parsed "metadata" object from this infoUri.
//	@Description	The `meta` object inside the `metadata` property is left to the user to add random information about the account.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			address	path		string	true	"Account address"
//	@Success		200		{object}	Account
//	@Success		200		{object}	AccountMetadata
//	@Router			/accounts/{address} [get]
//	@Router			/accounts/{address}/metadata [get]
func (a *API) accountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	if len(util.TrimHex(ctx.URLParam("address"))) != common.AddressLength*2 {
		return ErrAddressMalformed
	}
	addr := common.HexToAddress(ctx.URLParam("address"))
	acc, err := a.vocapp.State.GetAccount(addr, true)
	if err != nil {
		return ErrAccountNotFound.With(addr.Hex())
	}

	getAccountMetadata := func() *AccountMetadata {
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
		return accMetadata
	}

	if acc == nil {
		return ErrAccountNotFound.With(addr.Hex())
	}

	accMetadata := getAccountMetadata()
	// cache the name and avatar, so the organization list can render them without
	// resolving the metadata of every row
	a.cacheAccountMetadata(addr.Bytes(), accMetadata)

	// take the last word of the URL path to determine the type of request
	// if the last word is "metadata" then return only the account metadata
	// otherwise return the full account information
	if strings.HasSuffix(ctx.Request.URL.Path, "/metadata") {
		if accMetadata == nil {
			return ErrAccountNotFound.With(addr.Hex())
		}
		data, err := json.Marshal(accMetadata)
		if err != nil {
			return err
		}
		return ctx.Send(data, apirest.HTTPstatusOK)
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
		Metadata:       accMetadata,
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
//	@Param					transaction	body		object{txPayload=string,metadata=string}	true	"Transaction payload and metadata object encoded using base64 "
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

	// check if the transaction is of the correct type and extract metadata URI
	metadataURI, err := func() (string, error) {
		stx := &models.SignedTx{}
		if req.TxPayload == nil {
			return "", ErrUnmarshalingServerProto
		}
		if err := proto.Unmarshal(req.TxPayload, stx); err != nil {
			return "", err
		}
		tx := &models.Tx{}
		gotTx := stx.GetTx()
		if gotTx == nil {
			return "", ErrUnmarshalingServerProto
		}
		if err := proto.Unmarshal(gotTx, tx); err != nil {
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
