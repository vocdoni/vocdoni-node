package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
)

const (
	ChainHandler = "chain"
)

var (
	ErrTransactionNotFound = fmt.Errorf("transaction hash not found")
	ErrBlockNotFound       = fmt.Errorf("block not found")
)

func (a *API) enableChainHandlers() error {
	if err := a.endpoint.RegisterMethod(
		"/chain/organizations",
		"GET",
		apirest.MethodAccessTypePublic,
		a.organizationListHandler,
	); err != nil {
		return err
	}
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

	return nil
}

// /chain/organizations/pages/<page>
// list the existing organizations
func (a *API) organizationListHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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

	list := a.indexer.EntityList(MaxPageSize, page, "")
	for _, orgID := range list {
		organization.Organizations = append(organization.Organizations, &OrganizationList{
			OrganizationID: orgID,
			ElectionCount:  a.indexer.ProcessCount(orgID),
		})
	}

	var data []byte
	if data, err = json.Marshal(organization); err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /chain/organizations/count
// return the number of organizations
func (a *API) organizationCountHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	count := a.indexer.EntityCount()
	organization := &Organization{Count: &count}
	data, err := json.Marshal(organization)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)

}

// /chain/info
// returns the chain ID, blocktimes, timestamp and height of the blockchain
func (a *API) chainInfoHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	chainID := a.vocapp.ChainID()
	blockTimes := a.vocinfo.BlockTimes()
	height := a.vocapp.Height()
	timestamp := a.vocapp.Timestamp()
	// Get current circuit tag
	circuitConfigTag := a.vocapp.CircuitConfigurationTag()

	data, err := json.Marshal(ChainInfo{
		ID:                      chainID,
		BlockTime:               blockTimes,
		Height:                  &height,
		Timestamp:               &timestamp,
		CircuitConfigurationTag: circuitConfigTag,
	})
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /chain/info/circuit
// returns the circuit configuration according to the current circuit
func (a *API) chainCircuitInfoHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	// Get current circuit tag
	circuitConfig := circuit.GetCircuitConfiguration(a.vocapp.CircuitConfigurationTag())
	// Set LocalDir parameter to empty to be omitted
	circuitConfig.LocalDir = ""
	// Encode the circuit configuration to JSON
	data, err := json.Marshal(circuitConfig)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /chain/dateToblock/<timestamp>
// returns the estimated block height for the timestamp provided
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
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// POST /chain/transactions
// submits a blockchain transaction
func (a *API) chainSendTxHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /chain/transaction/cost
// returns de list of transactions and its cost
func (a *API) chainTxCostHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
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
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /chain/transactions/page/<page>
func (a *API) chainTxListPaginated(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	page, err := strconv.Atoi(ctx.URLParam("page"))
	if err != nil {
		return err
	}
	offset := int32(page * MaxPageSize)
	refs, err := a.indexer.GetLastTxReferences(MaxPageSize, offset)
	if err != nil {
		return err
	}
	data, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /chain/transactions/reference/<hash>
func (a *API) chainTxbyHashHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	hash, err := hex.DecodeString(util.TrimHex(ctx.URLParam("hash")))
	if err != nil {
		return err
	}
	ref, err := a.indexer.GetTxHashReference(hash)
	if err != nil {
		return ErrTransactionNotFound
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}

	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// /chain/transactions/<height>/<index>
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
		return fmt.Errorf("cannot get tx: %w", err)
	}
	return ctx.Send([]byte(protoFormat(stx.Tx)), apirest.HTTPstatusCodeOK)
}

// /chain/transactions/reference/index/<index>
func (a *API) chainTxByIndexHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	index, err := strconv.ParseUint(ctx.URLParam("index"), 10, 64)
	if err != nil {
		return err
	}
	ref, err := a.indexer.GetTxReference(index)
	if err != nil {
		return fmt.Errorf("cannot get tx: %w", err)
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// GET /chain/validators
// returns the list of validators
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
	return ctx.Send(data, apirest.HTTPstatusCodeOK)
}

// GET /chain/blocks/<height>
// returns the block at the given height
func (a *API) chainBlockHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	height, err := strconv.ParseInt(ctx.URLParam("height"), 10, 64)
	if err != nil {
		return err
	}
	block := a.vocapp.GetBlockByHeight(height)
	if block == nil {
		return ErrBlockNotFound
	}
	data, err := json.Marshal(block)
	if err != nil {
		return err
	}

	return ctx.Send(convertKeysToCamel(data), apirest.HTTPstatusCodeOK)
}

// GET /chain/blocks/hash/<hash>
// returns the block from the given hash
func (a *API) chainBlockByHashHandler(msg *apirest.APIdata, ctx *httprouter.HTTPContext) error {
	hash, err := hex.DecodeString(util.TrimHex(ctx.URLParam("hash")))
	if err != nil {
		return err
	}
	block := a.vocapp.GetBlockByHash(hash)
	if block == nil {
		return ErrBlockNotFound
	}
	data, err := json.Marshal(block)
	if err != nil {
		return err
	}
	return ctx.Send(convertKeysToCamel(data), apirest.HTTPstatusCodeOK)
}
