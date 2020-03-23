// Package vochain provides all the functions for creating and managing a vocdoni voting blockchain
package vochain

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/config"

	cfg "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	cmn "github.com/tendermint/tendermint/libs/common"
	tlog "github.com/tendermint/tendermint/libs/log"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"

	"gitlab.com/vocdoni/go-dvote/log"
)

var (
	// ProdSeedNodes production vochain network seed nodes
	ProdSeedNodes = []string{"121e65eb5994874d9c05cd8d584a54669d23f294@116.202.8.150:11714"}
	// TestSeedNodes testing vochain network seed nodes
	DevSeedNodes = []string{"7440a5b086e16620ce7b13198479016aa2b07988@116.202.8.150:11715"}
)

// NewVochain starts a node with an ABCI application
func NewVochain(globalCfg *config.VochainCfg, genesis []byte, pv *privval.FilePV) *BaseApplication {
	// creating new vochain app
	app, err := NewBaseApplication(globalCfg.DataDir + "/data")
	if err != nil {
		log.Errorf("cannot init vochain application: %s", err)
	}

	log.Info("creating tendermint node and application")
	go func() {
		app.Node, err = newTendermint(app, globalCfg, genesis, pv)
		if err != nil {
			log.Fatal(err)
		}
		if err := app.Node.Start(); err != nil {
			log.Fatal(err)
		}
	}()
	return app
}

// tenderLogger implements tendermint's Logger interface, with a couple of
// modifications.
//
// First, it routes the logs to go-dvote's logger, so that we don't end up with
// two loggers writing directly to stdout or stderr.
//
// Second, because we generally don't care about tendermint errors such as
// failures to connect to peers, we route all log levels to our debug level.
// They will only surface if dvote's log level is "debug".
type tenderLogger struct {
	keyvals []interface{}
}

var _ tlog.Logger = (*tenderLogger)(nil)

// TODO(mvdan): use zap's WithCallerSkip so that we show the position
// information corresponding to where tenderLogger was called, instead of just
// the pointless positions here.

func (l *tenderLogger) Debug(msg string, keyvals ...interface{}) {
	log.Debugw("[tendermint debug] "+msg, keyvals...)
}

func (l *tenderLogger) Info(msg string, keyvals ...interface{}) {
	log.Debugw("[tendermint info] "+msg, keyvals...)
}

func (l *tenderLogger) Error(msg string, keyvals ...interface{}) {
	log.Debugw("[tendermint error] "+msg, keyvals...)
}

func (l *tenderLogger) With(keyvals ...interface{}) tlog.Logger {
	// Make sure we copy the values, to avoid modifying the parent.
	// TODO(mvdan): use zap's With method directly.
	l2 := &tenderLogger{}
	l2.keyvals = append(l2.keyvals, l.keyvals...)
	l2.keyvals = append(l2.keyvals, keyvals...)
	return l2
}

// we need to set init (first time validators and oracles)
func newTendermint(app *BaseApplication, localConfig *config.VochainCfg, genesis []byte, pv *privval.FilePV) (*nm.Node, error) {
	// create node config
	var err error

	tconfig := cfg.DefaultConfig()
	tconfig.FastSyncMode = true
	tconfig.SetRoot(localConfig.DataDir)
	os.MkdirAll(localConfig.DataDir+"/config", 0755)
	os.MkdirAll(localConfig.DataDir+"/data", 0755)

	tconfig.LogLevel = localConfig.LogLevel
	tconfig.RPC.ListenAddress = "tcp://" + localConfig.RPCListen
	tconfig.P2P.ListenAddress = "tcp://" + localConfig.P2PListen
	tconfig.P2P.ExternalAddress = localConfig.PublicAddr
	log.Infof("announcing external address %s", tconfig.P2P.ExternalAddress)

	if !localConfig.CreateGenesis {
		tconfig.P2P.Seeds = strings.Trim(strings.Join(localConfig.Seeds[:], ","), "[]\"")
		if len(tconfig.P2P.Seeds) < 8 && !localConfig.SeedMode {
			if !localConfig.Dev {
				tconfig.P2P.Seeds = strings.Join(ProdSeedNodes[:], ",")
			} else {
				tconfig.P2P.Seeds = strings.Join(DevSeedNodes[:], ",")
			}
		}
		log.Infof("seed nodes: %s", tconfig.P2P.Seeds)

		if len(localConfig.Peers) > 0 {
			tconfig.P2P.PersistentPeers = strings.Trim(strings.Join(localConfig.Peers[:], ","), "[]\"")
		}
		if len(tconfig.P2P.PersistentPeers) > 0 {
			log.Infof("persistent peers: %s", tconfig.P2P.PersistentPeers)
		}
	}

	tconfig.P2P.AddrBookStrict = false
	tconfig.P2P.SeedMode = localConfig.SeedMode
	tconfig.RPC.CORSAllowedOrigins = []string{"*"}

	tconfig.Consensus.TimeoutProposeDelta = time.Millisecond * 500
	tconfig.Consensus.TimeoutPropose = time.Second * 3
	tconfig.Consensus.TimeoutPrevoteDelta = time.Millisecond * 500
	tconfig.Consensus.TimeoutPrevote = time.Second * 3
	tconfig.Consensus.TimeoutPrecommitDelta = time.Millisecond * 500
	tconfig.Consensus.TimeoutPrecommit = time.Second * 3
	tconfig.Consensus.TimeoutCommit = time.Second * 10

	// tx events
	tconfig.TxIndex.IndexTags = "tx.hash,processCreated.entityId"
	tconfig.TxIndex.IndexAllTags = true

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

	// create logger
	logger := tlog.Logger(&tenderLogger{})

	logger, err = tmflags.ParseLogLevel(tconfig.LogLevel, logger, cfg.DefaultLogLevel())
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}

	// read or create private validator
	if pv == nil {
		pv, err = NewPrivateValidator(localConfig, tconfig)
		if err != nil {
			return nil, fmt.Errorf("cannot create validator key and state: %w", err)
		}
	} else {
		pv.Save()
	}

	log.Infof("using miner key: %s", pv.Key.Address)

	// read or create node key
	var nodeKey *p2p.NodeKey
	if localConfig.KeyFile != "" {
		nodeKey, err = p2p.LoadOrGenNodeKey(localConfig.KeyFile)
		log.Infof("using keyfile %s", localConfig.KeyFile)
	} else {
		nodeKey, err = p2p.LoadOrGenNodeKey(tconfig.NodeKeyFile())
		log.Infof("using keyfile %s", tconfig.NodeKeyFile())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load node's key: %w", err)
	}
	log.Infof("my vochain address: %s", nodeKey.PubKey().Address())
	log.Infof("my vochain ID: %s", nodeKey.ID())

	// read or create genesis file
	if cmn.FileExists(tconfig.Genesis) {
		log.Infof("found genesis file %s", tconfig.Genesis)
	} else {
		log.Debugf("loaded genesis: %s", string(genesis))
		if err := ioutil.WriteFile(tconfig.Genesis, genesis, 0644); err != nil {
			return nil, err
		}
		log.Infof("new genesis created, stored at %s", tconfig.Genesis)
	}

	// create node
	node, err := nm.NewNode(
		tconfig,
		pv,      // the node val
		nodeKey, // node val key
		proxy.NewLocalClientCreator(app),
		// Note we use proxy.NewLocalClientCreator here to create a local client instead of one communicating through a socket or gRPC.
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
