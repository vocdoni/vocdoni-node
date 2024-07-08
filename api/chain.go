package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	comettypes "github.com/cometbft/cometbft/types"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	ChainHandler = "chain"
)

func (a *API) enableChainHandlers() error {
	if err := a.Endpoint.RegisterMethod(
		"/chain/organizations/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.organizationListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/organizations/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.organizationCountHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/info",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainInfoHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/info/circuit",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainCircuitInfoHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/info/electionPriceFactors",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainInfoPriceFactors,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/dateToBlock/{timestamp}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainEstimateHeightHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/blockToDate/{height}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainEstimateDateHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/cost",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxCostHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/reference/{hash}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxbyHashHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/reference/index/{index}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxByIndexHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/blocks/{height}/transactions/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxByHeightHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/{height}/{index}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions",
		"POST",
		apirest.MethodAccessTypePublic,
		a.chainSendTxHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxListPaginated,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/validators",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainValidatorsHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/blocks/{height}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainBlockHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/blocks/hash/{hash}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainBlockByHashHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/organizations/filter/page/{page}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.chainOrganizationsFilterPaginatedHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxCountHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/fees/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainListFeesHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/fees/reference/{reference}/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainListFeesByReferenceHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/fees/type/{type}/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainListFeesByTypeHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/export/indexer",
		"GET",
		apirest.MethodAccessTypeAdmin,
		a.chainIndexerExportHandler,
	); err != nil {
		return err
	}

	return nil
}

// organizationListHandler
//
//	@Summary				List organizations
//	@Description.markdown	organizationListHandler
//	@Tags					Chain
//	@Accept					json
//	@Produce				json
//	@Param					page	path		int	true	"Page number"
//	@Success				200		{object}	api.organizationListHandler.response
//	@Router					/chain/organizations/page/{page} [get]
func (a *API) organizationListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize
	organizations := []*OrganizationList{}

	list := a.indexer.EntityList(MaxPageSize, page, "")
	for _, org := range list {
		organizations = append(organizations, &OrganizationList{
			OrganizationID: org.EntityID,
			ElectionCount:  uint64(org.ProcessCount),
		})
	}

	type response struct {
		Organizations []*OrganizationList `json:"organizations"`
	}

	data, err := json.Marshal(response{organizations})
	if err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// organizationCountHandler
//
//	@Summary		Count organizations
//	@Description	Return the number of organizations
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{count=int}	"Number of registered organizations"
//	@Router			/chain/organizations/count [get]
func (a *API) organizationCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count := a.indexer.CountTotalEntities()
	organization := &Organization{Count: &count}
	data, err := json.Marshal(organization)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainInfoHandler
//
//	@Summary				Vochain information
//	@Description.markdown	chainInfoHandler
//	@Tags					Chain
//	@Accept					json
//	@Produce				json
//	@Success				200	{object}	api.ChainInfo
//	@Router					/chain/info [get]
func (a *API) chainInfoHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	transactionCount, err := a.indexer.CountTotalTransactions()
	if err != nil {
		return err
	}
	validators, err := a.vocapp.State.Validators(true)
	if err != nil {
		return err
	}
	// TODO: merge the "count total" methods for entities/processes/votes in the indexer
	voteCount, err := a.indexer.CountTotalVotes()
	if err != nil {
		return err
	}
	maxCensusSize, err := a.vocapp.State.MaxProcessSize()
	if err != nil {
		return err
	}
	networkCapacity, err := a.vocapp.State.NetworkCapacity()
	if err != nil {
		return err
	}

	var blockTimesInMs [5]uint64
	for i, v := range a.vocinfo.BlockTimes() {
		blockTimesInMs[i] = uint64(v.Milliseconds())
	}
	if blockTimesInMs[0] == 0 {
		blockTimesInMs[0] = uint64(a.vocapp.BlockTimeTarget().Milliseconds())
	}

	blockStoreBase := uint32(0)
	if a.vocapp.Node != nil && a.vocapp.Node.BlockStore() != nil {
		blockStoreBase = uint32(a.vocapp.Node.BlockStore().Base())
	}

	data, err := json.Marshal(&ChainInfo{
		ID:                a.vocapp.ChainID(),
		BlockTime:         blockTimesInMs,
		ElectionCount:     a.indexer.CountTotalProcesses(),
		OrganizationCount: a.indexer.CountTotalEntities(),
		Height:            a.vocapp.Height(),
		Syncing:           !a.vocapp.IsSynced(),
		TransactionCount:  transactionCount,
		ValidatorCount:    uint32(len(validators)),
		Timestamp:         a.vocapp.Timestamp(),
		VoteCount:         voteCount,
		GenesisTime:       a.vocapp.Genesis().GenesisTime,
		InitialHeight:     uint32(a.vocapp.Genesis().InitialHeight),
		BlockStoreBase:    blockStoreBase,
		CircuitVersion:    circuit.Version(),
		MaxCensusSize:     maxCensusSize,
		NetworkCapacity:   networkCapacity,
	})
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainCircuitInfoHandler
//
//	@Summary		Circuit info
//	@Description	Returns the circuit configuration according to the current circuit
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	circuit.Config
//	@Router			/chain/info/circuit [get]
func (a *API) chainCircuitInfoHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// Encode the current circuit configuration to JSON
	data, err := json.Marshal(circuit.Global().Config)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainInfoPriceFactors
