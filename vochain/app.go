package vochain

import (
	"fmt"

	amino "github.com/tendermint/go-amino"
	abci "github.com/tendermint/tendermint/abci/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	cryptoAmino "github.com/tendermint/tendermint/crypto/encoding/amino"
	mempl "github.com/tendermint/tendermint/mempool"
	nm "github.com/tendermint/tendermint/node"
	voclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
	"gitlab.com/vocdoni/go-dvote/types"
)

// BaseApplication reflects the ABCI application implementation.
type BaseApplication struct {
	State  *State
	Codec  *amino.Codec
	Node   *nm.Node
	Client *voclient.HTTP
}

var _ abcitypes.Application = (*BaseApplication)(nil)

// NewBaseApplication creates a new BaseApplication given a name an a DB backend
func NewBaseApplication(dbpath string) (*BaseApplication, error) {
	c := amino.NewCodec()
	cryptoAmino.RegisterAmino(c)
	s, err := NewState(dbpath, c)
	if err != nil {
		return nil, fmt.Errorf("cannot create vochain state: (%s)", err)
	}
	return &BaseApplication{
		State: s,
		Codec: c,
	}, nil
}

// SendTX sends a transaction to the mempool (sync)
func (app *BaseApplication) SendTX(tx []byte) (*ctypes.ResultBroadcastTx, error) {
	var t tmtypes.Tx
	t = tx
	resCh := make(chan *abci.Response, 1)
	err := app.Node.Mempool().CheckTx(tx, func(res *abci.Response) {
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
		Hash: t.Hash(),
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
	hash := app.State.AppHash(false)
	log.Infof("current height is %d, current APP hash is %x", height, hash)
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
	var genesisAppState types.GenesisAppState
	err := app.Codec.UnmarshalJSON(req.AppStateBytes, &genesisAppState)
	if err != nil {
		log.Errorf("cannot unmarshal app state bytes: %s", err)
	}
	// get oracles
	for _, v := range genesisAppState.Oracles {
		log.Infof("adding genesis oracle %s", v)
		app.State.AddOracle(v)
	}
	// get validators
	for i := 0; i < len(genesisAppState.Validators); i++ {
		log.Infof("adding genesis validator %s", genesisAppState.Validators[i].PubKey.Address())
		app.State.AddValidator(genesisAppState.Validators[i].PubKey, genesisAppState.Validators[i].Power)
	}

	var header abcitypes.Header
	header.Height = 0
	header.AppHash = []byte{}
	headerBytes, err := app.Codec.MarshalBinaryBare(header)
	if err != nil {
		log.Errorf("cannot marshal header: %s", err)
	}
	app.State.AppTree.Set(headerKey, headerBytes)
	app.State.Save()

	// TBD: using empty list here, should return validatorsUpdate to use the validators obtained here
	return abcitypes.ResponseInitChain{}
}

// BeginBlock signals the beginning of a new block. Called prior to any DeliverTxs.
// The header contains the height, timestamp, and more - it exactly matches the Tendermint block header.
// The LastCommitInfo and ByzantineValidators can be used to determine rewards and punishments for the validators.
func (app *BaseApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	// reset app state to latest persistent data
	app.State.Rollback()
	// set immutable state

	app.State.ILock.Lock()
	app.State.IAppTree = app.State.AppTree.ImmutableTree
	app.State.IProcessTree = app.State.ProcessTree.ImmutableTree
	app.State.IVoteTree = app.State.VoteTree.ImmutableTree
	app.State.ILock.Unlock()

	headerBytes, err := app.Codec.MarshalBinaryBare(req.Header)
	if err != nil {
		log.Warnf("cannot marshal header in BeginBlock")
	}
	app.State.AppTree.Set(headerKey, headerBytes)
	return abcitypes.ResponseBeginBlock{}
}

func (BaseApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

func (app *BaseApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	var data []byte
	var err error
	var tx GenericTX

	if tx, err = UnmarshalTx(req.Tx); err == nil {
		if data, err = AddTx(tx, app.State, false); err != nil {
			return abcitypes.ResponseCheckTx{Code: 1, Data: []byte(err.Error())}
		}
	} else {
		return abcitypes.ResponseCheckTx{Code: 1, Data: []byte(err.Error())}
	}
	return abcitypes.ResponseCheckTx{Code: 0, Data: data}
}

func (app *BaseApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	var data []byte
	var err error
	var tx GenericTX

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

func (app *BaseApplication) RegisterMetrics(ma *metrics.Agent) {
	ma.Register(VochainHeight)
	ma.Register(VochainMempool)
	ma.Register(VochainAppTree)
	ma.Register(VochainProcessTree)
	ma.Register(VochainVoteTree)
}

func (app *BaseApplication) GetMetrics() {
	VochainHeight.Set(float64(app.Node.BlockStore().Height()))
	VochainMempool.Set(float64(app.Node.Mempool().Size()))
	VochainAppTree.Set(float64(app.State.AppTree.Size()))
	VochainProcessTree.Set(float64(app.State.ProcessTree.Size()))
	VochainVoteTree.Set(float64(app.State.VoteTree.Size()))
}
