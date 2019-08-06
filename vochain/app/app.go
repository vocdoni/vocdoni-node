package vochain

import (
	"strings"

	"github.com/gogo/protobuf/proto"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tm-db"
	vlog "gitlab.com/vocdoni/go-dvote/log"
	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	name  string          // application name from ABCI info
	db    dbm.DB          // common DB backend
	store *voctypes.Store // main state (non volatile state)
	// volatile state
	checkState   *voctypes.State // checkState is set on initialization and reset on Commit
	deliverState *voctypes.State // deliverState is set on InitChain and BeginBlock and cleared on Commit

	consensusParams *abcitypes.ConsensusParams
}

var _ abcitypes.Application = (*BaseApplication)(nil)

// NewBaseApplication creates a new BaseApplication given a name an a DB backend
func NewBaseApplication(db dbm.DB, name string) *BaseApplication {
	return &BaseApplication{
		name:  name,
		db:    db,
		store: voctypes.NewStore(db),
	}
}

func (BaseApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{}
}

func (BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

/*
In the current design, a block can include incorrect transactions (those who passed CheckTx,
but failed DeliverTx or transactions included by the proposer directly). This is done for performance reasons.

WHAT DOES IT MEAN FOR US
*/

func (BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	// we can't commit transactions inside the DeliverTx because in such case Query, which may be called in parallel, will return inconsistent data
	// CHECK IF THE REQ IS VALID
	// CHECK IF THE SIGNATURE IS VALID
	// CHECK IF THE VOTE EXISTS
	// CHECK IF THE VOTER IS IN THE CENSUS
	tx := splitTx(req.Tx)
	m, err := tx.GetMethod()
	vlog.Infof("%+v", m)
	if err != nil {
		//
	}
	return abcitypes.ResponseDeliverTx{Code: 0}
}

func (BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	// CHECK IF REQ IS VALID
	tx := splitTx(req.Tx)
	m, err := tx.GetMethod()
	vlog.Infof("%s", m)
	vlog.Infof("%s", err)

	if err != nil {
		// reject
		return abcitypes.ResponseCheckTx{Code: 1}
	}
	// TODO CHECK ARGS METHOD
	return abcitypes.ResponseCheckTx{Info: strings.Join(tx.GetArgs(), m.String()), Code: 0}
}

func (BaseApplication) Commit() abcitypes.ResponseCommit {
	return abcitypes.ResponseCommit{}
}

func (BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}

// ______________________ INITCHAIN ______________________

func (app *BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	// stash the consensus params in the store and memoize
	if req.ConsensusParams != nil {
		app.setConsensusParams(req.ConsensusParams)
		app.storeConsensusParams(req.ConsensusParams)
	}
	initHeader := abcitypes.Header{ChainID: req.ChainId, Time: req.Time}
	// initialize the deliver state and check state with a correct header
	app.setDeliverState(initHeader)
	app.setCheckState(initHeader)

	return abcitypes.ResponseInitChain{
		Validators:      req.Validators,
		ConsensusParams: req.ConsensusParams,
	}
}

// setConsensusParams memoizes the consensus params.
func (app *BaseApplication) setConsensusParams(consensusParams *abcitypes.ConsensusParams) {
	app.consensusParams = consensusParams
}

// setConsensusParams stores the consensus params.
func (app *BaseApplication) storeConsensusParams(consensusParams *abcitypes.ConsensusParams) {
	consensusParamsBz, err := proto.Marshal(consensusParams)
	if err != nil {
		panic(err)
	}
	app.db.Set([]byte("consensus_params"), consensusParamsBz)
}

// setCheckState sets checkState with the store and the context wrapping it.
// It is called by InitChain() and Commit()
func (app *BaseApplication) setCheckState(header abcitypes.Header) {
	store := app.store
	state := new(voctypes.State)
	state.SetStore(*store)
	state.SetContext(voctypes.NewContext(*store, header, true))
	app.deliverState = state
}

// setDeliverState sets checkState with the store and the context wrapping it.
// It is called by InitChain() and BeginBlock(), and deliverState is set nil on Commit().
func (app *BaseApplication) setDeliverState(header abcitypes.Header) {
	store := app.store
	state := new(voctypes.State)
	state.SetStore(*store)
	state.SetContext(voctypes.NewContext(*store, header, false))
	app.deliverState = state
}

func (BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	return abcitypes.ResponseBeginBlock{}
}

func (BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
