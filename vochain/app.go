package vochain

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"gitlab.com/vocdoni/go-dvote/log"

	"github.com/tendermint/tendermint/abci/example/code"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/version"
	dbm "github.com/tendermint/tm-cmn/db"
)

var (
	stateKey                         = []byte("stateKey")
	ProtocolVersion version.Protocol = 0x1
)

// State represents the application internal state
type State struct {
	db      dbm.DB
	Size    int64  `json:"size"`
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
}

func loadState(db dbm.DB) State {
	stateBytes := db.Get(stateKey)
	var state State
	if len(stateBytes) != 0 {
		err := json.Unmarshal(stateBytes, &state)
		if err != nil {
			panic(err)
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
	state.db.Set(stateKey, stateBytes)
}

type VotingApplication struct {
	abcitypes.BaseApplication
	state     State
	hashCount int
	txCount   int
	serial    bool
}

func NewVotingApplication(serial bool, dbName string, dbDir string) *VotingApplication {
	db, err := dbm.NewGoLevelDB(dbName, dbDir)
	if err != nil {
		log.DPanicf("Cannot initialize application db : %v", err)
		//return err
	}
	state := loadState(db)
	return &VotingApplication{state: state}
}

func (app *VotingApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{Data: fmt.Sprintf("{\"hashes\":%v,\"txs\":%v}", app.hashCount, app.txCount)}
}

func (app *VotingApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	key, value := req.Key, req.Value
	if key == "serial" && value == "on" {
		app.serial = true
	} else {
		/*
			TODO Panic and have the ABCI server pass an exception.
			The client can call SetOptionSync() and get an `error`.
			return types.ResponseSetOption{
				Error: fmt.Sprintf("Unknown key (%s) or value (%s)", key, value),
			}
		*/
		return abcitypes.ResponseSetOption{}
	}

	return abcitypes.ResponseSetOption{}
}

func (app *VotingApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	if app.serial {
		if len(req.Tx) > 8 {
			return abcitypes.ResponseDeliverTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Max tx size is 8 bytes, got %d", len(req.Tx))}
		}
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(req.Tx):], req.Tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue != uint64(app.txCount) {
			return abcitypes.ResponseDeliverTx{
				Code: code.CodeTypeBadNonce,
				Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.txCount, txValue)}
		}
	}
	app.txCount++
	return abcitypes.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *VotingApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	if app.serial {
		if len(req.Tx) > 8 {
			return abcitypes.ResponseCheckTx{
				Code: code.CodeTypeEncodingError,
				Log:  fmt.Sprintf("Max tx size is 8 bytes, got %d", len(req.Tx))}
		}
		tx8 := make([]byte, 8)
		copy(tx8[len(tx8)-len(req.Tx):], req.Tx)
		txValue := binary.BigEndian.Uint64(tx8)
		if txValue < uint64(app.txCount) {
			return abcitypes.ResponseCheckTx{
				Code: code.CodeTypeBadNonce,
				Log:  fmt.Sprintf("Invalid nonce. Expected >= %v, got %v", app.txCount, txValue)}
		}
	}
	return abcitypes.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *VotingApplication) Commit() (resp abcitypes.ResponseCommit) {
	app.hashCount++
	if app.txCount == 0 {
		return abcitypes.ResponseCommit{}
	}
	hash := make([]byte, 8)
	binary.BigEndian.PutUint64(hash, uint64(app.txCount))
	saveState(app.state)
	return abcitypes.ResponseCommit{Data: hash}
}

func (app *VotingApplication) Query(reqQuery abcitypes.RequestQuery) abcitypes.ResponseQuery {
	log.Infof("%+v", "In Query")
	switch reqQuery.Path {
	case "hash":
		return abcitypes.ResponseQuery{Value: []byte(fmt.Sprintf("%v", app.hashCount))}
	case "tx":
		return abcitypes.ResponseQuery{Value: []byte(fmt.Sprintf("%v", app.txCount))}
	case "state":
		state := loadState(app.state.db)
		return abcitypes.ResponseQuery{Value: []byte(fmt.Sprintf("%+v\n\n%+v", app.state, state))}
	default:
		return abcitypes.ResponseQuery{Log: fmt.Sprintf("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}
	}
}

func (app *VotingApplication) InitChain(reqInit abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	log.Infof("%+v", "Initializing Chain")
	return abcitypes.ResponseInitChain{}
}
