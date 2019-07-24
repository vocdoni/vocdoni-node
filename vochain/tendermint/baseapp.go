package tendermint

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tendermint/libs/db"
)

// State represents the application internal state
type State struct {
	db      dbm.DB
	Size    int64  `json:"size"`
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
}

// BaseApp reflects the ABCI application implementation.
type BaseApp struct {
	name            string                // application name from abci.Info
	appVersion      string                // application's version string
	consensusParams *abci.ConsensusParams // consensus params
	state           State                 // application's state
}

var _ abci.Application = (*BaseApp)(nil)

// NewBaseApp initialize a base app with a fresh db
func NewBaseApp(name string, appVersion string) *BaseApp {
	state := loadState(dbm.NewMemDB())
	app := &BaseApp{
		state:      state,
		name:       name,
		appVersion: appVersion,
	}
	return app
}

// Name returns the name of the BaseApp.
func (app *BaseApp) Name() string {
	return app.name
}

// setConsensusParams memoizes the consensus params.
func (app *BaseApp) setConsensusParams(consensusParams *abci.ConsensusParams) {
	app.consensusParams = consensusParams
}

func (app *BaseApp) Info(req abci.RequestInfo) abci.ResponseInfo {
	return abci.ResponseInfo{
		Data: fmt.Sprintf("{\"size\":%v, \"height\":%v, \"appHash\":%X}", app.state.Size, app.state.Height, app.state.AppHash),
	}
}

func (app *BaseApp) CheckTx(req abci.RequestCheckTx) (res abci.ResponseCheckTx) {
	return abci.ResponseCheckTx{
		Code: code.CodeTypeOK,
	}
}

func (app *BaseApp) Commit() (res abci.ResponseCommit) {
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash
	app.state.Height++
	saveState(app.state)
	return abci.ResponseCommit{
		Data: appHash,
	}
}

func (app *BaseApp) InitChain(req abci.RequestInitChain) (res abci.ResponseInitChain)    { return }
func (app *BaseApp) SetOption(req abci.RequestSetOption) (res abci.ResponseSetOption)    { return }
func (app *BaseApp) BeginBlock(req abci.RequestBeginBlock) (res abci.ResponseBeginBlock) { return }
func (app *BaseApp) DeliverTx(req abci.RequestDeliverTx) (res abci.ResponseDeliverTx)    { return }
func (app *BaseApp) EndBlock(req abci.RequestEndBlock) (res abci.ResponseEndBlock)       { return }
func (app *BaseApp) Query(req abci.RequestQuery) (res abci.ResponseQuery)                { return }

func loadState(db dbm.DB) State {
	stateBytes := db.Get([]byte("stateKey"))
	var state State
	if len(stateBytes) != 0 {
		err := json.Unmarshal(stateBytes, &state)
		if err != nil {
			log.Fatalf("Cannot load App state: %+v", err)
		}
	}
	state.db = db
	return state
}

func saveState(state State) {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	state.db.Set([]byte("stateKey"), stateBytes)
}
