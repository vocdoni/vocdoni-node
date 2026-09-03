package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	comettypes "github.com/cometbft/cometbft/types"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/processid"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// maxTransactionBatchSize bounds how many transactions POST /chain/transactions/batch accepts.
const maxTransactionBatchSize = 100

const (
	ChainHandler = "chain"
)

func (a *API) enableChainHandlers() error {
	if err := a.Endpoint.RegisterMethod(
		"/chain/organizations",
		"GET",
		apirest.MethodAccessTypePublic,
		a.organizationListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/organizations/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.organizationListByPageHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/stats",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainStatsHandler,
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
		a.chainTxRefByHashHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/blocks/{height}/transactions/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxListByHeightAndPageHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/{height}/{index}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxByHeightAndIndexHandler,
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
		"/chain/transactions/batch",
		"POST",
		apirest.MethodAccessTypePublic,
		a.chainSendTxBatchHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/{hash}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxByHashHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transactions/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxListByPageHandler,
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
		a.chainBlockByHeightHandler,
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
		"/chain/blocks",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainBlockListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/organizations/filter/page/{page}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.organizationListByFilterAndPageHandler,
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
		"/chain/fees",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainFeesListHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/fees/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainFeesListByPageHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/fees/reference/{reference}/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainFeesListByReferenceAndPageHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/fees/type/{type}/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainFeesListByTypeAndPageHandler,
	); err != nil {
		return err
	}
	if err := a.Endpoint.RegisterMethod(
		"/chain/transfers",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTransfersListHandler,
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
//	@Param					page			query		number	false	"Page"
//	@Param					limit			query		number	false	"Items per page"
//	@Param					organizationId	query		string	false	"Filter by partial organizationId"
//	@Param					name			query		string	false	"Filter by organization name, case-insensitive substring match (ASCII case folding only)"
//	@Param					sortBy			query		string	false	"Sort by createdAt (when the organization first appeared, default), electionCount or name"	Enums(createdAt, electionCount, name)
//	@Param					order			query		string	false	"Sort direction. Defaults to desc for createdAt and electionCount, asc for name"			Enums(asc, desc)
//	@Success				200				{object}	OrganizationsList
//	@Router					/chain/organizations [get]
func (a *API) organizationListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseOrganizationParams(
		ctx.QueryParam(ParamPage),
		ctx.QueryParam(ParamLimit),
		ctx.QueryParam(ParamOrganizationId),
		ctx.QueryParam(ParamName),
		ctx.QueryParam(ParamSortBy),
		ctx.QueryParam(ParamOrder),
	)
	if err != nil {
		return err
	}

	list, err := a.organizationList(params)
	if err != nil {
		return err
	}

	return marshalAndSend(ctx, list)
}

// organizationListByPageHandler
//
//	@Summary		List organizations
//	@Description	List all organizations
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/organizations?page=xxx)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			page	path		number	true	"Page"
//	@Success		200		{object}	OrganizationsList
//	@Router			/chain/organizations/page/{page} [get]
func (a *API) organizationListByPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseOrganizationParams(
		ctx.URLParam(ParamPage),
		"",
		"",
		"",
		"",
		"",
	)
	if err != nil {
		return err
	}

	list, err := a.organizationList(params)
	if err != nil {
		// keep legacy behaviour of sending an empty list rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return marshalAndSend(ctx, emptyOrganizationsList())
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// organizationListByFilterAndPageHandler
//
//	@Summary		List organizations (filtered)
//	@Description	Returns a list of organizations filtered by its partial id, paginated by the given page
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/organizations?page=xxx&organizationId=xxx)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			body	body		OrganizationParams	true	"Partial organizationId to filter by"
//	@Param			page	path		number				true	"Page"
//	@Success		200		{object}	OrganizationsList
//	@Router			/chain/organizations/filter/page/{page} [post]
func (a *API) organizationListByFilterAndPageHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// get organizationId from the request params
	params := &OrganizationParams{}

	// but support legacy URLParam
	urlParams, err := parsePaginationParams(ctx.URLParam(ParamPage), "")
	if err != nil {
		return err
	}
	params.PaginationParams = urlParams

	if err := json.Unmarshal(msg.Data, &params); err != nil {
		return ErrCantParseDataAsJSON.WithErr(err)
	}
	if params == nil { // happens when client POSTs a literal `null` JSON
		return ErrMissingParameter
	}

	list, err := a.organizationList(params)
	if err != nil {
		// keep legacy behaviour of sending an empty list rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return marshalAndSend(ctx, emptyOrganizationsList())
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// organizationList produces a filtered, sorted, paginated OrganizationsList.
//
// The ordering is validated here rather than in parseOrganizationParams so that
// the deprecated POST filter endpoint, which unmarshals its OrganizationParams
// straight from the request body, is covered as well.
//
// Errors returned are always of type APIerror.
func (a *API) organizationList(params *OrganizationParams) (*OrganizationsList, error) {
	sortBy, order, err := parseOrganizationSort(params.SortBy, params.Order)
	if err != nil {
		return nil, err
	}

	orgs, total, err := a.indexer.EntityList(
		params.Limit,
		params.Page*params.Limit,
		params.OrganizationID,
		params.Name,
		sortBy,
		order,
	)
	if err != nil {
		return nil, ErrIndexerQueryFailed.WithErr(err)
	}

	pagination, err := calculatePagination(params.Page, params.Limit, total)
	if err != nil {
		return nil, err
	}

	list := &OrganizationsList{
		Organizations: []*OrganizationSummary{},
		Pagination:    pagination,
	}
	for _, org := range orgs {
		list.Organizations = append(list.Organizations, &OrganizationSummary{
			OrganizationID: org.EntityID,
			ElectionCount:  uint64(org.ProcessCount),
			Name:           org.Name,
			Avatar:         org.Avatar,
		})
	}
	return list, nil
}

// organizationCountHandler
//
//	@Summary		Count organizations
//	@Description	Return the number of organizations
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/organizations which reports totalItems)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	CountResult	"Number of registered organizations"
//	@Router			/chain/organizations/count [get]
func (a *API) organizationCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count := a.indexer.CountTotalEntities()
	return marshalAndSend(ctx, &CountResult{Count: count})
}

