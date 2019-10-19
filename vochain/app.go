package vochain

import (
	"encoding/json"
	"fmt"
	"strconv"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	vlog "gitlab.com/vocdoni/go-dvote/log"
	vochain "gitlab.com/vocdoni/go-dvote/types"
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	State *VochainState
}

var _ abcitypes.Application = (*BaseApplication)(nil)

// NewBaseApplication creates a new BaseApplication given a name an a DB backend
func NewBaseApplication(dbpath string) (*BaseApplication, error) {
	s, err := NewVochainState(dbpath)
	if err != nil {
		return nil, fmt.Errorf("cannot create vochain state: (%s)", err.Error())
	}
	return &BaseApplication{
		State: s,
	}, nil
}

// Info Return information about the application state.
// Used to sync Tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the Header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during Commit,
// ensuring that Commit is never called twice for the same block height.
func (app *BaseApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	// print some basic version info about tendermint components (coreVersion, p2pVersion, blockVersion)
	vlog.Infof("tendermint Core version: %s", req.Version)
	vlog.Infof("tendermint P2P protocol version: %d", req.P2PVersion)
	vlog.Infof("tendermint Block protocol version: %d", req.BlockVersion)

	// gets the app header from database
	var height int64
	_, heightBytes := app.State.AppTree.Get([]byte("height"))
	if heightBytes != nil {
		err := app.State.Codec.UnmarshalBinaryBare(heightBytes, &height)
		if err != nil {
			vlog.Errorf("cannot unmarshal header from database")
		}
	} else {
		vlog.Infof("initializing tendermint application database for first time, height %d", 0)
	}
	//vlog.Infof("height : %d", header)
	// gets the app hash from database
	var appHashBytes []byte
	_, appHashBytes = app.State.AppTree.Get([]byte(appHashKey))
	if appHashBytes != nil {
		vlog.Infof("app hash: %x", appHashBytes)
	} else {
		vlog.Warnf("app hash is empty")
		appHashBytes = []byte{}
	}
	return abcitypes.ResponseInfo{
		LastBlockHeight:  height,
		LastBlockAppHash: appHashBytes,
	}
}

// InitChain called once upon genesis
// ResponseInitChain can return a list of validators. If the list is empty,
// Tendermint will use the validators loaded in the genesis file.
func (app *BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	var appState vochain.AppState
	err := json.Unmarshal(req.AppStateBytes, &appState)
	if err != nil {
		vlog.Errorf("cannot unmarshal app state bytes: %s", err.Error())
	}
	for _, v := range appState.Oracles {
		app.State.AddOracle(v)
	}
	for i := 0; i < len(appState.Validators); i++ {
		p, err := strconv.ParseInt(appState.Validators[i].Power, 10, 64)
		if err != nil {
			vlog.Errorf("cannot parse power from validator: %s", err.Error())
		}
		app.State.AddValidator(appState.Validators[i].PubKey.Value, p)
	}
	initHeight, err := app.State.Codec.MarshalBinaryBare(0)
	if err != nil {
		vlog.Errorf("cannot marshal initial height: %s", err)
	}
	app.State.AppTree.Set([]byte("height"), initHeight)
	app.State.Save()
	return abcitypes.ResponseInitChain{}
}

// BeginBlock signals the beginning of a new block. Called prior to any DeliverTxs.
// The header contains the height, timestamp, and more - it exactly matches the Tendermint block header.
// The LastCommitInfo and ByzantineValidators can be used to determine rewards and punishments for the validators.
func (app *BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	if app.State.Lock {
		vlog.Warn("app state is locked")
	} else {
		app.State.Lock = true
		app.State.Rollback()
	}
	header, err := app.State.Codec.MarshalBinaryBare(req.Header)
	if err != nil {
		vlog.Error("cannot marshal height")
	}
	app.State.AppTree.Set([]byte(headerKey), header)
	return abcitypes.ResponseBeginBlock{}
}

func (BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

func (app *BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	if _, _, err := ValidateTx(req.Tx, app.State); err != nil {
		return abcitypes.ResponseCheckTx{Code: 1, Info: err.Error()}
	}
	return abcitypes.ResponseCheckTx{Code: 0}
}

func (app *BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	if err := ValidateAndDeliverTx(req.Tx, app.State); err != nil {
		return abcitypes.ResponseDeliverTx{Code: 1}
	}
	return abcitypes.ResponseDeliverTx{Code: 0}
}

func (app *BaseApplication) Commit() abcitypes.ResponseCommit {
	app.State.AppTree.Get([]byte("height"))
	app.State.Save()
	app.State.Lock = false
	return abcitypes.ResponseCommit{
		Data: app.State.GetHash(),
	}
}
func (app *BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	var reqData vochain.QueryData
	err := json.Unmarshal(req.GetData(), &reqData)
	if err != nil {
		return abcitypes.ResponseQuery{Code: 1, Info: fmt.Sprintf("cannot unmarshal request in app query: %s", err)}
	}
	switch reqData.Method {
	case "getEnvelopeStatus":
		p, err := app.State.GetProcess(fmt.Sprintf("%s_%s", reqData.ProcessID, reqData.Nullifier))
		if err != nil || p == nil {
			return abcitypes.ResponseQuery{Code: 0, Info: "", Value: []byte{0}}
		} else {
			return abcitypes.ResponseQuery{Code: 0, Info: "", Value: []byte{1}}
		}
	case "getEnvelope":
		p, err := app.State.GetProcess(fmt.Sprintf("%s_%s", reqData.ProcessID, reqData.Nullifier)) // nullifier hash(addr+pid), processId by pid_nullifier
		if err != nil {
			return abcitypes.ResponseQuery{Code: 1, Info: fmt.Sprintf("cannot get process: %s", err.Error()), Value: make([]byte, 0)}
		}
		pBytes, err := app.State.Codec.MarshalBinaryBare(p)
		if err != nil {
			return abcitypes.ResponseQuery{Code: 1, Info: "cannot marshal processBytes", Value: make([]byte, 0)}
		}
		return abcitypes.ResponseQuery{Code: 0, Info: "", Value: pBytes}
	case "getEnvelopeHeight":
		votes := app.State.CountVotes(reqData.ProcessID)
		vBytes, err := app.State.Codec.MarshalBinaryBare(votes)
		if err != nil {
			return abcitypes.ResponseQuery{Code: 1, Info: "cannot marshal votes count bytes", Value: make([]byte, 0)}
		}
		return abcitypes.ResponseQuery{Code: 0, Info: "", Value: vBytes}
	case "getBlockHeight":
		return abcitypes.ResponseQuery{Code: 0, Info: "", Value: app.State.GetHeight()}
	case "getProcessList":
		return abcitypes.ResponseQuery{Code: 1, Info: "not implemented", Value: make([]byte, 0)}
	case "getEnvelopeList":
		return abcitypes.ResponseQuery{Code: 1, Info: "not implemented", Value: make([]byte, 0)}
	default:
		return abcitypes.ResponseQuery{Code: 1, Info: "undefined query method"}
	}
}

func (app *BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
