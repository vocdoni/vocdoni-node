// Package vochain provides all the functions for creating and managing a vocdoni voting blockchain
package vochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	vocdoniGenesis "go.vocdoni.io/dvote/vochain/genesis"

	tmcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/proxy"

	tmlog "github.com/cometbft/cometbft/libs/log"
	tmos "github.com/cometbft/cometbft/libs/os"
	tmnode "github.com/cometbft/cometbft/node"
	"go.vocdoni.io/dvote/log"
)

// NewVochain starts a node with an ABCI application
func NewVochain(vochaincfg *config.VochainCfg, genesis []byte) *BaseApplication {
	// creating new vochain app
	c := *vochaincfg
	c.DataDir = filepath.Join(vochaincfg.DataDir, "data")
	app, err := NewBaseApplication(&c)
	if err != nil {
		log.Fatalf("cannot initialize vochain application: %s", err)
	}
	log.Info("creating tendermint node and application")
	err = app.SetNode(vochaincfg, genesis)
	if err != nil {
		log.Fatal(err)
	}
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
	keyvals  []any
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

func (l *TenderLogger) Debug(msg string, keyvals ...any) {
	if l.logLevel == 0 {
		log.Logger().Debug().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

func (l *TenderLogger) Info(msg string, keyvals ...any) {
	if l.logLevel <= 1 {
		log.Logger().Info().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

func (l *TenderLogger) Error(msg string, keyvals ...any) {
	if l.logLevel <= 2 {
		log.Logger().Error().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

func (l *TenderLogger) With(keyvals ...any) tmlog.Logger {
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
	localConfig *config.VochainCfg, genesis []byte) (*tmnode.Node, error) {
	var err error

	tconfig := tmcfg.DefaultConfig()
	tconfig.SetRoot(localConfig.DataDir)
	if err := os.MkdirAll(filepath.Join(localConfig.DataDir, "config"), 0750); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(localConfig.DataDir, "data"), 0750); err != nil {
		log.Fatal(err)
	}

	tconfig.LogLevel = localConfig.LogLevel
	if tconfig.LogLevel == "none" {
		tconfig.LogLevel = "disabled"
	}
	tconfig.P2P.ExternalAddress = localConfig.PublicAddr
	if localConfig.Dev {
		tconfig.P2P.AllowDuplicateIP = true
		tconfig.P2P.AddrBookStrict = false
	}
	tconfig.P2P.Seeds = strings.Trim(strings.Join(localConfig.Seeds, ","), "[]\"")
	if _, ok := vocdoniGenesis.Genesis[localConfig.Chain]; len(tconfig.P2P.Seeds) < 8 &&
		!localConfig.IsSeedNode && ok {
		tconfig.P2P.Seeds = strings.Join(vocdoniGenesis.Genesis[localConfig.Chain].SeedNodes, ",")
	}
	if len(tconfig.P2P.Seeds) > 0 {
		log.Infof("seed nodes: %s", tconfig.P2P.Seeds)
	}

	if len(localConfig.Peers) > 0 {
		tconfig.P2P.PersistentPeers = strings.Trim(strings.Join(localConfig.Peers, ","), "[]\"")
	}
	if len(tconfig.P2P.PersistentPeers) > 0 {
		log.Infof("persistent peers: %s", tconfig.P2P.PersistentPeers)
	}

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

	// if seed node
	if localConfig.IsSeedNode {
		tconfig.P2P.SeedMode = true
	}

	log.Infow("consensus time target",
		"precommit", tconfig.Consensus.TimeoutPrecommit.Seconds(),
		"propose", tconfig.Consensus.TimeoutPropose.Seconds(),
		"prevote", tconfig.Consensus.TimeoutPrevote.Seconds(),
		"commit", tconfig.Consensus.TimeoutCommit.Seconds(),
		"block", blockTime)

	// disable transaction indexer (we don't use it)
	tconfig.TxIndex = &tmcfg.TxIndexConfig{Indexer: "null"}
	// mempool config
	tconfig.Mempool.Size = localConfig.MempoolSize
	tconfig.Mempool.Recheck = true
	tconfig.Mempool.KeepInvalidTxsInCache = true
	tconfig.Mempool.MaxTxBytes = 1024 * 100 // 100 KiB
	tconfig.Mempool.MaxTxsBytes = int64(tconfig.Mempool.Size * tconfig.Mempool.MaxTxBytes)
	tconfig.Mempool.CacheSize = 100000
	tconfig.Mempool.Broadcast = true

	log.Debugf("mempool config: %+v", tconfig.Mempool)
	// tmdbBackend defaults to goleveldb, but switches to cleveldb if
	// -tags=cleveldb is used. See tmdb_*.go.
	tconfig.DBBackend = string(tmdbBackend)
	if localConfig.Genesis != "" {
		tconfig.Genesis = localConfig.Genesis
	}

	if err := tconfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("config is invalid: %w", err)
	}

	logger := NewTenderLogger("comet", tconfig.LogLevel)

	// read or create local private validator
	pv, err := NewPrivateValidator(
		localConfig.MinerKey,
		tconfig.PrivValidatorKeyFile(),
		tconfig.PrivValidatorStateFile())
	if err != nil {
		return nil, fmt.Errorf("cannot create validator key and state: (%v)", err)
	}
	pv.Save()
	if pv.Key.PrivKey.Bytes() != nil {
		signer := ethereum.SignKeys{}
		if err := signer.AddHexKey(hex.EncodeToString(pv.Key.PrivKey.Bytes())); err != nil {
			return nil, fmt.Errorf("cannot add private validator key: %w", err)
		}
		app.NodeAddress, err = NodePvKeyToAddress(pv.Key.PubKey)
		if err != nil {
			return nil, fmt.Errorf("cannot create node address from pubkey: %w", err)
		}
	}

	// nodekey is used for the p2p transport layer
	nodeKey := new(p2p.NodeKey)
	if len(localConfig.NodeKey) > 0 {
		nodeKey, err = NewNodeKey(localConfig.NodeKey, tconfig.NodeKeyFile())
		if err != nil {
			return nil, fmt.Errorf("cannot create node key: %w", err)
		}
	} else {
		if nodeKey, err = p2p.LoadOrGenNodeKey(tconfig.NodeKeyFile()); err != nil {
			return nil, fmt.Errorf("cannot create or load node key: %w", err)
		}
	}
	log.Infow("vochain initialized",
		"db-backend", tconfig.DBBackend,
		"publicKey", hex.EncodeToString(pv.Key.PubKey.Bytes()),
		"accountAddr", app.NodeAddress,
		"validatorAddr", pv.Key.PubKey.Address(),
		"external-address", tconfig.P2P.ExternalAddress,
		"nodeId", nodeKey.ID(),
		"seed", tconfig.P2P.SeedMode)

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
			MaxOpenConnections:   2,
			Namespace:            "comet",
		}
	}

	// We need to fetch chain_id in order to make Replay work,
	// since signatures depend on it.
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
	log.Infow("genesis file", "genesis", tconfig.GenesisFile(), "chainID", genesisCID.ChainID)
	app.SetChainID(genesisCID.ChainID)

	// assign the default tendermint methods
	app.SetDefaultMethods()
	node, err := tmnode.NewNode(tconfig,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(app),
		tmnode.DefaultGenesisDocProviderFunc(tconfig),
		tmcfg.DefaultDBProvider,
		tmnode.DefaultMetricsProvider(tconfig.Instrumentation),
		logger,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create new Tendermint node: %w", err)
	}

	return node, nil
}