// chainStatsHandler
//
//	@Summary		Chain statistics
//	@Description	Returns the chain-wide counters in a single response: transactions
//	@Description	grouped by type, elections grouped by status, and the total number of
//	@Description	accounts, elections and votes. Buckets with a count of zero are omitted.
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	ChainStats
//	@Router			/chain/stats [get]
func (a *API) chainStatsHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	txCountByType, err := a.indexer.CountTransactionsByType()
	if err != nil {
		return ErrIndexerQueryFailed.WithErr(err)
	}
	electionCountByStatus, err := a.indexer.CountProcessesByStatus()
	if err != nil {
		return ErrIndexerQueryFailed.WithErr(err)
	}
	accountCount, err := a.indexer.CountTotalAccounts()
	if err != nil {
		return ErrIndexerQueryFailed.WithErr(err)
	}
	voteCount, err := a.indexer.CountTotalVotes()
	if err != nil {
		return ErrIndexerQueryFailed.WithErr(err)
	}

	return marshalAndSend(ctx, &ChainStats{
		TxCountByType:         txCountByType,
		ElectionCountByStatus: electionCountByStatus,
		AccountCount:          accountCount,
		ElectionCount:         a.indexer.CountTotalProcesses(),
		VoteCount:             voteCount,
	})
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
	height, err := strconv.ParseUint(ctx.URLParam(ParamHeight), 10, 64)
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

