package mock

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tlog "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proxy"
	"gitlab.com/vocdoni/go-dvote/log"

	"github.com/tendermint/tendermint/abci/example/code"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	pvm "github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// State represents the application internal state
type State struct {
	db      dbm.DB
	Size    int64  `json:"size"`
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
}

type CounterApplication struct {
	abcitypes.BaseApplication
	state     State
	hashCount int
	txCount   int
	serial    bool
}

func NewCounterApplication(serial bool) *CounterApplication {
	return &CounterApplication{serial: serial}
}

func (app *CounterApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{Data: fmt.Sprintf("{\"hashes\":%v,\"txs\":%v}", app.hashCount, app.txCount)}
}

func (app *CounterApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
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

func (app *CounterApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
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

func (app *CounterApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
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

func (app *CounterApplication) Commit() (resp abcitypes.ResponseCommit) {
	app.hashCount++
	if app.txCount == 0 {
		return abcitypes.ResponseCommit{}
	}
	hash := make([]byte, 8)
	binary.BigEndian.PutUint64(hash, uint64(app.txCount))
	return abcitypes.ResponseCommit{Data: hash}
}

func (app *CounterApplication) Query(reqQuery abcitypes.RequestQuery) abcitypes.ResponseQuery {
	log.Infof("%+v", "In Query")
	switch reqQuery.Path {
	case "hash":
		return abcitypes.ResponseQuery{Value: []byte(fmt.Sprintf("%v", app.hashCount))}
	case "tx":
		return abcitypes.ResponseQuery{Value: []byte(fmt.Sprintf("%v", app.txCount))}
	default:
		return abcitypes.ResponseQuery{Log: fmt.Sprintf("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}
	}
}

func (app *CounterApplication) InitChain(reqInit abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	log.Infof("%+v", "Initializing Chain")
	ctx := NewDefaultContext()
	_, err := startInProcess(ctx)
	if err != nil {
		log.Errorf("%+v", err)
	}
	return abcitypes.ResponseInitChain{}
}

type Context struct {
	Config *cfg.Config
	Logger tlog.Logger
}

func NewDefaultContext() *Context {
	return NewContext(
		cfg.DefaultConfig(),
		tlog.NewTMLogger(tlog.NewSyncWriter(os.Stdout)),
	)
}

func NewContext(config *cfg.Config, logger tlog.Logger) *Context {
	return &Context{config, logger}
}

var DBBackend = ""

// NewLevelDB instantiate a new LevelDB instance according to DBBackend.
func NewLevelDB(name, dir string) (db dbm.DB, err error) {
	backend := dbm.GoLevelDBBackend
	if DBBackend == string(dbm.CLevelDBBackend) {
		backend = dbm.CLevelDBBackend
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("couldn't create db: %v", r)
		}
	}()
	return dbm.NewDB(name, backend, dir), err
}

func openDB(rootDir string) (dbm.DB, error) {
	log.Infof("%+v", "Creating levelDB...")
	dataDir := filepath.Join(rootDir, "tendermint_data")
	db, err := NewLevelDB("application", dataDir)
	return db, err
}

func startInProcess(ctx *Context) (*node.Node, error) {
	cfg := ctx.Config
	home := cfg.RootDir

	db, err := openDB(home)
	if err != nil {
		return nil, err
	}

	app := NewCounterApplication(false)
	app.state.db = db

	// private validator
	privValKeyFile := cfg.PrivValidatorKeyFile()
	privValStateFile := cfg.PrivValidatorStateFile()
	var pv *pvm.FilePV
	if cmn.FileExists(privValKeyFile) {
		pv = pvm.LoadFilePV(privValKeyFile, privValStateFile)
		log.Infof("Found private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	} else {
		pv = pvm.GenFilePV(privValKeyFile, privValStateFile)
		pv.Save()
		log.Infof("Generated private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}

	nodeKeyFile := cfg.NodeKeyFile()
	if cmn.FileExists(nodeKeyFile) {
		log.Infof("Found node key", "path", nodeKeyFile)
	}
	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyFile)
	if err != nil {
		log.DPanicf("Cannot load or generate node key: %v", err)
		//return err
	}
	log.Info("Generated node key", "path", nodeKeyFile)

	// genesis file
	genFile := cfg.GenesisFile()
	if cmn.FileExists(genFile) {
		log.Infof("Found genesis file", "path", genFile)
	} else {
		genDoc := tmtypes.GenesisDoc{
			ChainID:         fmt.Sprintf("test-chain-%v", cmn.RandStr(6)),
			GenesisTime:     tmtime.Now(),
			ConsensusParams: tmtypes.DefaultConsensusParams(),
		}
		key := pv.GetPubKey()
		genDoc.Validators = []tmtypes.GenesisValidator{{
			Address: key.Address(),
			PubKey:  key,
			Power:   10,
		}}

		if err := genDoc.SaveAs(genFile); err != nil {
			log.DPanicf("Cannot load or generate genesis file: %v", err)
			//return err
		}
		log.Info("Generated genesis file", "path", genFile)
	}
	// create & start tendermint node
	tmNode, err := node.NewNode(
		cfg,
		pvm.LoadOrGenFilePV(privValKeyFile, privValStateFile),
		nodeKey,
		proxy.NewLocalClientCreator(app),
		node.DefaultGenesisDocProviderFunc(cfg),
		node.DefaultDBProvider,
		node.DefaultMetricsProvider(cfg.Instrumentation),
		ctx.Logger.With("module", "node"),
	)
	/*
		n, err := node.DefaultNewNode(cfg, ctx.Logger.With("module", "node"))
		if err != nil {
			log.DPanicf("Failed to create node: %v", err)
		}*/

	// Stop upon receiving SIGTERM or CTRL-C.
	cmn.TrapSignal(ctx.Logger.With("module", "node"), func() {
		if tmNode.IsRunning() {
			tmNode.Stop()
		}
	})

	if err := tmNode.Start(); err != nil {
		log.DPanicf("Failed to start node: %v", err)
	}
	log.Info("Started node", "nodeInfo", tmNode.Switch().NodeInfo())

	// run forever (the node will not be returned)
	select {}
}

// TrapSignal traps SIGINT and SIGTERM and terminates the server correctly.
func TrapSignal(cleanupFunc func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		switch sig {
		case syscall.SIGTERM:
			defer cleanupFunc()
			os.Exit(128 + int(syscall.SIGTERM))
		case syscall.SIGINT:
			defer cleanupFunc()
			os.Exit(128 + int(syscall.SIGINT))
		}
	}()
}
