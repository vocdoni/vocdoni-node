package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	tmtypes "github.com/cometbft/cometbft/types"
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
	if err := a.endpoint.RegisterMethod(
		"/chain/organizations/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.organizationListHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/organizations/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.organizationCountHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/info",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainInfoHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/info/circuit",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainCircuitInfoHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/info/electionPriceFactors",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainInfoPriceFactors,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/dateToBlock/{timestamp}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainEstimateHeightHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transactions/cost",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxCostHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transactions/reference/{hash}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxbyHashHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transactions/reference/index/{index}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxByIndexHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transactions/reference/height/{height}/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxByHeightHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transactions/{height}/{index}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transactions",
		"POST",
		apirest.MethodAccessTypePublic,
		a.chainSendTxHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transactions/page/{page}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxListPaginated,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/validators",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainValidatorsHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/blocks/{height}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainBlockHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/blocks/hash/{hash}",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainBlockByHashHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/organizations/filter/page/{page}",
		"POST",
		apirest.MethodAccessTypePublic,
		a.chainOrganizationsFilterPaginatedHandler,
	); err != nil {
		return err
	}
	if err := a.endpoint.RegisterMethod(
		"/chain/transactions/count",
		"GET",
		apirest.MethodAccessTypePublic,
		a.chainTxCountHandler,
	); err != nil {
		return err
	}

	return nil
}

// organizationListHandler
//
//	@Summary		List organizations
//	@Description	List the existing organizations
//	@Success		200	{object}	object
//	@Router			/chain/organizations/page/{page} [get]
func (a *API) organizationListHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
	for _, orgID := range list {
		organizations = append(organizations, &OrganizationList{
			OrganizationID: orgID,
			ElectionCount:  a.indexer.ProcessCount(orgID),
		})
	}

	data, err := json.Marshal(struct {
		Organizations []*OrganizationList `json:"organizations"`
	}{organizations})
	if err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusOK)
}

// organizationCountHandler
//
//	@Summary		Organizations count
//	@Description	Return the number of organizations
//	@Success		200	{object}	Organization
//	@Router			/chain/organizations/count [get]
func (a *API) organizationCountHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count := a.indexer.EntityCount()
	organization := &Organization{Count: &count}
	data, err := json.Marshal(organization)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)

}

