// Package vochain provides all the functions for creating and managing a vocdoni voting blockchain
package vochain

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.vocdoni.io/dvote/config"

	tmcfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	mempl "github.com/tendermint/tendermint/mempool"

	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	"go.vocdoni.io/dvote/log"
)

// NewVochain starts a node with an ABCI application
func NewVochain(vochaincfg *config.VochainCfg, genesis []byte) *BaseApplication {
	// creating new vochain app
	app, err := NewBaseApplication(vochaincfg.DataDir + "/data")
	if err != nil {
		log.Fatalf("cannot init vochain application: %s", err)
	}
	log.Info("creating tendermint node and application")
	app.Node, err = newTendermint(app, vochaincfg, genesis)
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Node.Start(); err != nil {
		log.Fatal(err)
	}
	// Set mempool function for removing transactions
	app.State.MemPoolRemoveTxKey = app.Node.Mempool().(*mempl.CListMempool).RemoveTxByKey
	// Set function for informing the State if the blockchain is synchronizing or not.
	// This function needs to be part of the state so modules importing only the state are
	// able to query on the curret sync state.
	app.State.IsSynchronizing = app.IsSynchronizing
	// Create custom logger for mempool
	logDisable := false
	if vochaincfg.LogLevelMemPool == "none" {
		logDisable = true
		vochaincfg.LogLevelMemPool = "error"
	}
	logger, err := tmflags.ParseLogLevel(
		vochaincfg.LogLevelMemPool,
		NewTenderLogger("mempool", logDisable),
		tmcfg.DefaultLogLevel)
	if err != nil {
		log.Errorf("failed to parse log level: %v", err)
	}
	app.Node.Mempool().(*mempl.CListMempool).SetLogger(logger)

	return app
}

// TenderLogger implements tendermint's Logger interface, with a couple of
// modifications.
//
// First, it routes the logs to go-dvote's logger, so that we don't end up with
// two loggers writing directly to stdout or stderr.
//
// Second, because we generally don't care about tendermint errors such as
// failures to connect to peers, we route all log levels to our debug level.
// They will only surface if dvote's log level is "debug".
type TenderLogger struct {
	keyvals  []interface{}
	Artifact string
	Disabled bool
}

var _ tmlog.Logger = (*TenderLogger)(nil)

// TODO(mvdan): use zap's WithCallerSkip so that we show the position
// information corresponding to where tenderLogger was called, instead of just
// the pointless positions here.

func (l *TenderLogger) Debug(msg string, keyvals ...interface{}) {
	if !l.Disabled {
		log.Infow(fmt.Sprintf("[%s] %s", l.Artifact, msg), keyvals...)
	}
}

func (l *TenderLogger) Info(msg string, keyvals ...interface{}) {
	if !l.Disabled {
		log.Infow(fmt.Sprintf("[%s] %s", l.Artifact, msg), keyvals...)
	}
}

func (l *TenderLogger) Error(msg string, keyvals ...interface{}) {
	if !l.Disabled {
		log.Warnw(fmt.Sprintf("[%s] %s", l.Artifact, msg), keyvals...)
	}
}

func (l *TenderLogger) With(keyvals ...interface{}) tmlog.Logger {
	// Make sure we copy the values, to avoid modifying the parent.
	// TODO(mvdan): use zap's With method directly.
	l2 := &TenderLogger{Artifact: l.Artifact, Disabled: l.Disabled}
	l2.keyvals = append(l2.keyvals, l.keyvals...)
	l2.keyvals = append(l2.keyvals, keyvals...)
	return l2
}

// NewTenderLogger creates a Tenderming compatible logger for specified artifact
func NewTenderLogger(artifact string, disabled bool) *TenderLogger {
	return &TenderLogger{Artifact: artifact, Disabled: disabled}
}

