package vochain

import (
	"strings"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tm-db"
	vlog "gitlab.com/vocdoni/go-dvote/log"
	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

type BaseApplication struct {
	db             dbm.DB
	name           string
	checkTxState   *voctypes.State //
	deliverTxState *voctypes.State // the working state for block execution
}

var _ abcitypes.Application = (*BaseApplication)(nil)

func NewBaseApplication(db dbm.DB, name string) *BaseApplication {
	return &BaseApplication{
		db:   db,
		name: name,
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

func (BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	return abcitypes.ResponseInitChain{}
}

func (BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	return abcitypes.ResponseBeginBlock{}
}

func (BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