// chainSendTxBatchHandler
//
//	@Summary				Submit a batch of transactions
//	@Description.markdown	chainSendTxBatchHandler
//	@Tags					Chain
//	@Accept					json
//	@Produce				json
//	@Param					transactions	body		TransactionBatch		true	"List of base64 transaction payloads, submitted in order"
//	@Success				200				{object}	TransactionBatchResult	"Every input transaction grouped as submitted / failed / pending"
//
// The transactions are submitted in order and fail-fast: the first one that fails
// to be submitted goes to `failed` and submission stops, leaving the rest in
// `pending`. "submitted" means the mempool accepted the broadcast, NOT that the
// transaction is block-confirmed — the caller must confirm each submitted item
// on-chain and resubmit any that did not land, together with failed and pending.
// For NewProcess transactions each item carries its predicted `processId`.
//
//	@Router					/chain/transactions/batch [post]
func (a *API) chainSendTxBatchHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &TransactionBatch{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return ErrCantParseDataAsJSON.WithErr(err)
	}
	if len(req.Transactions) == 0 {
		return ErrTransactionBatchEmpty
	}
	if len(req.Transactions) > maxTransactionBatchSize {
		return ErrTransactionBatchTooLarge.Withf("%d, max is %d", len(req.Transactions), maxTransactionBatchSize)
	}

	// Validate election metadata up front (same checks as POST /elections): a
	// client-side metadata mistake rejects the whole batch before anything is
	// submitted, so no transaction lands on-chain because of it.
	for i := range req.Transactions {
		if err := a.checkBatchItemMetadata(req.Transactions[i]); err != nil {
			return err
		}
	}

	// per-creator (EntityId) running process-index delta, advanced for every
	// NewProcess item regardless of send outcome, so each item gets the exact
	// processId the chain will assign in ordered commit.
	deltas := map[string]int32{}
	result := classifyTransactionBatch(req.Transactions,
		func(payload []byte) []byte { return a.batchProcessID(payload, deltas) },
		func(payload []byte) (hash []byte, code uint32, err error) {
			res, err := a.sendTx(payload)
			if err != nil {
				return nil, 0, err
			}
			return res.Hash.Bytes(), res.Code, nil
		},
	)

	// Pin the metadata of the transactions that were actually submitted, best-effort,
	// exactly like POST /elections. This relies on the fail-fast invariant of
	// classifyTransactionBatch: submitted items are always the leading prefix of the
	// input in order, so result.Submitted[i] corresponds to req.Transactions[i]. If
	// that strategy ever changes, this index correspondence must be revisited.
	for i := range result.Submitted {
		md := req.Transactions[i].Metadata
		if a.storage == nil || len(md) == 0 {
			continue
		}
		sctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		cid, err := a.storage.Publish(sctx, md)
		cancel()
		if err != nil {
			log.Errorf("could not publish batch metadata to storage: %v", err)
			continue
		}
		result.Submitted[i].MetadataURL = a.storage.URIprefix() + cid
		if strings.TrimPrefix(cid, "ipfs://") != strings.TrimPrefix(ipfs.CalculateCIDv1json(md), "ipfs://") {
			log.Errorf("%s (%s != %s)", ErrVochainReturnedWrongMetadataCID, cid, ipfs.CalculateCIDv1json(md))
		}
	}

	data, err := json.Marshal(result)
	if err != nil {
		return ErrMarshalingServerJSONFailed.WithErr(err)
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// classifyTransactionBatch submits the transactions in order and groups every
// input item into submitted / failed / pending. It is fail-fast: the first send
// error goes to `failed` and no further items are sent (they share the sender's
// now-broken nonce sequence and cannot commit), so the rest land in `pending`.
// `processID` supplies each item's predicted processId (nil if not applicable);
// it is called for every item, including the unsent ones. Injecting `processID`
// and `send` keeps the grouping logic testable without a live chain.
func classifyTransactionBatch(
	txs []TransactionPayload,
	processID func(payload []byte) []byte,
	send func(payload []byte) (hash []byte, code uint32, err error),
) *TransactionBatchResult {
	result := &TransactionBatchResult{
		Submitted: []TransactionBatchItem{},
		Failed:    []TransactionBatchItem{},
		Pending:   []TransactionBatchItem{},
	}
	stopped := false
	for i := range txs {
		item := TransactionBatchItem{ProcessID: processID(txs[i].Payload)}
		if stopped {
			result.Pending = append(result.Pending, item)
			continue
		}
		hash, code, err := send(txs[i].Payload)
		if err != nil {
			item.Error = err.Error()
			result.Failed = append(result.Failed, item)
			stopped = true
			continue
		}
		item.Hash = hash
		item.Code = &code
		result.Submitted = append(result.Submitted, item)
	}
	return result
}

// batchProcessID returns the predicted processId for a NewProcess payload and
// advances the per-creator positional delta. Returns nil for non-NewProcess or
// undecodable payloads (the tx is still submitted; only the predicted id is absent).
func (a *API) batchProcessID(payload []byte, deltas map[string]int32) []byte {
	stx := &models.SignedTx{}
	if err := proto.Unmarshal(payload, stx); err != nil {
		return nil
	}
	// Decode the inner Tx and build the signed body (also used to recover the signer).
	signedBody, tx, err := ethereum.BuildVocdoniTransaction(stx.GetTx(), a.vocapp.ChainID())
	if err != nil {
		return nil
	}
	proc := tx.GetNewProcess().GetProcess()
	if proc == nil {
		return nil
	}
	// Resolve the organization exactly as NewProcessTxCheck does: when EntityId is
	// unset, it defaults to the tx signer's address (vochain/transaction/election_tx.go).
	// Both the delta key and BuildProcessID must use the resolved entity, otherwise
	// the predicted id would be dropped and the deltas would collide on the empty key.
	entity := proc.GetEntityId()
	if len(entity) == 0 {
		addr, err := ethereum.AddrFromSignature(signedBody, stx.GetSignature())
		if err != nil {
			return nil
		}
		entity = addr.Bytes()
		proc.EntityId = entity
	}
	key := string(entity)
	delta := deltas[key]
	deltas[key] = delta + 1
	pid, err := processid.BuildProcessID(proc, a.vocapp.State, delta)
	if err != nil {
		return nil
	}
	return pid.Marshal()
}

// checkBatchItemMetadata validates a batch item's optional raw election metadata,
// mirroring POST /elections: only when raw metadata is provided, the NewProcess tx
// must carry a matching metadata URI. Returns nil when there is nothing to check.
func (a *API) checkBatchItemMetadata(item TransactionPayload) error {
	if len(item.Metadata) == 0 {
		return nil
	}
	// Extract the metadata URI (and encrypted flag) from the NewProcess payload.
	metadataURI, encryptedMetadata := "", false
	stx := &models.SignedTx{}
	if err := proto.Unmarshal(item.Payload, stx); err == nil {
		tx := &models.Tx{}
		if err := proto.Unmarshal(stx.GetTx(), tx); err == nil {
			if p := tx.GetNewProcess().GetProcess(); p != nil {
				metadataURI = p.GetMetadata()
				encryptedMetadata = p.GetMode() != nil && p.GetMode().EncryptedMetaData
			}
		}
	}
	if metadataURI == "" {
		return ErrMetadataProvidedButNoURI
	}
	if !encryptedMetadata {
		if err := json.Unmarshal(item.Metadata, &ElectionMetadata{}); err != nil {
			return ErrCantParseMetadataAsJSON.WithErr(err)
		}
	}
	if !ipfs.CIDequals(ipfs.CalculateCIDv1json(item.Metadata), metadataURI) {
		return ErrMetadataURINotMatchContent
	}
	return nil
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
			if errors.Is(err, state.ErrTxCostNotFound) {
				txCosts.Costs[k] = 0
				continue
			}
			return err
		}
	}
	var data []byte
	if data, err = json.Marshal(txCosts); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxRefByHashHandler
//
//	@Summary				Transaction by hash
//	@Description.markdown	chainTxRefByHashHandler
//	@Accept					json
//	@Produce				json
//	@Tags					Chain
//	@Param					hash	path		string	true	"Transaction hash"
//	@Success				200		{object}	indexertypes.Transaction
//	@Success				204		"See [errors](vocdoni-api#errors) section"
//	@Router					/chain/transactions/reference/{hash} [get]
func (a *API) chainTxRefByHashHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	hash, err := hex.DecodeString(util.TrimHex(ctx.URLParam(ParamHash)))
	if err != nil {
		return err
	}
	ref, err := a.indexer.GetTxMetadataByHash(hash)
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

// chainTxByHeightAndIndexHandler
//
//	@Summary		Transaction by block height and index
//	@Description	Get transaction full information by block height and index. It returns JSON transaction protobuf encoded. Depending of transaction type will return different types of objects. Current transaction types can be found calling `/chain/transactions/cost`
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/transactions/{hash})
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			height	path		int	true	"Block height"
//	@Param			index	path		int	true	"Transaction index on block"
//	@Success		200		{object}	GenericTransactionWithInfo
//	@Success		204		"See [errors](vocdoni-api#errors) section"
//	@Router			/chain/transactions/{height}/{index} [get]
func (a *API) chainTxByHeightAndIndexHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	height, err := strconv.ParseInt(ctx.URLParam(ParamHeight), 10, 64)
	if err != nil {
		return err
	}
	index, err := strconv.ParseInt(ctx.URLParam("index"), 10, 64)
	if err != nil {
		return err
	}
	itx, err := a.indexer.GetTransactionByHeightAndIndex(height, index)
	if err != nil {
		if errors.Is(err, indexer.ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return ErrVochainGetTxFailed.WithErr(err)
	}
	tx := &GenericTransactionWithInfo{
		TxContent: protoTxAsJSON(itx.RawTx),
		Signature: itx.Signature,
		TxInfo:    itx,
	}
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxByHashHandler
//
//	@Summary		Transaction by hash
//	@Description	Get transaction full information by hash. It returns JSON transaction protobuf encoded. Depending of transaction type will return different types of objects. Current transaction types can be found calling `/chain/transactions/cost`
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			hash	path		string	true	"Transaction hash"
//	@Success		200		{object}	GenericTransactionWithInfo
//	@Success		204		"See [errors](vocdoni-api#errors) section"
//	@Router			/chain/transactions/{hash} [get]
func (a *API) chainTxByHashHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	hash, err := hex.DecodeString(util.TrimHex(ctx.URLParam(ParamHash)))
	if err != nil {
		return ErrCantParseHexString.WithErr(err)
	}
	itx, err := a.indexer.GetTransactionByHash(hash)
	if err != nil {
		if errors.Is(err, indexer.ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return ErrVochainGetTxFailed.WithErr(err)
	}
	tx := &GenericTransactionWithInfo{
		TxContent: protoTxAsJSON(itx.RawTx),
		Signature: itx.Signature,
		TxInfo:    itx,
	}
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxListHandler
//
//	@Summary		List transactions
//	@Description	To get full transaction information use  [/chain/transaction/{hash}](transaction-by-hash).
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			page	query		number				false	"Page"
//	@Param			limit	query		number				false	"Items per page"
//	@Param			hash	query		string				false	"Tx hash"
//	@Param			height	query		number				false	"Block height"
//	@Param			type	query		string				false	"Tx type"
//	@Param			subtype	query		string				false	"Tx subtype"
//	@Param			signer	query		string				false	"Tx signer"
//	@Success		200		{object}	TransactionsList	"List of transactions (metadata only)"
//	@Router			/chain/transactions [get]
func (a *API) chainTxListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseTransactionParams(
		ctx.QueryParam(ParamPage),
		ctx.QueryParam(ParamLimit),
		ctx.QueryParam(ParamHash),
		ctx.QueryParam(ParamHeight),
		ctx.QueryParam(ParamType),
		ctx.QueryParam(ParamSubtype),
		ctx.QueryParam(ParamSigner),
	)
	if err != nil {
		return err
	}

	list, err := a.transactionList(params)
	if err != nil {
		return err
	}

	return marshalAndSend(ctx, list)
}

// chainTxListByPageHandler
//
//	@Summary		List transactions (legacy)
//	@Description	To get full transaction information use  [/chain/transaction/{blockHeight}/{txIndex}](transaction-by-block-index).\nWhere transactionIndex is the index of the transaction on the containing block.
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/transactions?page=xxx)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			page	path		number				true	"Page"
//	@Success		200		{object}	TransactionsList	"List of transactions (metadata only)"
//	@Router			/chain/transactions/page/{page} [get]
func (a *API) chainTxListByPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseTransactionParams(
		ctx.URLParam(ParamPage),
		"",
		"",
		"",
		"",
		"",
		"",
	)
	if err != nil {
		return err
	}

	list, err := a.transactionList(params)
	if err != nil {
		// keep the odd legacy behaviour of sending a 204 rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return ErrTransactionNotFound
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// chainTxListByHeightAndPageHandler
//
//	@Summary		Transactions in a block
//	@Description	Given a block returns the list of transactions for that block
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/transactions?page=xxx&height=xxx)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			height	path		number	true	"Block height"
//	@Param			page	path		number	true	"Page"
//	@Success		200		{object}	TransactionsList
//	@Router			/chain/blocks/{height}/transactions/page/{page} [get]
func (a *API) chainTxListByHeightAndPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseTransactionParams(
		ctx.URLParam(ParamPage),
		"",
		"",
		// parseTransactionParams takes hash before height; the height used to be
		// passed in the hash slot, which filtered by a hash substring instead and
		// left the height filter unset. The result was not an empty list but a
		// wrong one: every transaction whose hex hash happened to contain the
		// height as a substring, from any block.
		ctx.URLParam(ParamHeight),
		"",
		"",
		"",
	)
	if err != nil {
		return err
	}

	list, err := a.transactionList(params)
	if err != nil {
		// keep legacy behaviour of sending an empty list rather than a 404
		if errors.Is(err, ErrPageNotFound) {
			return marshalAndSend(ctx, emptyTransactionsList())
		}
		return err
	}

	return marshalAndSend(ctx, list)
}