//
//	@Summary				Price factors information
//	@Description.markdown	chainInfoPriceFactors
//	@Tags					Chain
//	@Accept					json
//	@Produce				json
//	@Success				200	{object}	electionprice.Calculator
//	@Router					/chain/info/electionPriceFactors [get]
func (a *API) chainInfoPriceFactors(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// Encode the values and factors to JSON
	data, err := json.Marshal(a.vocapp.State.ElectionPriceCalc)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainEstimateHeightHandler
//
//	@Summary		Estimate date to block
//	@Description	Returns the estimated block height for the timestamp provided
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			timestamp	path		string					true	"Timestamp on unix format"
//	@Success		200			{object}	object{height=number}	"Estimated block height"
//	@Router			/chain/dateToBlock/{timestamp} [get]
func (a *API) chainEstimateHeightHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	timestamp, err := strconv.ParseInt(ctx.URLParam("timestamp"), 10, 64)
	if err != nil {
		return err
	}
	height, err := a.vocinfo.EstimateBlockHeight(time.Unix(timestamp, 0))
	if err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Height uint64 `json:"height"`
	}{Height: height},
	)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainEstimateDateHandler
//
//	@Summary		Estimate block to date
//	@Description	Returns the estimated timestamp for the block height provided
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			height	path		number	true	"Block height"
//	@Success		200		{object}	object{date=string}
//	@Router			/chain/blockToDate/{height} [get]
func (a *API) chainEstimateDateHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	height, err := strconv.ParseUint(ctx.URLParam("height"), 10, 64)
	if err != nil {
		return err
	}
	timestamp := a.vocinfo.HeightTime(height)
	if timestamp.IsZero() {
		// if block was not found in store, the indexer might have it anyway
		timestamp, err = a.indexer.BlockTimestamp(int64(height))
		if err != nil {
			return err
		}
	}

	data, err := json.Marshal(struct {
		Date time.Time `json:"date"`
	}{Date: timestamp},
	)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainSendTxHandler
//
//	@Summary				Submit transaction
//	@Description.markdown	chainSendTxHandler
//	@Tags					Chain
//	@Accept					json
//	@Produce				json
//	@Param					transaction	body		object{payload=string}	true	"Base64 payload string containing transaction data and signature"
//	@Success				200			{object}	api.Transaction			"Return blockchain response. `response` could differ depending of transaction type."
//	@Router					/chain/transactions [post]
func (a *API) chainSendTxHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &Transaction{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return ErrCantParseDataAsJSON.WithErr(err)
	}
	res, err := a.sendTx(req.Payload)
	if err != nil {
		return err
	}
	data, err := json.Marshal(Transaction{
		Response: res.Data.Bytes(),
		Code:     &res.Code,
		Hash:     res.Hash.Bytes(),
	})
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxCostHandler
//
//	@Summary		Transaction costs
//	@Description	Returns the list of transactions and its cost
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	genesis.TransactionCosts
//	@Router			/chain/transactions/cost [get]
func (a *API) chainTxCostHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	txCosts := &Transaction{
		Costs: make(map[string]uint64),
	}
	var err error
	for k, v := range genesis.TxCostNameToTxTypeMap {
		txCosts.Costs[k], err = a.vocapp.State.TxBaseCost(v, true)
		if err != nil {
			return err
		}
	}
	var data []byte
	if data, err = json.Marshal(txCosts); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxListPaginated
