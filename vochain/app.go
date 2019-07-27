package vochain

import (
	vlog "gitlab.com/vocdoni/go-dvote/log"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tm-cmn/db"
)

type BaseApplication struct {
	db      *dbm.GoLevelDB
	name    string
	appHash string
}

var _ abcitypes.Application = (*BaseApplication)(nil)

func NewBaseApplication(db *dbm.GoLevelDB) *BaseApplication {
	return &BaseApplication{
		db: db,
	}
}

func (BaseApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{}
}

func (BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

func (BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	return abcitypes.ResponseDeliverTx{Code: 0}
}

func (BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	return abcitypes.ResponseCheckTx{Code: 0}
}

func (BaseApplication) Commit() abcitypes.ResponseCommit {
	return abcitypes.ResponseCommit{}
}

func (BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}

func (BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	vlog.Infof("GETAPPSTATEBYTES: %+v", req.GetAppStateBytes())
	return abcitypes.ResponseInitChain{}
}

func (BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	return abcitypes.ResponseBeginBlock{}
}

func (BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