// transactionList produces a filtered, paginated TransactionList.
//
// Errors returned are always of type APIerror.
func (a *API) transactionList(params *TransactionParams) (*TransactionsList, error) {
	txs, total, err := a.indexer.SearchTransactions(
		params.Limit,
		params.Page*params.Limit,
		params.Height,
		params.Hash,
		params.Type,
		params.Subtype,
		params.Signer,
	)
	if err != nil {
		return nil, ErrIndexerQueryFailed.WithErr(err)
	}

	pagination, err := calculatePagination(params.Page, params.Limit, total)
	if err != nil {
		return nil, err
	}

	list := &TransactionsList{
		Transactions: txs,
		Pagination:   pagination,
	}
	return list, nil
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

// chainBlockByHeightHandler
//
//	@Summary		Get block (by height)
//	@Description	Returns the full block information at the given height
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			height	path		int	true	"Block height"
//	@Success		200		{object}	api.Block
//	@Router			/chain/blocks/{height} [get]
func (a *API) chainBlockByHeightHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	height, err := strconv.ParseUint(ctx.URLParam(ParamHeight), 10, 64)
	if err != nil {
		return err
	}
	idxblock, err := a.indexer.BlockByHeight(int64(height))
	if err != nil {
		if errors.Is(err, indexer.ErrBlockNotFound) {
			return ErrBlockNotFound
		}
		return ErrBlockNotFound.WithErr(err)
	}
	txcount, err := a.indexer.CountTransactionsByHeight(int64(height))
	if err != nil {
		return ErrIndexerQueryFailed.WithErr(err)
	}
	block := &Block{
		Header: comettypes.Header{
			ChainID:         idxblock.ChainID,
			Height:          idxblock.Height,
			Time:            idxblock.Time,
			ProposerAddress: []byte(idxblock.ProposerAddress),
			LastBlockID: comettypes.BlockID{
				Hash: []byte(idxblock.LastBlockHash),
			},
		},
		Hash:    idxblock.Hash,
		TxCount: txcount,
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
	idxblock, err := a.indexer.BlockByHash(hash)
	if err != nil {
		if errors.Is(err, indexer.ErrBlockNotFound) {
			return ErrBlockNotFound
		}
		return ErrBlockNotFound.WithErr(err)
	}
	txcount, err := a.indexer.CountTransactionsByHeight(idxblock.Height)
	if err != nil {
		return ErrIndexerQueryFailed.WithErr(err)
	}
	block := &Block{
		Header: comettypes.Header{
			ChainID:         idxblock.ChainID,
			Height:          idxblock.Height,
			Time:            idxblock.Time,
			ProposerAddress: []byte(idxblock.ProposerAddress),
			LastBlockID: comettypes.BlockID{
				Hash: []byte(idxblock.LastBlockHash),
			},
		},
		Hash:    idxblock.Hash,
		TxCount: txcount,
	}
	data, err := json.Marshal(block)
	if err != nil {
		return err
	}
	return ctx.Send(convertKeysToCamel(data), apirest.HTTPstatusOK)
}

// chainBlockListHandler
//
//	@Summary		List all blocks
//	@Description	Returns the list of blocks, ordered by descending height.
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			page			query		number	false	"Page"
//	@Param			limit			query		number	false	"Items per page"
//	@Param			chainId			query		string	false	"Filter by exact chainId"
//	@Param			hash			query		string	false	"Filter by partial hash"
//	@Param			proposerAddress	query		string	false	"Filter by exact proposerAddress"
//	@Success		200				{object}	BlockList
//	@Router			/chain/blocks [get]
func (a *API) chainBlockListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseBlockParams(
		ctx.QueryParam(ParamPage),
		ctx.QueryParam(ParamLimit),
		ctx.QueryParam(ParamChainId),
		ctx.QueryParam(ParamHash),
		ctx.QueryParam(ParamProposerAddress),
	)
	if err != nil {
		return err
	}

	return a.sendBlockList(ctx, params)
}

