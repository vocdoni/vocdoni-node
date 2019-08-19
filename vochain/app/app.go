package vochain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	vlog "gitlab.com/vocdoni/go-dvote/log"
	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

var (
	processesKey          = []byte("processeskey")
	validatorsPubKKey     = []byte("validatorsPubKKey")
	trustedOraclesPubKKey = []byte("trustedOraclesPubKKey")
	heightKey             = []byte("heightKey")
	appHashKey            = []byte("appHashKey")
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	// Database allowing processes be persistent
	db dbm.DB `json:"db"`
	// volatile states
	checkTxState   *voctypes.State `json:"checkstate"`   // checkState is set on initialization and reset on Commit
	deliverTxState *voctypes.State `json:"deliverstate"` // deliverState is set on InitChain and BeginBlock and cleared on Commit
}

var _ abcitypes.Application = (*BaseApplication)(nil)

// NewBaseApplication creates a new BaseApplication given a name an a DB backend
func NewBaseApplication(db dbm.DB) *BaseApplication {
	return &BaseApplication{
		db:             db,
		checkTxState:   voctypes.NewState(),
		deliverTxState: voctypes.NewState(),
	}
}

func (BaseApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{}
}

func (BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

func (app *BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	// we can't commit transactions inside the DeliverTx because in such case Query, which may be called in parallel, will return inconsistent data
	tx, err := SplitAndCheckTxBytes(req.Tx)
	if err != nil {
		vlog.Info(err)
		return abcitypes.ResponseDeliverTx{Code: 1}
	}

	switch tx.Method {
	case "newProcessTx":
		var npta = tx.Args.(voctypes.NewProcessTxArgs)
		vlog.Info("TX ARGS STRING %v", npta)
		vlog.Info("TX ARGS STRING END")

		if _, ok := app.deliverTxState.Processes[npta.MkRoot]; ok {
			app.deliverTxState.Processes[npta.MkRoot] = voctypes.Process{
				EntityID:       npta.EntityID,
				Votes:          make([]voctypes.Vote, 0),
				MkRoot:         npta.MkRoot,
				NumberOfBlocks: npta.NumberOfBlocks,
				InitBlock:      npta.InitBlock,
				CurrentState:   voctypes.Scheduled,
				EncryptionKeys: npta.EncryptionKeys,
			}
		}

		newBytes, err := json.Marshal(app.deliverTxState.Processes)
		if err != nil {
			vlog.Errorf("Cannot marshal DeliverTxState processes")
		}
		app.db.Set(processesKey, newBytes)
	}

	return abcitypes.ResponseDeliverTx{Info: tx.String(), Code: 0}
}

// CheckTx called by Tendermint for every transaction received from the network users,
// before it enters the mempool. This is intended to filter out transactions to avoid
// filling out the mempool and polluting the blocks with invalid transactions.
// At this level, only the basic checks are performed
// Here we do some basic sanity checks around the raw Tx received.
func (BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	// basic sanity checks of incoming tx
	tx, err := SplitAndCheckTxBytes(req.Tx)
	if err != nil {
		vlog.Info(err)
		return abcitypes.ResponseCheckTx{Code: 1}
	}
	return abcitypes.ResponseCheckTx{Info: tx.String(), Code: 0}
}

func (app *BaseApplication) Commit() abcitypes.ResponseCommit {
	/*
		heightBytes, err := json.Marshal(app.deliverTxState.Height)
		if err != nil {
			vlog.Errorf("Cannot marshal DeliverTxState height")
		}
		app.db.Set(heightKey, heightBytes)
	*/
	out := app.db.Get(processesKey)
	processes := make(map[string]voctypes.Process, 0)
	_ = json.Unmarshal(out, &processes)
	vlog.Info("DB CONTENT: %v", processes)
	h := sha256.New()
	h.Write(out)
	app.db.Set(appHashKey, h.Sum(nil))
	return abcitypes.ResponseCommit{
		Data: h.Sum(nil),
	}
}

func (BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}

// ______________________ INITCHAIN ______________________

func (app *BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {

	var processes map[string]voctypes.Process
	processesBytes := app.db.Get(processesKey)
	if len(processesBytes) != 0 {
		err := json.Unmarshal(processesBytes, &processes)
		if err != nil {
			vlog.Errorf("Cannot unmarshal processes")
		}
		app.deliverTxState.Processes = processes
	} else {
		app.deliverTxState.Processes = make(map[string]voctypes.Process, 0)
	}

	var validators []tmtypes.Address
	validatorsBytes := app.db.Get(validatorsPubKKey)
	if len(validatorsBytes) != 0 {
		err := json.Unmarshal(validatorsBytes, &validators)
		if err != nil {
			vlog.Errorf("Cannot unmarshal validators")
		}
		app.deliverTxState.ValidatorsPubK = validators
	} else {
		app.deliverTxState.ValidatorsPubK = validators
	}
	var oracles []tmtypes.Address
	oraclesBytes := app.db.Get(trustedOraclesPubKKey)
	if len(oraclesBytes) != 0 {
		err := json.Unmarshal(oraclesBytes, &oracles)
		if err != nil {
			vlog.Errorf("Cannot unmarshal oracles")
		}
		app.deliverTxState.TrustedOraclesPubK = oracles
	} else {
		app.deliverTxState.TrustedOraclesPubK = oracles
	}

	var height int64
	heightBytes := app.db.Get(heightKey)
	if len(heightBytes) != 0 {
		err := json.Unmarshal(heightBytes, &height)
		if err != nil {
			vlog.Errorf("Cannot unmarshal height")
		}
		app.deliverTxState.Height = height
	} else {
		app.deliverTxState.Height = 0
	}

	app.deliverTxState.AppHash = req.GetAppStateBytes()

	return abcitypes.ResponseInitChain{}
}

func (app *BaseApplication) validateHeight(req abci.RequestBeginBlock) error {
	if req.Header.Height < 1 {
		return fmt.Errorf("invalid height: %d", req.Header.Height)
	}
	return nil
}

func (app *BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	// validate chain height
	if err := app.validateHeight(req); err != nil {
		panic(err)
	}
	if app.deliverTxState == nil {
		app.deliverTxState = &voctypes.State{}
	} else {
		app.deliverTxState.Height = req.Header.Height
		app.deliverTxState.AppHash = req.Header.AppHash
	}

	return abcitypes.ResponseBeginBlock{}
}

func (app *BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	app.deliverTxState = nil
	return abcitypes.ResponseEndBlock{}
}
