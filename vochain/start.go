// Package vochain provides all the functions for creating and managing a vocdoni voting blockchain
package vochain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.vocdoni.io/dvote/config"
	vocdoniGenesis "go.vocdoni.io/dvote/vochain/genesis"

	abciclient "github.com/tendermint/tendermint/abci/client"
	tmcfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"

	tmjson "github.com/tendermint/tendermint/libs/json"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/libs/service"
	tmnode "github.com/tendermint/tendermint/node"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/log"
)

// NewVochain starts a node with an ABCI application
func NewVochain(vochaincfg *config.VochainCfg, genesis []byte) *BaseApplication {
	// creating new vochain app
	app, err := NewBaseApplication(vochaincfg.DBType, filepath.Join(vochaincfg.DataDir, "data"))
	if err != nil {
		log.Fatalf("cannot initialize vochain application: %s", err)
	}
	log.Info("creating tendermint node and application")
	err = app.SetNode(vochaincfg, genesis)
	if err != nil {
		log.Fatal(err)
	}
	app.SetDefaultMethods()
	// Set the vote cache at least as big as the mempool size
	if app.State.CacheSize() < vochaincfg.MempoolSize {
		app.State.SetCacheSize(vochaincfg.MempoolSize)
	}
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
	logLevel int // 0:debug 1:info 2:error 3:disabled
}

var _ tmlog.Logger = (*TenderLogger)(nil)

func (l *TenderLogger) SetLogLevel(logLevel string) {
	switch logLevel {
	case "debug":
		l.logLevel = 0
	case "info":
		l.logLevel = 1
	case "error":
		l.logLevel = 2
	case "disabled", "none":
		l.logLevel = 3
	}
}