// sendBlockList produces a filtered, paginated BlockList,
// and sends it marshalled over ctx.Send
//
// Errors returned are always of type APIerror.
func (a *API) sendBlockList(ctx *httprouter.HTTPContext, params *BlockParams) error {
	blocks, total, err := a.indexer.BlockList(
		params.Limit,
		params.Page*params.Limit,
		params.ChainID,
		params.Hash,
		params.ProposerAddress,
	)
	if err != nil {
		return ErrIndexerQueryFailed.WithErr(err)
	}

	pagination, err := calculatePagination(params.Page, params.Limit, total)
	if err != nil {
		return err
	}

	list := &BlockList{
		Blocks:     blocks,
		Pagination: pagination,
	}
	return marshalAndSend(ctx, list)
}

// chainTransactionCountHandler
//
//	@Summary		Transactions count
//	@Description	Returns the number of transactions
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/transactions which reports totalItems)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	CountResult
//	@Router			/chain/transactions/count [get]
func (a *API) chainTxCountHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count, err := a.indexer.CountTotalTransactions()
	if err != nil {
		return err
	}
	return marshalAndSend(ctx, &CountResult{Count: count})
}

// chainFeesListHandler
//
//	@Summary		List all token fees
//	@Description	Returns the token fees list ordered by date. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			page		query		number	false	"Page"
//	@Param			limit		query		number	false	"Items per page"
//	@Param			reference	query		string	false	"Reference filter"
//	@Param			type		query		string	false	"Type filter"
//	@Param			accountId	query		string	false	"Specific accountId"
//	@Success		200			{object}	FeesList
//	@Router			/chain/fees [get]
func (a *API) chainFeesListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseFeesParams(
		ctx.QueryParam(ParamPage),
		ctx.QueryParam(ParamLimit),
		ctx.QueryParam(ParamReference),
		ctx.QueryParam(ParamType),
		ctx.QueryParam(ParamAccountId),
	)
	if err != nil {
		return err
	}

	list, err := a.feesList(params)
	if err != nil {
		return err
	}

	return marshalAndSend(ctx, list)
}

