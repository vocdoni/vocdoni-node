package vochain

import (
	"fmt"

	ethcommon "github.com/ethereum/go-ethereum/common"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	mempl "github.com/tendermint/tendermint/mempool"
	nm "github.com/tendermint/tendermint/node"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"google.golang.org/protobuf/proto"

	models "github.com/vocdoni/dvote-protobuf/build/go/models"
	"gitlab.com/vocdoni/go-dvote/log"
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	State *State
	Node  *nm.Node
}

var _ abcitypes.Application = (*BaseApplication)(nil)

// NewBaseApplication creates a new BaseApplication given a name an a DB backend
func NewBaseApplication(dbpath string) (*BaseApplication, error) {
	state, err := NewState(dbpath)
	if err != nil {
		return nil, fmt.Errorf("cannot create vochain state: (%s)", err)
	}
	return &BaseApplication{
		State: state,
	}, nil
}

// SendTX sends a transaction to the mempool (sync)
func (app *BaseApplication) SendTX(tx []byte) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *abcitypes.Response, 1)
	defer close(resCh)
	err := app.Node.Mempool().CheckTx(tx, func(res *abcitypes.Response) {
		resCh <- res
	}, mempl.TxInfo{})
	if err != nil {
		return nil, err
	}
	res := <-resCh
	r := res.GetCheckTx()
	return &ctypes.ResultBroadcastTx{
		Code: r.Code,
		Data: r.Data,
		Log:  r.Log,
		Hash: tmtypes.Tx(tx).Hash(),
	}, nil
}

// Info Return information about the application state.
// Used to sync Tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the Header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during Commit,
// ensuring that Commit is never called twice for the same block height.
func (app *BaseApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	// print some basic version info about tendermint components (coreVersion, p2pVersion, blockVersion)
	log.Infof("tendermint Core version: %s", req.Version)
	log.Infof("tendermint P2P protocol version: %d", req.P2PVersion)
	log.Infof("tendermint Block protocol version: %d", req.BlockVersion)
	var height int64
	header := app.State.Header(false)
	if header != nil {
		height = header.Height
	}
	app.State.Rollback()
	hash := app.State.AppHash(false)
	log.Infof("replaying blocks. Current height %d, current APP hash %x", height, hash)
	return abcitypes.ResponseInfo{
		LastBlockHeight:  height,
		LastBlockAppHash: hash,
	}
}

// InitChain called once upon genesis
// ResponseInitChain can return a list of validators. If the list is empty,
// Tendermint will use the validators loaded in the genesis file.
func (app *BaseApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	// setting the app initial state with validators, oracles, height = 0 and empty apphash
	// unmarshal app state from genesis
	var genesisAppState models.AppState
	err := proto.Unmarshal(req.AppStateBytes, &genesisAppState)
	if err != nil {
		log.Errorf("cannot unmarshal app state bytes: %s", err)
	}
	// get oracles
	for _, v := range genesisAppState.GetOracles() {
		log.Infof("adding genesis oracle %s", v)
		app.State.AddOracle(ethcommon.BytesToAddress(v))
	}
	// get validators
	for i := 0; i < len(genesisAppState.Validators); i++ {
		log.Infof("adding genesis validator %s", genesisAppState.Validators[i].Address)
		if err = app.State.AddValidator(genesisAppState.Validators[i]); err != nil {
			log.Fatal(err)
		}
	}

	var header models.TendermintHeader
	header.Height = 0
	header.AppHash = []byte{}
	header.ChainId = req.ChainId
	headerBytes, err := proto.Marshal(&header)
	if err != nil {
		log.Fatalf("cannot marshal header: %s", err)
	}
	app.State.Lock()
	if err = app.State.Store.Tree(AppTree).Add(headerKey, headerBytes); err != nil {
		log.Fatal(err)
	}
	app.State.Unlock()
	app.State.Save() // Is this save needed?
	// TBD: using empty list here, should return validatorsUpdate to use the validators obtained here
	return abcitypes.ResponseInitChain{}
}

// BeginBlock signals the beginning of a new block. Called prior to any DeliverTxs.
// The header contains the height, timestamp, and more - it exactly matches the Tendermint block header.
// The LastCommitInfo and ByzantineValidators can be used to determine rewards and punishments for the validators.
func (app *BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	header := &models.TendermintHeader{
		Height:         req.Header.GetHeight(),
		ConsensusHash:  req.Header.GetConsensusHash(),
		AppHash:        req.Header.GetAppHash(),
		BlockID:        req.Header.LastBlockId.GetHash(),
		DataHash:       req.Header.GetDataHash(),
		EvidenceHash:   req.Header.GetEvidenceHash(),
		LastCommitHash: req.Header.GetLastCommitHash(),
		Timestamp:      req.Header.GetTime().Unix(),
	}
	headerBytes, err := proto.Marshal(header)
	if err != nil {
		log.Warnf("cannot marshal header in BeginBlock")
	}
	// reset app state to latest persistent data
	app.State.Rollback()
	app.State.Lock()
	if err = app.State.Store.Tree(AppTree).Add(headerKey, headerBytes); err != nil {
		log.Fatal(err)
	}
	app.State.Unlock()
	app.State.VoteCachePurge(app.State.Header(true).Height)
	return abcitypes.ResponseBeginBlock{}
}

func (BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

func (app *BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	var data []byte
	var err error
	var tx *models.Tx
	if req.Type == abcitypes.CheckTxType_Recheck {
		return abcitypes.ResponseCheckTx{Code: 0, Data: data}
	}
	if tx, err = UnmarshalTx(req.Tx); err == nil {
		if data, err = AddTx(tx, app.State, false); err != nil {
			log.Debugf("checkTx error: %s", err)
			return abcitypes.ResponseCheckTx{Code: 1, Data: []byte("addTx " + err.Error())}
		}
	} else {
		return abcitypes.ResponseCheckTx{Code: 1, Data: []byte("unmarshalTx " + err.Error())}
	}
	return abcitypes.ResponseCheckTx{Code: 0, Data: data}
}

func (app *BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	var data []byte
	var err error
	var tx *models.Tx

	if tx, err = UnmarshalTx(req.Tx); err == nil {
		if data, err = AddTx(tx, app.State, true); err != nil {
			return abcitypes.ResponseDeliverTx{Code: 1, Data: []byte(err.Error())}
		}
	} else {
		return abcitypes.ResponseDeliverTx{Code: 1, Data: []byte(err.Error())}
	}
	return abcitypes.ResponseDeliverTx{Code: 0, Data: data}
}

func (app *BaseApplication) Commit() abcitypes.ResponseCommit {
	return abcitypes.ResponseCommit{
		Data: app.State.Save(),
	}
}

func (app *BaseApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{}
}

func (app *BaseApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