// chainInfoHandler
//
//	@Summary		Chain info
//	@Description	Returns the chain parameters and info
//	@Success		200	{object}	ChainInfo
//	@Router			/chain/info [get]
func (a *API) chainInfoHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	transactionCount, err := a.indexer.TransactionCount()
	if err != nil {
		return err
	}
	validators, err := a.vocapp.State.Validators(true)
	if err != nil {
		return err
	}
	voteCount, err := a.indexer.CountVotes(nil)
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

	data, err := json.Marshal(&ChainInfo{
		ID:                      a.vocapp.ChainID(),
		BlockTime:               *a.vocinfo.BlockTimes(),
		ElectionCount:           a.indexer.ProcessCount(nil),
		OrganizationCount:       a.indexer.EntityCount(),
		Height:                  a.vocapp.Height(),
		Syncing:                 a.vocapp.IsSynchronizing(),
		TransactionCount:        transactionCount,
		ValidatorCount:          uint32(len(validators)),
		Timestamp:               a.vocapp.Timestamp(),
		VoteCount:               voteCount,
		GenesisTime:             a.vocapp.Genesis().GenesisTime,
		CircuitConfigurationTag: a.vocapp.CircuitConfigurationTag(),
		MaxCensusSize:           maxCensusSize,
		NetworkCapacity:         networkCapacity,
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
//	@Success		200	{object}	circuit.ZkCircuitConfig
//	@Router			/chain/info/circuit [get]
func (a *API) chainCircuitInfoHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// Get current circuit tag
	circuitConfig := circuit.GetCircuitConfiguration(a.vocapp.CircuitConfigurationTag())
	// Encode the circuit configuration to JSON
	data, err := json.Marshal(circuitConfig)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainInfoPriceFactors
//
//	@Summary		Election price factors info
//	@Description	Returns the factors and values used to calculate the election price
//	@Success		200	{object}	election	price	factors
//	@Router			/chain/info/electionPriceFactors [get]
func (a *API) chainInfoPriceFactors(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// Encode the values and factors to JSON
	data, err := json.Marshal(a.vocapp.State.ElectionPriceCalc)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainEstimateHeightHandler
//
//	@Summary		Estimate block for timestamp
//	@Description	Returns the estimated block height for the timestamp provided
//	@Success		200	{object}	object
//	@Router			/chain/dateToBlock/{timestamp} [get]
func (a *API) chainEstimateHeightHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainSendTxHandler
//
//	@Summary		Submit transaction
//	@Description	Submits a blockchain transaction
//	@Success		200	{object}	Transaction
//	@Router			/chain/transactions [post]
func (a *API) chainSendTxHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	req := &Transaction{}
	if err := json.Unmarshal(msg.Data, req); err != nil {
		return ErrCantParseDataAsJSON.WithErr(err)
	}
	res, err := a.vocapp.SendTx(req.Payload)
	if err != nil {
		return ErrVochainSendTxFailed.WithErr(err)
	}
	if res == nil {
		return ErrVochainEmptyReply
	}
	if res.Code != 0 {
		return ErrVochainReturnedErrorCode.Withf("(%d) %s", res.Code, string(res.Data))
	}
	var data []byte
	if data, err = json.Marshal(Transaction{
		Response: res.Data.Bytes(),
		Code:     &res.Code,
		Hash:     res.Hash.Bytes(),
	}); err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxCostHandler
//
//	@Summary		Transaction costs
//	@Description	Returns the list of transactions and its cost
//	@Success		200	{object}	Transaction
//	@Router			/chain/transactions/cost [get]
func (a *API) chainTxCostHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	object
//	@Router			/chain/transactions/page/{page} [get]
func (a *API) chainTxListPaginated(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
	// wrap list in a struct to consistently return list in a object, return empty
	// object if the list does not contains any result
	data, err := json.Marshal(struct {
		Txs []*indexertypes.Transaction `json:"transactions"`
	}{refs})
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusOK)
}

// chainTxbyHashHandler
//
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	indexertypes.TxReference
//	@Router			/chain/transactions/reference/{hash} [get]
func (a *API) chainTxbyHashHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	object
//	@Router			/chain/transactions/{height}/{index} [get]
func (a *API) chainTxHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
//	@Summary		TODO
//	@Description	TODO
//	@Success		200	{object}	indexertypes.TxReference
//	@Router			/chain/transactions/reference/index/{index} [get]
func (a *API) chainTxByIndexHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
//	@Summary		Returns the list of transactions for a given block
//	@Description	Given a block returns the list of transactions for that block
//	@Success		200	{object}	TransactionList
//
//	@Router			/chain/transactions/reference/height/{height}/page/{page} [get]
func (a *API) chainTxByHeightHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
//	@Summary		Validators list
//	@Description	Returns the list of validators
//	@Success		200	{object}	ValidatorList
//	@Router			/chain/validators [get]
func (a *API) chainValidatorsHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	stateValidators, err := a.vocapp.State.Validators(true)
	if err != nil {
		return err
	}
	validators := ValidatorList{}
	for _, v := range stateValidators {
		validators.Validators = append(validators.Validators, Validator{
			Address: v.GetAddress(),
			Power:   v.GetPower(),
			Name:    v.GetName(),
			PubKey:  v.GetPubKey(),
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
//	@Description	Returns the block at the given height
//	@Success		200	{object}	types.Block
//	@Router			/chain/blocks/{height} [get]
func (a *API) chainBlockHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	height, err := strconv.ParseInt(ctx.URLParam("height"), 10, 64)
	if err != nil {
		return err
	}
	tmblock := a.vocapp.GetBlockByHeight(height)
	if tmblock == nil {
		return ErrBlockNotFound
	}
	block := &Block{
		Block: tmtypes.Block{
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
//	@Success		200	{object}	types.Block
//	@Router			/chain/blocks/hash/{hash} [get]
func (a *API) chainBlockByHashHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	hash, err := hex.DecodeString(util.TrimHex(ctx.URLParam("hash")))
	if err != nil {
		return err
	}
	tmblock := a.vocapp.GetBlockByHash(hash)
	if tmblock == nil {
		return ErrBlockNotFound
	}
	block := &Block{
		Block: tmtypes.Block{
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
//	@Summary		Organizations list (paginated)
//	@Description	Returns a list of organizations paginated by the given page
//	@Success		200	{object}	object
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

	for _, orgID := range matchingOrganizationIds {
		organizations = append(organizations, &OrganizationList{
			OrganizationID: orgID,
			ElectionCount:  a.indexer.ProcessCount(orgID),
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
//	@Success		200	{object}	uint64
//	@Router			/chain/transactions/count [get]
func (a *API) chainTxCountHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count, err := a.indexer.TransactionCount()
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
