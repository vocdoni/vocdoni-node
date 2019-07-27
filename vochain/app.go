package vochain

import (
	"github.com/cosmos/cosmos-sdk/codec"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tm-cmn/db"
	vlog "gitlab.com/vocdoni/go-dvote/log"
)

type BaseApplication struct {
	db   *dbm.GoLevelDB
	name string
}

var _ abcitypes.Application = (*BaseApplication)(nil)

func NewBaseApplication(db *dbm.GoLevelDB, name string) *BaseApplication {
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

func (BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	return abcitypes.ResponseDeliverTx{Code: 0}
}

func (BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	_ = splitTx(req.Tx)
	return abcitypes.ResponseCheckTx{Code: 0}
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

func splitTx(content []byte) error {
	vlog.Infof("%+v", content)
	var validTx ValidTx
	err := codec.Cdc.UnmarshalJSON(content, &validTx)
	vlog.Infof("%+v", err)
	return nil

}
