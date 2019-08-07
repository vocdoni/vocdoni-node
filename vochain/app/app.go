package vochain

import (
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
	return abcitypes.ResponseDeliverTx{Code: 0}
}

// CheckTx called by Tendermint for every transaction received from the network users,
// before it enters the mempool. This is intended to filter out transactions to avoid
// filling out the mempool and polluting the blocks with invalid transactions.
// At this level, only the basic checks are performed
// Here we do some basic sanity checks around the raw Tx received.
func (BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	tx := splitTx(req.Tx)
	m := tx.ValidateMethod()
	vlog.Infof("Is valid method? %s", m)
	// reject because of method
	if !m {
		return abcitypes.ResponseCheckTx{Code: 1}
	}
	a := tx.ValidateArgs()
	// reject because of args
	if !a {
		return abcitypes.ResponseCheckTx{Code: 1}
	}
	return abcitypes.ResponseCheckTx{Info: tx.String(), Code: 0}
}

func (BaseApplication) Commit() abcitypes.ResponseCommit {
	return abcitypes.ResponseCommit{}
}

func (BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}

// ______________________ INITCHAIN ______________________

func (BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	return abcitypes.ResponseInitChain{}
}

func (BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	return abcitypes.ResponseBeginBlock{}
}

func (BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