//
//	@Summary		List Transactions
//	@Description	To get full transaction information use  [/chain/transaction/{blockHeight}/{txIndex}](transaction-by-block-index).\nWhere transactionIndex is the index of the transaction on the containing block.
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			page	path		int									true	"Page number"
//	@Success		200		{object}	api.chainTxListPaginated.response	"It return a list of transactions references"
//	@Router			/chain/transactions/page/{page} [get]
func (a *API) chainTxListPaginated(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	page := 0
	if ctx.URLParam("page") != "" {
		var err error
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return err
		}
	}
	offset := int32(page * MaxPageSize)
	refs, err := a.indexer.GetLastTransactions(MaxPageSize, offset)
	if err != nil {
		if errors.Is(err, indexer.ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return err
	}
	// wrap list in a struct to consistently return list in an object, return empty
	// object if the list does not contains any result
	type response struct {
		Txs []*indexertypes.Transaction `json:"transactions"`
	}
	data, err := json.Marshal(response{refs})
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxbyHashHandler
//
//	@Summary				Transaction by hash
//	@Description.markdown	chainTxbyHashHandler
//	@Accept					json
//	@Produce				json
//	@Tags					Chain
//	@Param					hash	path		string	true	"Transaction hash"
//	@Success				200		{object}	indexertypes.Transaction
//	@Success				204		"See [errors](vocdoni-api#errors) section"
//	@Router					/chain/transactions/reference/{hash} [get]
func (a *API) chainTxbyHashHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	hash, err := hex.DecodeString(util.TrimHex(ctx.URLParam("hash")))
	if err != nil {
		return err
	}
	ref, err := a.indexer.GetTxHashReference(hash)
	if err != nil {
		if errors.Is(err, indexer.ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return ErrTransactionNotFound.WithErr(err)
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxHandler
//
//	@Summary		Transaction by block height and index
//	@Description	Get transaction full information by block height and index. It returns JSON transaction protobuf encoded. Depending of transaction type will return different types of objects. Current transaction types can be found calling `/chain/transactions/cost`
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			height	path		int	true	"Block height"
//	@Param			index	path		int	true	"Transaction index on block"
//	@Success		200		{object}	GenericTransactionWithInfo
//	@Success		204		"See [errors](vocdoni-api#errors) section"
//	@Router			/chain/transactions/{height}/{index} [get]
func (a *API) chainTxHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	height, err := strconv.ParseInt(ctx.URLParam("height"), 10, 64)
	if err != nil {
		return err
	}
	index, err := strconv.ParseInt(ctx.URLParam("index"), 10, 64)
	if err != nil {
		return err
	}
	stx, err := a.vocapp.GetTx(uint32(height), int32(index))
	if err != nil {
		if errors.Is(err, vochain.ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return ErrVochainGetTxFailed.WithErr(err)
	}

	ref, err := a.indexer.GetTxReferenceByBlockHeightAndBlockIndex(height, index)
	if err != nil {
		if errors.Is(err, indexer.ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return ErrVochainGetTxFailed.WithErr(err)
	}
	tx := &GenericTransactionWithInfo{
		TxContent: []byte(protoFormat(stx.Tx)),
		Signature: stx.Signature,
		TxInfo:    *ref,
	}
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxByIndexHandler
//
//	@Summary		Transaction by index
//	@Description	Get transaction by its index. This is not transaction reference (hash), and neither the block height and block  index. The transaction index is an incremental counter for each transaction.  You could use the transaction `block` and `index` to retrieve full info using [transaction by block and index](transaction-by-block-index).
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			index	path		int	true	"Index of the transaction"
//	@Success		200		{object}	indexertypes.Transaction
//	@Success		204		"See [errors](vocdoni-api#errors) section"
//	@Router			/chain/transactions/reference/index/{index} [get]
func (a *API) chainTxByIndexHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	index, err := strconv.ParseUint(ctx.URLParam("index"), 10, 64)
	if err != nil {
		return err
	}
	ref, err := a.indexer.GetTransaction(index)
	if err != nil {
		if errors.Is(err, indexer.ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return ErrVochainGetTxFailed.WithErr(err)
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxByHeightHandler
//
//	@Summary		Transactions in a block
//	@Description	Given a block returns the list of transactions for that block
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			height	path		number	true	"Block height"
//	@Param			page	path		number	true	"Page to paginate"
//	@Success		200		{object}	[]TransactionMetadata
//	@Router			/chain/blocks/{height}/transactions/page/{page} [get]
func (a *API) chainTxByHeightHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	height, err := strconv.ParseUint(ctx.URLParam("height"), 10, 64)
	if err != nil {
		return err
	}
	block := a.vocapp.GetBlockByHeight(int64(height))
	if block == nil {
		return ErrBlockNotFound
	}
	blockTxs := &BlockTransactionsInfo{
		BlockNumber:       height,
		TransactionsCount: uint32(len(block.Txs)),
		Transactions:      make([]TransactionMetadata, 0),
	}

	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber.WithErr(err)
		}
	}
	page = page * MaxPageSize
	count := 0
	for i := page; i < len(block.Txs); i++ {
		if count >= MaxPageSize {
			break
		}
		signedTx := new(models.SignedTx)
		tx := new(models.Tx)
		var err error
		if err := proto.Unmarshal(block.Txs[i], signedTx); err != nil {
			return ErrUnmarshalingServerProto.WithErr(err)
		}
		if err := proto.Unmarshal(signedTx.Tx, tx); err != nil {
			return ErrUnmarshalingServerProto.WithErr(err)
		}
		txType := string(
			tx.ProtoReflect().WhichOneof(
				tx.ProtoReflect().Descriptor().Oneofs().Get(0)).Name())

		// TODO: can we avoid indexer Get calls in a loop?
		txRef, err := a.indexer.GetTxHashReference(block.Txs[i].Hash())
		if err != nil {
			return ErrTransactionNotFound
		}
		blockTxs.Transactions = append(blockTxs.Transactions, TransactionMetadata{
			Type:   txType,
			Index:  int32(i),
			Number: uint32(txRef.Index),
			Hash:   block.Txs[i].Hash(),
		})
		count++
	}
	data, err := json.Marshal(blockTxs)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainValidatorsHandler
//
//	@Summary		List validators
//	@Description	Returns the list of validators
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	ValidatorList
//	@Router			/chain/validators [get]
func (a *API) chainValidatorsHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	stateValidators, err := a.vocapp.State.Validators(true)
	if err != nil {
		return err
	}
	validators := ValidatorList{}
	for _, v := range stateValidators {
		validators.Validators = append(validators.Validators, Validator{
			AccountAddress:   v.GetAddress(),
			ValidatorAddress: v.GetValidatorAddress(),
			Power:            v.GetPower(),
			Name:             v.GetName(),
			PubKey:           v.GetPubKey(),
			JoinHeight:       v.GetHeight(),
			Votes:            v.GetVotes(),
			Proposals:        v.GetProposals(),
			Score:            v.GetScore(),
		})
	}
	data, err := json.Marshal(&validators)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainBlockHandler
//
//	@Summary		Get block (by height)
//	@Description	Returns the full block information at the given height
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			height	path		int	true	"Block height"
//	@Success		200		{object}	api.Block
//	@Router			/chain/blocks/{height} [get]
func (a *API) chainBlockHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	height, err := strconv.ParseUint(ctx.URLParam("height"), 10, 64)
	if err != nil {
		return err
	}
	tmblock := a.vocapp.GetBlockByHeight(int64(height))
	if tmblock == nil {
		return ErrBlockNotFound
	}
	block := &Block{
		Block: comettypes.Block{
			Header:     tmblock.Header,
			Data:       tmblock.Data,
			Evidence:   tmblock.Evidence,
			LastCommit: tmblock.LastCommit,
		},
		Hash: types.HexBytes(tmblock.Hash()),
	}
	data, err := json.Marshal(block)
	if err != nil {
		return err
	}
	return ctx.Send(convertKeysToCamel(data), apirest.HTTPstatusOK)
}

// chainBlockByHashHandler
//
//	@Summary		Get block (by hash)
//	@Description	Returns the block from the given hash
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			hash	path		string	true	"Block hash"
//	@Success		200		{object}	api.Block
//	@Router			/chain/blocks/hash/{hash} [get]
func (a *API) chainBlockByHashHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	hash, err := hex.DecodeString(util.TrimHex(ctx.URLParam("hash")))
	if err != nil {
		return err
	}
	tmblock := a.vocapp.GetBlockByHash(hash)
	if tmblock == nil {
		return ErrBlockNotFound
	}
	block := &Block{
		Block: comettypes.Block{
			Header:     tmblock.Header,
			Data:       tmblock.Data,
			Evidence:   tmblock.Evidence,
			LastCommit: tmblock.LastCommit,
		},
		Hash: types.HexBytes(tmblock.Hash()),
	}
	data, err := json.Marshal(block)
	if err != nil {
		return err
	}
	return ctx.Send(convertKeysToCamel(data), apirest.HTTPstatusOK)
}

// chainOrganizationsFilterPaginatedHandler
//
//	@Summary		List organizations (filtered)
//	@Description	Returns a list of organizations filtered by its partial id, paginated by the given page
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			organizationId	body		object{organizationId=string}	true	"Partial organizationId to filter by"
//	@Param			page			path		int								true	"Current page"
//	@Success		200				{object}	object{organizations=[]api.OrganizationList}
//	@Router			/chain/organizations/filter/page/{page} [post]
func (a *API) chainOrganizationsFilterPaginatedHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// get organizationId from the request body
	requestData := struct {
		OrganizationId string `json:"organizationId"`
	}{}
	if err := json.Unmarshal(msg.Data, &requestData); err != nil {
		return ErrCantParseDataAsJSON.WithErr(err)
	}
	// get page
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber.WithErr(err)
		}
	}
	page = page * MaxPageSize

	organizations := []*OrganizationList{}
	// get matching organization ids from the indexer
	matchingOrganizationIds := a.indexer.EntityList(MaxPageSize, page, util.TrimHex(requestData.OrganizationId))
	if len(matchingOrganizationIds) == 0 {
		return ErrOrgNotFound
	}

	for _, org := range matchingOrganizationIds {
		organizations = append(organizations, &OrganizationList{
			OrganizationID: org.EntityID,
			ElectionCount:  uint64(org.ProcessCount),
		})
	}

	data, err := json.Marshal(struct {
		Organizations []*OrganizationList `json:"organizations"`
	}{organizations})
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTransactionCountHandler
//
//	@Summary		Transactions count
//	@Description	Returns the number of transactions
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	uint64
//	@Success		200	{object}	object{count=number}
//	@Router			/chain/transactions/count [get]
func (a *API) chainTxCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count, err := a.indexer.CountTotalTransactions()
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

// chainListFeesHandler
//
//	@Summary		List all token fees
//	@Description	Returns the token fees list ordered by date. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			page	path		string	true	"Paginator page"
//	@Success		200		{object}	object{fees=[]indexertypes.TokenFeeMeta}
//	@Router			/chain/fees/page/{page} [get]
func (a *API) chainListFeesHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize

	fees, err := a.indexer.GetTokenFees(int32(page), MaxPageSize)
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

// chainListFeesByReferenceHandler
//
//	@Summary		List all token fees by reference
//	@Description	Returns the token fees list filtered by reference and ordered by date. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			reference	path		string	true	"Reference filter"
//	@Param			page		path		string	true	"Paginator page"
//	@Success		200			{object}	object{fees=[]indexertypes.TokenFeeMeta}
//	@Router			/chain/fees/reference/{reference}/page/{page} [get]
func (a *API) chainListFeesByReferenceHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize

	reference := ctx.URLParam("reference")
	if reference == "" {
		return ErrMissingParameter
	}

	fees, err := a.indexer.GetTokenFeesByReference(reference, int32(page), MaxPageSize)
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

// chainListFeesByTypeHandler
//
//	@Summary		List all token fees by type
//	@Description	Returns the token fees list filtered by type and ordered by date. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			type	path		string	true	"Type filter"
//	@Param			page	path		string	true	"Paginator page"
//	@Success		200		{object}	object{fees=[]indexertypes.TokenFeeMeta}
//	@Router			/chain/fees/type/{type}/page/{page} [get]
func (a *API) chainListFeesByTypeHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return ErrCantParsePageNumber
		}
	}
	page = page * MaxPageSize

	typeFilter := ctx.URLParam("type")
	if typeFilter == "" {
		return ErrMissingParameter
	}

	fees, err := a.indexer.GetTokenFeesByType(typeFilter, int32(page), MaxPageSize)
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

// chainIndexerExportHandler
//
//	@Summary		Exports the indexer database
//	@Description	Exports the indexer SQL database in raw format
//	@Tags			Indexer
//	@Produce		json
//	@Success		200	{string}	raw-data
//	@Router			/chain/export/indexer [get]
func (a *API) chainIndexerExportHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	exportCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	data, err := a.indexer.ExportBackupAsBytes(exportCtx)
	if err != nil {
		return fmt.Errorf("error reading indexer backup: %w", err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}