// chainFeesListByPageHandler
//
//	@Summary		List all token fees
//	@Description	Returns the token fees list ordered by date. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/fees?page=xxx)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			page	path		number	true	"Page"
//	@Success		200		{object}	FeesList
//	@Router			/chain/fees/page/{page} [get]
func (a *API) chainFeesListByPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseFeesParams(
		ctx.URLParam(ParamPage),
		"",
		"",
		"",
		"",
	)
	if err != nil {
		return err
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

// chainFeesListByReferenceAndPageHandler
//
//	@Summary		List all token fees by reference
//	@Description	Returns the token fees list filtered by reference and ordered by date. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/fees?page=xxx)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			reference	path		string	true	"Reference filter"
//	@Param			page		path		number	true	"Page"
//	@Success		200			{object}	FeesList
//	@Router			/chain/fees/reference/{reference}/page/{page} [get]
func (a *API) chainFeesListByReferenceAndPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseFeesParams(
		ctx.URLParam(ParamPage),
		"",
		ctx.URLParam(ParamReference),
		"",
		"",
	)
	if err != nil {
		return err
	}

	if params.Reference == "" {
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

// chainFeesListByTypeAndPageHandler
//
//	@Summary		List all token fees by type
//	@Description	Returns the token fees list filtered by type and ordered by date. A spending is an amount of tokens burnt from one account for executing transactions.
//	@Deprecated
//	@Description	(deprecated, in favor of /chain/fees?page=xxx)
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			type	path		string	true	"Type filter"
//	@Param			page	path		number	true	"Page"
//	@Success		200		{object}	FeesList
//	@Router			/chain/fees/type/{type}/page/{page} [get]
func (a *API) chainFeesListByTypeAndPageHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseFeesParams(
		ctx.URLParam(ParamPage),
		"",
		"",
		ctx.URLParam(ParamType),
		"",
	)
	if err != nil {
		return err
	}

	if params.Type == "" {
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

// feesList produces a filtered, paginated FeesList.
//
// Errors returned are always of type APIerror.
func (a *API) feesList(params *FeesParams) (*FeesList, error) {
	if params.AccountID != "" && !a.indexer.AccountExists(params.AccountID) {
		return nil, ErrAccountNotFound
	}

	fees, total, err := a.indexer.TokenFeesList(
		params.Limit,
		params.Page*params.Limit,
		params.Type,
		params.Reference,
		params.AccountID,
	)
	if err != nil {
		return nil, ErrIndexerQueryFailed.WithErr(err)
	}

	pagination, err := calculatePagination(params.Page, params.Limit, total)
	if err != nil {
		return nil, err
	}

	list := &FeesList{
		Fees:       fees,
		Pagination: pagination,
	}
	return list, nil
}

// chainTransfersListHandler
//
//	@Summary		List all token transfers
//	@Description	Returns the token transfers list ordered by date.
//	@Tags			Chain
//	@Accept			json
//	@Produce		json
//	@Param			page			query		number	false	"Page"
//	@Param			limit			query		number	false	"Items per page"
//	@Param			accountId		query		string	false	"Specific accountId that sent or received the tokens"
//	@Param			accountIdFrom	query		string	false	"Specific accountId that sent the tokens"
//	@Param			accountIdTo		query		string	false	"Specific accountId that received the tokens"
//	@Success		200				{object}	TransfersList
//	@Router			/chain/transfers [get]
func (a *API) chainTransfersListHandler(_ *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	params, err := parseTransfersParams(
		ctx.QueryParam(ParamPage),
		ctx.QueryParam(ParamLimit),
		ctx.QueryParam(ParamAccountId),
		ctx.QueryParam(ParamAccountIdFrom),
		ctx.QueryParam(ParamAccountIdTo),
	)
	if err != nil {
		return err
	}

	list, err := a.transfersList(params)
	if err != nil {
		return err
	}

	return marshalAndSend(ctx, list)
}

// transfersList produces a filtered, paginated TransfersList.
//
// Errors returned are always of type APIerror.
func (a *API) transfersList(params *TransfersParams) (*TransfersList, error) {
	for _, param := range []string{params.AccountID, params.AccountIDFrom, params.AccountIDTo} {
		if param != "" && !a.indexer.AccountExists(param) {
			return nil, ErrAccountNotFound.With(param)
		}
	}

	transfers, total, err := a.indexer.TokenTransfersList(
		params.Limit,
		params.Page*params.Limit,
		params.AccountID,
		params.AccountIDFrom,
		params.AccountIDTo,
	)
	if err != nil {
		return nil, ErrIndexerQueryFailed.WithErr(err)
	}

	pagination, err := calculatePagination(params.Page, params.Limit, total)
	if err != nil {
		return nil, err
	}

	list := &TransfersList{
		Transfers:  transfers,
		Pagination: pagination,
	}
	return list, nil
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

// parseOrganizationParams returns an OrganizationParams filled with the passed params.
//
// paramSortBy and paramOrder are carried through as given; organizationList
// validates and defaults them, so that the endpoints taking an OrganizationParams
// from a JSON body are covered by the same check.
func parseOrganizationParams(paramPage, paramLimit, paramOrganizationID, paramName,
	paramSortBy, paramOrder string,
) (*OrganizationParams, error) {
	pagination, err := parsePaginationParams(paramPage, paramLimit)
	if err != nil {
		return nil, err
	}

	return &OrganizationParams{
		PaginationParams: pagination,
		OrganizationID:   util.TrimHex(paramOrganizationID),
		Name:             paramName,
		SortBy:           paramSortBy,
		Order:            paramOrder,
	}, nil
}

// parseFeesParams returns an FeesParams filled with the passed params
func parseFeesParams(paramPage, paramLimit, paramReference, paramType, paramAccountId string) (*FeesParams, error) {
	pagination, err := parsePaginationParams(paramPage, paramLimit)
	if err != nil {
		return nil, err
	}

	return &FeesParams{
		PaginationParams: pagination,
		Reference:        util.TrimHex(paramReference),
		Type:             paramType,
		AccountID:        util.TrimHex(paramAccountId),
	}, nil
}

// parseTransfersParams returns an TransfersParams filled with the passed params
func parseTransfersParams(paramPage, paramLimit, paramAccountId, paramAccountIdFrom, paramAccountIdTo string) (*TransfersParams, error) {
	pagination, err := parsePaginationParams(paramPage, paramLimit)
	if err != nil {
		return nil, err
	}

	return &TransfersParams{
		PaginationParams: pagination,
		AccountID:        util.TrimHex(paramAccountId),
		AccountIDFrom:    util.TrimHex(paramAccountIdFrom),
		AccountIDTo:      util.TrimHex(paramAccountIdTo),
	}, nil
}

// parseTransactionParams returns an TransactionParams filled with the passed params
func parseTransactionParams(paramPage, paramLimit, paramHash, paramHeight, paramType, paramSubtype, paramSigner string) (*TransactionParams, error) {
	pagination, err := parsePaginationParams(paramPage, paramLimit)
	if err != nil {
		return nil, err
	}

	height, err := parseNumber(paramHeight)
	if err != nil {
		return nil, err
	}

	return &TransactionParams{
		PaginationParams: pagination,
		Hash:             util.TrimHex(paramHash),
		Height:           uint64(height),
		Type:             paramType,
		Subtype:          paramSubtype,
		Signer:           util.TrimHex(paramSigner),
	}, nil
}

// parseBlockParams returns an BlockParams filled with the passed params
func parseBlockParams(paramPage, paramLimit, paramChainId, paramHash, paramProposerAddress string) (*BlockParams, error) {
	pagination, err := parsePaginationParams(paramPage, paramLimit)
	if err != nil {
		return nil, err
	}

	return &BlockParams{
		PaginationParams: pagination,
		ChainID:          paramChainId,
		Hash:             util.TrimHex(paramHash),
		ProposerAddress:  util.TrimHex(paramProposerAddress),
	}, nil
}