// we need to set init (first time validators and oracles)
func newTendermint(app *BaseApplication, localConfig *config.VochainCfg, genesis []byte) (*nm.Node, error) {
	// create node config
	var err error

	tconfig := tmcfg.DefaultConfig()
	tconfig.FastSyncMode = true
	tconfig.SetRoot(localConfig.DataDir)
	os.MkdirAll(localConfig.DataDir+"/config", 0755)
	os.MkdirAll(localConfig.DataDir+"/data", 0755)

	// p2p config
	tconfig.LogLevel = localConfig.LogLevel
	tconfig.RPC.ListenAddress = "tcp://" + localConfig.RPCListen
	tconfig.P2P.ListenAddress = "tcp://" + localConfig.P2PListen
	tconfig.P2P.AllowDuplicateIP = false
	tconfig.P2P.AddrBookStrict = true
	if localConfig.Dev {
		tconfig.P2P.AllowDuplicateIP = true
		tconfig.P2P.AddrBookStrict = false
		tconfig.P2P.HandshakeTimeout = time.Second * 10
	}
	tconfig.P2P.ExternalAddress = localConfig.PublicAddr
	log.Infof("announcing external address %s", tconfig.P2P.ExternalAddress)
	if !localConfig.CreateGenesis {
		tconfig.P2P.Seeds = strings.Trim(strings.Join(localConfig.Seeds, ","), "[]\"")
		if _, ok := Genesis[localConfig.Chain]; len(tconfig.P2P.Seeds) < 8 && !localConfig.SeedMode && ok {
			tconfig.P2P.Seeds = strings.Join(Genesis[localConfig.Chain].SeedNodes, ",")
		}
		log.Infof("seed nodes: %s", tconfig.P2P.Seeds)
	}

	if len(localConfig.Peers) > 0 {
		tconfig.P2P.PersistentPeers = strings.Trim(strings.Join(localConfig.Peers, ","), "[]\"")
	}
	if len(tconfig.P2P.PersistentPeers) > 0 {
		log.Infof("persistent peers: %s", tconfig.P2P.PersistentPeers)
	}

	tconfig.P2P.SeedMode = localConfig.SeedMode
	tconfig.RPC.CORSAllowedOrigins = []string{"*"}

	// consensus config
	blockTime := 10
	if localConfig.MinerTargetBlockTimeSeconds > 0 {
		blockTime = localConfig.MinerTargetBlockTimeSeconds
	}
	tconfig.Consensus.TimeoutProposeDelta = time.Millisecond * 200
	tconfig.Consensus.TimeoutPropose = time.Second * time.Duration(float32(blockTime)*0.6)
	tconfig.Consensus.TimeoutPrevoteDelta = time.Millisecond * 200
	tconfig.Consensus.TimeoutPrevote = time.Second * 1
	tconfig.Consensus.TimeoutPrecommitDelta = time.Millisecond * 200
	tconfig.Consensus.TimeoutPrecommit = time.Second * 1
	tconfig.Consensus.TimeoutCommit = time.Second * time.Duration(blockTime)

	log.Infof("consensus block time target: commit=%.2fs propose=%.2fs",
		tconfig.Consensus.TimeoutCommit.Seconds(), tconfig.Consensus.TimeoutPropose.Seconds())

	// indexing
	tconfig.TxIndex.Indexer = "kv"

	// mempool config
	tconfig.Mempool.Size = localConfig.MempoolSize
	tconfig.Mempool.Recheck = false
	tconfig.Mempool.KeepInvalidTxsInCache = true
	tconfig.Mempool.MaxTxsBytes = int64(tconfig.Mempool.Size * tconfig.Mempool.MaxTxBytes)
	tconfig.Mempool.CacheSize = 100000

	//	tconfig.Mempool.MaxBatchBytes = 500 * tconfig.Mempool.MaxTxBytes // maximum 500 full-size txs

	// tmdbBackend defaults to goleveldb, but switches to cleveldb if
	// -tags=cleveldb is used, and switches to badgerdb if -tags=badger is
	// used. See tmdb_*.go.
	// TODO: probably switch to just badger after some benchmarking.
	tconfig.DBBackend = string(tmdbBackend)

	if localConfig.Genesis != "" && !localConfig.CreateGenesis {
		if isAbs := strings.HasPrefix(localConfig.Genesis, "/"); !isAbs {
			dir, err := os.Getwd()
			if err != nil {
				log.Fatal(err)
			}
			tconfig.Genesis = dir + "/" + localConfig.Genesis

		} else {
			tconfig.Genesis = localConfig.Genesis
		}
	} else {
		tconfig.Genesis = tconfig.GenesisFile()
	}

	if err := tconfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("config is invalid: %w", err)
	}

	// create tendermint logger
	logDisable := false
	if tconfig.LogLevel == "none" {
		logDisable = true
		tconfig.LogLevel = "error"
	}
	logger, err := tmflags.ParseLogLevel(tconfig.LogLevel, NewTenderLogger("tendermint", logDisable), tmcfg.DefaultLogLevel)
	if err != nil {
		log.Errorf("failed to parse log level: %v", err)
	}

	// read or create private validator
	pv, err := NewPrivateValidator(localConfig.MinerKey, tconfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create validator key and state: (%v)", err)
	}
	pv.Save()

	log.Infof("tendermint private key %x", pv.Key.PrivKey)
	log.Infof("tendermint address: %s", pv.Key.Address)
	aminoPrivKey, aminoPubKey, err := AminoKeys(pv.Key.PrivKey.(crypto25519.PrivKey))
	if err != nil {
		return nil, err
	}
	log.Infof("amino private key: %s", aminoPrivKey)
	log.Infof("amino public key: %s", aminoPubKey)
	log.Infof("using keyfile %s", tconfig.PrivValidatorKeyFile())

	// nodekey is used for the p2p transport layer
	var nodeKey *p2p.NodeKey
	if len(localConfig.NodeKey) > 0 {
		nodeKey, err = NewNodeKey(localConfig.NodeKey, tconfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create node key: %w", err)
		}
	} else {
		if nodeKey, err = p2p.LoadOrGenNodeKey(tconfig.NodeKeyFile()); err != nil {
			return nil, fmt.Errorf("cannot create or load node key: %w", err)
		}
	}
	log.Infof("tendermint p2p node ID: %s", nodeKey.ID())

	// read or create genesis file
	if tmos.FileExists(tconfig.Genesis) {
		log.Infof("found genesis file %s", tconfig.Genesis)
	} else {
		log.Debugf("loaded genesis: %s", string(genesis))
		if err := os.WriteFile(tconfig.Genesis, genesis, 0600); err != nil {
			return nil, err
		}
		log.Infof("new genesis created, stored at %s", tconfig.Genesis)
	}

	if localConfig.TendermintMetrics {
		tconfig.Instrumentation = &tmcfg.InstrumentationConfig{
			Prometheus:           true,
			PrometheusListenAddr: "",
			MaxOpenConnections:   1,
			Namespace:            "tendermint",
		}
	}

	// create node
	node, err := nm.NewNode(
		tconfig,
		pv,      // the node val
		nodeKey, // node key
		proxy.NewLocalClientCreator(app),
		// Note we use proxy.NewLocalClientCreator here to create a local client
		// instead of one communicating through a socket or gRPC.
		nm.DefaultGenesisDocProviderFunc(tconfig),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(tconfig.Instrumentation),
		logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create new Tendermint node: %w", err)
	}
	log.Debugf("consensus config %+v", *node.Config().Consensus)
	return node, nil
}

// AminoKeys is a helper function that transforms a standard EDDSA key into
// Tendermint like amino format useful for creating genesis files.
func AminoKeys(key crypto25519.PrivKey) (private, public []byte, err error) {
	aminoKey, err := tmjson.Marshal(key)
	if err != nil {
		return nil, nil, err
	}

	aminoPubKey, err := tmjson.Marshal(key.PubKey())
	if err != nil {
		return nil, nil, err
	}

	return aminoKey, aminoPubKey, nil
}