func (l *TenderLogger) Debug(msg string, keyvals ...interface{}) {
	if l.logLevel == 0 {
		log.Logger().Debug().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

func (l *TenderLogger) Info(msg string, keyvals ...interface{}) {
	if l.logLevel <= 1 {
		log.Logger().Info().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

func (l *TenderLogger) Error(msg string, keyvals ...interface{}) {
	if l.logLevel <= 2 {
		log.Logger().Error().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

func (l *TenderLogger) With(keyvals ...interface{}) tmlog.Logger {
	// Make sure we copy the values, to avoid modifying the parent.
	// TODO(mvdan): use zap's With method directly.
	l2 := &TenderLogger{Artifact: l.Artifact, logLevel: l.logLevel}
	l2.keyvals = append(l2.keyvals, l.keyvals...)
	l2.keyvals = append(l2.keyvals, keyvals...)
	return l2
}

// NewTenderLogger creates a Tendermint compatible logger for specified artifact
func NewTenderLogger(artifact string, logLevel string) *TenderLogger {
	tl := &TenderLogger{Artifact: artifact}
	tl.SetLogLevel(logLevel)
	return tl
}

// newTendermint creates a new tendermint node attached to the given ABCI app
func newTendermint(app *BaseApplication,
	localConfig *config.VochainCfg, genesis []byte) (service.Service, error) {
	var err error

	tconfig := tmcfg.DefaultConfig()
	tconfig.SetRoot(localConfig.DataDir)
	if err := os.MkdirAll(filepath.Join(localConfig.DataDir, "config"), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(localConfig.DataDir, "data"), 0o755); err != nil {
		log.Fatal(err)
	}
	// p2p config
	tconfig.LogLevel = localConfig.LogLevel
	if tconfig.LogLevel == "none" {
		tconfig.LogLevel = "disabled"
	}
	tconfig.RPC.ListenAddress = "tcp://" + localConfig.RPCListen
	tconfig.P2P.ListenAddress = "tcp://" + localConfig.P2PListen
	tconfig.P2P.AllowDuplicateIP = false
	tconfig.P2P.AddrBookStrict = true
	tconfig.P2P.FlushThrottleTimeout = 500 * time.Millisecond
	tconfig.P2P.MaxPacketMsgPayloadSize = 10240
	tconfig.P2P.RecvRate = 5120000
	tconfig.P2P.DialTimeout = time.Second * 5
	if localConfig.Dev {
		tconfig.P2P.AllowDuplicateIP = true
		tconfig.P2P.AddrBookStrict = false
		tconfig.P2P.HandshakeTimeout = time.Second * 10
	}
	tconfig.P2P.ExternalAddress = localConfig.PublicAddr
	log.Infof("announcing external address %s", tconfig.P2P.ExternalAddress)
	tconfig.P2P.BootstrapPeers = strings.Trim(strings.Join(localConfig.Seeds, ","), "[]\"")
	if _, ok := vocdoniGenesis.Genesis[localConfig.Chain]; len(tconfig.P2P.BootstrapPeers) < 8 &&
		!localConfig.IsSeedNode && ok {
		tconfig.P2P.BootstrapPeers = strings.Join(vocdoniGenesis.Genesis[localConfig.Chain].SeedNodes, ",")
	}
	if len(tconfig.P2P.BootstrapPeers) > 0 {
		log.Infof("seed nodes: %s", tconfig.P2P.BootstrapPeers)
	}

	if len(localConfig.Peers) > 0 {
		tconfig.P2P.PersistentPeers = strings.Trim(strings.Join(localConfig.Peers, ","), "[]\"")
	}
	if len(tconfig.P2P.PersistentPeers) > 0 {
		log.Infof("persistent peers: %s", tconfig.P2P.PersistentPeers)
	}
	tconfig.RPC.CORSAllowedOrigins = []string{"*"}

	// consensus config
	blockTime := 8
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

	// Enable only FastSync (until StateSync is implemented)
	tconfig.BlockSync.Enable = true
	tconfig.StateSync.Enable = false

	// if gateway or oracle
	tconfig.Mode = tmcfg.ModeFull
	// if seed node
	if localConfig.IsSeedNode {
		tconfig.Mode = tmcfg.ModeSeed
	} else if len(localConfig.MinerKey) > 0 {
		// if validator
		tconfig.Mode = tmcfg.ModeValidator
	}

	log.Infof("tendermint configured as %s node", tconfig.Mode)
	log.Infof("consensus block time target: commit=%.2fs propose=%.2fs",
		tconfig.Consensus.TimeoutCommit.Seconds(), tconfig.Consensus.TimeoutPropose.Seconds())

	// disable transaction indexer (we don't use it)
	tconfig.TxIndex = &tmcfg.TxIndexConfig{Indexer: []string{"null"}}
	// mempool config
	tconfig.Mempool.Size = localConfig.MempoolSize
	tconfig.Mempool.Recheck = false
	tconfig.Mempool.KeepInvalidTxsInCache = false
	tconfig.Mempool.MaxTxBytes = 1024 * 100 // 100 KiB
	tconfig.Mempool.MaxTxsBytes = int64(tconfig.Mempool.Size * tconfig.Mempool.MaxTxBytes)
	tconfig.Mempool.CacheSize = 100000
	tconfig.Mempool.Broadcast = true
	tconfig.Mempool.MaxBatchBytes = 500 * tconfig.Mempool.MaxTxBytes // maximum 500 full-size txs
	// Set mempool TTL to 15 minutes
	tconfig.Mempool.TTLDuration = time.Minute * 15
	tconfig.Mempool.TTLNumBlocks = 100
	log.Infof("mempool config: %+v", tconfig.Mempool)

	// tmdbBackend defaults to goleveldb, but switches to cleveldb if
	// -tags=cleveldb is used. See tmdb_*.go.
	tconfig.DBBackend = string(tmdbBackend)
	log.Infof("using db backend %s", tconfig.DBBackend)

	if localConfig.Genesis != "" {
		tconfig.Genesis = localConfig.Genesis
	}

	if err := tconfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("config is invalid: %w", err)
	}

	logger := NewTenderLogger("tendermint", tconfig.LogLevel)

	// read or create local private validator
	pv, err := NewPrivateValidator(
		localConfig.MinerKey,
		tconfig.PrivValidator.KeyFile(),
		tconfig.PrivValidator.StateFile())
	if err != nil {
		return nil, fmt.Errorf("cannot create validator key and state: (%v)", err)
	}
	pv.Save()

	log.Infof("tendermint validator address: %s", pv.Key.Address)
	//aminoPrivKey, aminoPubKey, err := AminoKeys(pv.Key.PrivKey.(crypto25519.PrivKey))
	//if err != nil {
	//	return nil, err
	//}
	//log.Infof("amino private key: %s", aminoPrivKey)
	//log.Infof("amino public key: %s", aminoPubKey)
	//log.Infof("using keyfile %s", tconfig.PrivValidator.KeyFile())

	// nodekey is used for the p2p transport layer
	nodeKey := new(tmtypes.NodeKey)
	if len(localConfig.NodeKey) > 0 {
		nodeKey, err = NewNodeKey(localConfig.NodeKey, tconfig.NodeKeyFile())
		if err != nil {
			return nil, fmt.Errorf("cannot create node key: %w", err)
		}
	} else {
		if *nodeKey, err = tmtypes.LoadOrGenNodeKey(tconfig.NodeKeyFile()); err != nil {
			return nil, fmt.Errorf("cannot create or load node key: %w", err)
		}
	}
	log.Infof("tendermint p2p node ID: %s", nodeKey.ID)
	log.Debugf("tendermint p2p config: %+v", tconfig.P2P)

	// read or create genesis file
	if tmos.FileExists(tconfig.GenesisFile()) {
		log.Infof("found genesis file %s", tconfig.GenesisFile())
	} else {
		log.Debugf("loaded genesis: %s", string(genesis))
		if err := os.WriteFile(tconfig.GenesisFile(), genesis, 0o600); err != nil {
			return nil, err
		}
		log.Infof("new genesis created, stored at %s", tconfig.GenesisFile())
	}

	if localConfig.TendermintMetrics {
		tconfig.Instrumentation = &tmcfg.InstrumentationConfig{
			Prometheus:           true,
			PrometheusListenAddr: "",
			MaxOpenConnections:   1,
			Namespace:            "tendermint",
		}
	}

	// We need to fetch chain_id in order to make Replay work,
	// since signatures depend on it.
	log.Infof("genesis file at %s", tconfig.GenesisFile())
	type genesisChainID struct {
		ChainID string `json:"chain_id"`
	}
	genesisData, err := os.ReadFile(tconfig.GenesisFile())
	if err != nil {
		return nil, fmt.Errorf("cannot read genesis file: %w", err)
	}
	genesisCID := &genesisChainID{}
	if err := json.Unmarshal(genesisData, genesisCID); err != nil {
		return nil, fmt.Errorf("cannot unmarshal genesis file for fetching chainID")
	}
	log.Infof("found chainID %s", genesisCID.ChainID)
	app.chainID = genesisCID.ChainID

	// create node
	// TO-DO: the last parameter can be used for adding a custom (user provided) genesis file
	service, err := tmnode.New(tconfig, logger, abciclient.NewLocalCreator(app), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new Tendermint node: %w", err)
	}

	return service, nil
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

// AminoPubKey is a helper function that transforms a standard EDDSA pubkey into
// Tendermint like amino format useful for creating genesis files.
func AminoPubKey(pubkey []byte) ([]byte, error) {
	return tmjson.Marshal(pubkey)
}
