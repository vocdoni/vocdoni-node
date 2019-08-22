package vochain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

	abci "github.com/tendermint/tendermint/abci/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	vlog "gitlab.com/vocdoni/go-dvote/log"

	voctypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

// database keys
var (
	processesKey          = []byte("processesKey")
	validatorsPubKKey     = []byte("validatorsPubKKey")
	trustedOraclesPubKKey = []byte("trustedOraclesPubKKey")
	heightKey             = []byte("heightKey")
	appHashKey            = []byte("appHashKey")
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	height  int64  // heigth is the number of blocks of the app
	appHash []byte // appHash is the root hash of the app
	db      dbm.DB // database allowing processes be persistent

	// volatile states
	// see https://tendermint.com/docs/spec/abci/apps.html#state
	checkTxState   *voctypes.State // checkState is set on initialization and reset on Commit
	deliverTxState *voctypes.State // deliverState is set on InitChain and BeginBlock and cleared on Commit
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

// Info Return information about the application state.
// Used to sync Tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the Header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during Commit,
// ensuring that Commit is never called twice for the same block height.
func (app *BaseApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {

	// print some basic version info about tendermint components (coreVersion, p2pVersion, blockVersion)
	vlog.Infof("Tendermint Core version: %v", req.Version)
	vlog.Infof("Tendermint P2P protocol version: %v", req.P2PVersion)
	vlog.Infof("Tendermint Block protocol version: %v", req.BlockVersion)

	// gets the app height from database
	var height int64
	heightBytes := app.db.Get(heightKey)
	if len(heightBytes) != 0 {
		err := json.Unmarshal(heightBytes, &height)
		if err != nil {
			// error if cannot unmarshal height from database
			vlog.Errorf("Cannot unmarshal Height from database")
		}
		vlog.Infof("Height from abci.Info : %v", height)
	} else {
		// database height value is empty
		vlog.Infof("Height from abci.Info is %v, initializing tendermint application database for first time", height)
	}

	// gets the app hash from database
	appHashBytes := app.db.Get(appHashKey)
	if len(appHashBytes) != 0 {
		vlog.Infof("AppHash from abci.Info : %v", appHashBytes)
	} else {
		vlog.Infof("AppHash is empty")
	}

	// return info required during the handshake that happens on startup
	return abcitypes.ResponseInfo{
		LastBlockHeight:  height,
		LastBlockAppHash: appHashBytes,
	}
}

// SetOption set non-consensus critical application specific options.
func (BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{
		Info: "SetOption is void",
	}
}

// DeliverTx is the workhorse of the application, non-optional. Executes the transaction in full
func (app *BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	// we can't commit transactions inside the DeliverTx because in such case Query, which may be called in parallel, will return inconsistent data
	// split incomin tx
	tx, err := SplitAndCheckTxBytes(req.Tx)
	if err != nil {
		vlog.Info(err)
		return abcitypes.ResponseDeliverTx{Code: 1}
	}

	// switch by method
	switch tx.Method {
	// new process tx
	case "newProcessTx":
		npta := tx.Args.(voctypes.NewProcessTxArgs)
		// check if process exists
		if _, ok := app.deliverTxState.Processes[npta.MkRoot]; !ok {
			// if not create new
			app.deliverTxState.Processes[npta.MkRoot] = &voctypes.Process{
				EntityID:       npta.EntityID,
				Votes:          make(map[string]*voctypes.Vote, 0),
				MkRoot:         npta.MkRoot,
				NumberOfBlocks: npta.NumberOfBlocks,
				InitBlock:      npta.InitBlock,
				CurrentState:   voctypes.Scheduled,
				EncryptionKeys: npta.EncryptionKeys,
			}
		} else {
			// process exists, return process data as info
			vlog.Info("The process already exists with the following data: \n")
			vlog.Infof("Process data: %v", app.deliverTxState.Processes[npta.MkRoot].String())
		}
	case "voteTx":
		vta := tx.Args.(voctypes.VoteTxArgs)
		// check if vote has a valid processID
		if _, ok := app.deliverTxState.Processes[vta.ProcessID]; ok {
			// check if vote is already submitted
			if _, ok := app.deliverTxState.Processes[vta.ProcessID].Votes[vta.Nullifier]; !ok {
				app.deliverTxState.Processes[vta.ProcessID].Votes[vta.Nullifier] = &voctypes.Vote{
					Payload:     vta.Payload,
					CensusProof: vta.CensusProof,
				}
			} else {
				vlog.Info("Vote already submitted")
			}
		} else {
			vlog.Info("Process does not exist")
		}
	}

	// marshall processes
	newBytes, err := json.Marshal(app.deliverTxState.Processes)
	if err != nil {
		vlog.Errorf("Cannot marshal DeliverTxState processes")
	}
	// save process into db
	app.db.Set(processesKey, newBytes)

	return abcitypes.ResponseDeliverTx{Info: tx.String(), Code: 0}
}

// CheckTx called by Tendermint for every transaction received from the network users,
// before it enters the mempool. This is intended to filter out transactions to avoid
// filling out the mempool and polluting the blocks with invalid transactions.
// At this level, only the basic checks are performed
// Here we do some basic sanity checks around the raw Tx received.
func (app *BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {

	// check raw tx data and returns OK if matches with any defined ValixTx schema
	tx, err := SplitAndCheckTxBytes(req.Tx)
	if err != nil {
		vlog.Info(err)
		return abcitypes.ResponseCheckTx{Code: 1}
	}
	return abcitypes.ResponseCheckTx{Info: tx.String(), Code: 0}
}

// Commit persist the application state
func (app *BaseApplication) Commit() abcitypes.ResponseCommit {
	// update app height
	b := []byte(strconv.FormatInt(app.height, 10))
	app.db.Set(heightKey, b)

	// marhsall state
	state, err := json.Marshal(*app.deliverTxState)
	if err != nil {
		vlog.Errorf("Cannot marshal processes")
	}
	// hash of the state
	h := sha256.New()
	h.Write(state)
	app.appHash = h.Sum(nil)
	app.db.Set(appHashKey, app.appHash)
	// reset deliverTxState
	app.deliverTxState = voctypes.NewState()

	// return apphash as data to be included into the block
	return abcitypes.ResponseCommit{
		Data: app.appHash,
	}

}

// Query query for data from the application at current or past height.
func (BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}

// ______________________ INITCHAIN ______________________

// InitChain called once upon genesis
// ResponseInitChain can return a list of validators. If the list is empty,
// Tendermint will use the validators loaded in the genesis file.
func (app *BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	app.deliverTxState = voctypes.NewState()
	// load processes from db
	var processes map[string]*voctypes.Process
	processesBytes := app.db.Get(processesKey)
	if len(processesBytes) != 0 {
		err := json.Unmarshal(processesBytes, &processes)
		if err != nil {
			vlog.Errorf("Cannot unmarshal processes")
		}
		app.deliverTxState.Processes = processes
	}

	// load validators public keys from db
	var valk []tmtypes.Address
	validatorBytes := app.db.Get(validatorsPubKKey)
	if len(validatorBytes) != 0 {
		err := json.Unmarshal(validatorBytes, &valk)
		if err != nil {
			vlog.Errorf("Cannot unmarshal validators public keys")
		}
		app.deliverTxState.ValidatorsPubK = valk
	}

	// load trusted oracles public keys from db
	var orlk []tmtypes.Address
	oraclesBytes := app.db.Get(trustedOraclesPubKKey)
	if len(oraclesBytes) != 0 {
		err := json.Unmarshal(oraclesBytes, &orlk)
		if err != nil {
			vlog.Errorf("Cannot unmarshal trusted oracles public keys")
		}
		app.deliverTxState.TrustedOraclesPubK = orlk
	}

	// load chain height from db
	var height int64
	heightBytes := app.db.Get(heightKey)
	if len(heightBytes) != 0 {
		err := json.Unmarshal(heightBytes, &height)
		if err != nil {
			vlog.Errorf("Cannot unmarshal height")
		}
	}
	app.height = height

	// load apphash from db
	app.appHash = app.db.Get(appHashKey)

	return abcitypes.ResponseInitChain{}
}

func (app *BaseApplication) validateHeight(req abci.RequestBeginBlock) error {
	if req.Header.Height < 1 {
		return fmt.Errorf("invalid height: %d", req.Header.Height)
	}
	return nil
}

// BeginBlock signals the beginning of a new block. Called prior to any DeliverTxs.
// The header contains the height, timestamp, and more - it exactly matches the Tendermint block header.
// The LastCommitInfo and ByzantineValidators can be used to determine rewards and punishments for the validators.
func (app *BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	// validate chain height
	if err := app.validateHeight(req); err != nil {
		panic(err)
	}

	// load processes from db
	var processes map[string]*voctypes.Process
	processesBytes := app.db.Get(processesKey)
	if len(processesBytes) != 0 {
		err := json.Unmarshal(processesBytes, &processes)
		if err != nil {
			vlog.Errorf("Cannot unmarshal processes")
		}
		app.deliverTxState.Processes = processes
	}

	// load validators public keys from db
	var valk []tmtypes.Address
	validatorBytes := app.db.Get(validatorsPubKKey)
	if len(validatorBytes) != 0 {
		err := json.Unmarshal(validatorBytes, &valk)
		if err != nil {
			vlog.Errorf("Cannot unmarshal validators public keys")
		}
		app.deliverTxState.ValidatorsPubK = valk
	}

	// load trusted oracles public keys from db
	var orlk []tmtypes.Address
	oraclesBytes := app.db.Get(trustedOraclesPubKKey)
	if len(oraclesBytes) != 0 {
		err := json.Unmarshal(oraclesBytes, &orlk)
		if err != nil {
			vlog.Errorf("Cannot unmarshal trusted oracles public keys")
		}
		app.deliverTxState.TrustedOraclesPubK = orlk
	}

	// app height and app hash from the request
	app.height = req.Header.Height
	app.appHash = req.Header.AppHash

	vlog.Infof("DB CONTENT ON BEGIN BLOCK: %v", app.deliverTxState)
	return abcitypes.ResponseBeginBlock{}
}

// EndBlock Signals the end of a block.
//
// Called after all transactions, prior to each Commit.
// Validator updates returned by block H impact blocks H+1, H+2, and H+3, but only effects changes on the validator set of H+2:
// 	- H+1: NextValidatorsHash
//	- H+2: ValidatorsHash (and thus the validator set)
//	- H+3: LastCommitInfo (ie. the last validator set)
// Consensus params returned for block H apply for block H+1
//
func (app *BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
