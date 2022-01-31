// Package vochain provides all the functions for creating and managing a vocdoni voting blockchain
package vochain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.vocdoni.io/dvote/config"

	abciclient "github.com/tendermint/tendermint/abci/client"
	tmcfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/privval"

	tmjson "github.com/tendermint/tendermint/libs/json"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/libs/service"
	tmnode "github.com/tendermint/tendermint/node"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/dvote/log"
)

const downloadZkVKsTimeout = 1 * time.Minute

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
	// get the zk Circuits VerificationKey files
	ctx, cancel := context.WithTimeout(context.Background(), downloadZkVKsTimeout)
	defer cancel()
	if err := app.LoadZkVKs(ctx); err != nil {
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
func newTendermint(app *BaseApplication,
	localConfig *config.VochainCfg, genesis []byte) (service.Service, error) {
	// create node config
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
	if !localConfig.CreateGenesis {
		tconfig.P2P.BootstrapPeers = strings.Trim(strings.Join(localConfig.Seeds, ","), "[]\"")
		if _, ok := Genesis[localConfig.Chain]; len(tconfig.P2P.BootstrapPeers) < 8 &&
			!localConfig.IsSeedNode && ok {
			tconfig.P2P.BootstrapPeers = strings.Join(Genesis[localConfig.Chain].SeedNodes, ",")
		}
		if len(tconfig.P2P.BootstrapPeers) > 0 {
			log.Infof("seed nodes: %s", tconfig.P2P.BootstrapPeers)
		}
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
	// -tags=cleveldb is used, and switches to badgerdb if -tags=badger is
	// used. See tmdb_*.go.
	// TODO: probably switch to just badger after some benchmarking.
	tconfig.DBBackend = string(tmdbBackend)
	log.Infof("using db backend %s", tconfig.DBBackend)

	if localConfig.Genesis != "" && !localConfig.CreateGenesis {
		if isAbs := strings.HasPrefix(localConfig.Genesis, "/"); !isAbs {
			dir, err := os.Getwd()
			if err != nil {
				log.Fatal(err)
			}
			tconfig.Genesis = filepath.Join(dir, localConfig.Genesis)

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
	if tconfig.LogLevel == "none" {
		tconfig.LogLevel = "error"
	}
	logger, err := tmlog.NewDefaultLogger("plain", tconfig.LogLevel, false)
	if err != nil {
		log.Errorf("failed to parse log level: %v", err)
	}

	// pv will contain a local private validator,
	// if PrivValidatorListenAddr is defined, there's no need to initialize it since nm.NewNode() will ignore it
	var pv *privval.FilePV

	// TO-DO: check if current tendermint version supports hardware wallets
	// if PrivValidatorListenAddr is defined, use a remote private validator, like TMKMS which allows signing with Ledger
	// else, use a local private validator
	//	if len(localConfig.PrivValidatorListenAddr) > 0 {
	//		log.Info("PrivValidatorListenAddr defined, Tendermint will wait for a remote private validator connection")
	//		tconfig.PrivValidatorListenAddr = localConfig.PrivValidatorListenAddr
	//	} else {

	// read or create local private validator
	pv, err = NewPrivateValidator(localConfig.MinerKey, tconfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create validator key and state: (%v)", err)
	}
	pv.Save()

	log.Infof("tendermint validator private key %x", pv.Key.PrivKey)
	log.Infof("tendermint validator address: %s", pv.Key.Address)
	aminoPrivKey, aminoPubKey, err := AminoKeys(pv.Key.PrivKey.(crypto25519.PrivKey))
	if err != nil {
		return nil, err
	}
	log.Infof("amino private key: %s", aminoPrivKey)
	log.Infof("amino public key: %s", aminoPubKey)
	log.Infof("using keyfile %s", tconfig.PrivValidator.KeyFile())

	// nodekey is used for the p2p transport layer
	nodeKey := new(tmtypes.NodeKey)
	if len(localConfig.NodeKey) > 0 {
		nodeKey, err = NewNodeKey(localConfig.NodeKey, tconfig)
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
	if tmos.FileExists(tconfig.Genesis) {
		log.Infof("found genesis file %s", tconfig.Genesis)
	} else {
		log.Debugf("loaded genesis: %s", string(genesis))
		if err := os.WriteFile(tconfig.Genesis, genesis, 0o600); err != nil {
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

	// We need to fetch chain_id in order to make Replay work,
	// since signatures depend on it.
	log.Infof("genesis file at %s", tconfig.GenesisFile())
	type genesisChainID struct {
		ChainID string `json:"chain_id"`
	}
	genesisData, err := os.ReadFile(tconfig.Genesis)
	if err != nil {
		return nil, fmt.Errorf("cannot read genesis file: %w", err)
	}
	genesisCID := &genesisChainID{}
	if err := json.Unmarshal(genesisData, genesisCID); err != nil {
		return nil, fmt.Errorf("cannot unmarshal genesis file for fetching chainID")
	}
	log.Infof("found chainID %s", genesisCID.ChainID)
	app.chainId = genesisCID.ChainID

	// create node
	// TO-DO: the last parameter can be used for adding a custom (user provided) genesis file
	service, err := tmnode.New(tconfig, logger, abciclient.NewLocalCreator(app), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new Tendermint node: %w", err)
	}

	// Disabled until hardware wallet support is back
	// If using a remote private validator, the pubkeys are only available after nm.NewNode()
	// In that case, print them now, for debugging purposes
	/*	if len(localConfig.PrivValidatorListenAddr) > 0 {
			pv := node.PrivValidator()
			pubKey, err := pv.GetPubKey()
			if err != nil {
				log.Warnf("failed to get pubkey from private validator: %v", err)
			}
			log.Infof("tendermint address: %v", pubKey.Address())

			aminoPubKey, err := AminoPubKey(pubKey.Bytes())
			if err != nil {
				log.Warnf("failed to get amino public key: %v", err)
			}
			log.Infof("amino public key: %s", aminoPubKey)
		}
	*/
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
