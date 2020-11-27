// Package vochain provides all the functions for creating and managing a vocdoni voting blockchain
package vochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/util"

	amino "github.com/tendermint/go-amino"
	tmcfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	cryptoamino "github.com/tendermint/tendermint/crypto/encoding/amino"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"

	"gitlab.com/vocdoni/go-dvote/log"
)

var (
	// ProdSeedNodes production vochain network seed nodes
	ProdSeedNodes = []string{"121e65eb5994874d9c05cd8d584a54669d23f294@seed.vocdoni.net:26656"}
	// DevSeedNodes testing vochain network seed nodes
	DevSeedNodes = []string{"7440a5b086e16620ce7b13198479016aa2b07988@seed.dev.vocdoni.net:26656"}
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

var _ tmlog.Logger = (*tenderLogger)(nil)

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

func (l *tenderLogger) With(keyvals ...interface{}) tmlog.Logger {
	// Make sure we copy the values, to avoid modifying the parent.
	// TODO(mvdan): use zap's With method directly.
	l2 := &tenderLogger{}
	l2.keyvals = append(l2.keyvals, l.keyvals...)
	l2.keyvals = append(l2.keyvals, keyvals...)
	return l2
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
	}
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
	tconfig.P2P.SeedMode = localConfig.SeedMode
	tconfig.RPC.CORSAllowedOrigins = []string{"*"}

	// consensus config
	tconfig.Consensus.TimeoutProposeDelta = time.Millisecond * 1
	tconfig.Consensus.TimeoutPropose = time.Millisecond * 8000
	tconfig.Consensus.TimeoutPrevoteDelta = time.Millisecond * 1
	tconfig.Consensus.TimeoutPrevote = time.Millisecond * 1000
	tconfig.Consensus.TimeoutPrecommitDelta = time.Millisecond * 1
	tconfig.Consensus.TimeoutPrecommit = time.Second * 1
	tconfig.Consensus.TimeoutCommit = time.Second * 10

	// mempool config
	tconfig.Mempool.Size = localConfig.MempoolSize
	tconfig.Mempool.Recheck = false

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
	logger := tmlog.Logger(&tenderLogger{})

	logger, err = tmflags.ParseLogLevel(tconfig.LogLevel, logger, tmcfg.DefaultLogLevel())
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}

	// read or create private validator
	pv, err := NewPrivateValidator(localConfig.MinerKey, tconfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create validator key and state: (%s)", err)
	}
	pv.Save()

	log.Infof("tendermint private key 0x%x", pv.Key.PrivKey)
	log.Infof("tendermint address: %s", pv.Key.Address)
	aminoPrivKey, aminoPubKey, err := HexKeyToAmino(fmt.Sprintf("%x", pv.Key.PrivKey))
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
			return nil, err
		}
	} else {
		if nodeKey, err = p2p.LoadOrGenNodeKey(tconfig.NodeKeyFile()); err != nil {
			return nil, err
		}
	}
	log.Infof("tendermint p2p node ID: %s", nodeKey.ID())

	// read or create genesis file
	if tmos.FileExists(tconfig.Genesis) {
		log.Infof("found genesis file %s", tconfig.Genesis)
	} else {
		log.Debugf("loaded genesis: %s", string(genesis))
		if err := ioutil.WriteFile(tconfig.Genesis, genesis, 0644); err != nil {
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

// HexKeyToAmino is a helper function that transforms a standard EDDSA hex string key into Tendermint like amino format
// usefull for creating genesis files
func HexKeyToAmino(hexKey string) (private, public string, err error) {
	// TO-DO find a better way to to this. Probably amino provides some helpers

	// Needed for recovering the tendermint compatible amino private key format
	type aminoKey struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	type aminoKeyFile struct {
		Privkey aminoKey `json:"priv_key"`
		PubKey  aminoKey `json:"pub_key"`
	}

	key, err := hex.DecodeString(util.TrimHex(hexKey))
	if err != nil {
		return "", "", err
	}

	cdc := amino.NewCodec()
	cryptoamino.RegisterAmino(cdc)

	var pv privval.FilePVKey
	var privKey crypto25519.PrivKeyEd25519

	if n := copy(privKey[:], key[:]); n != 64 {
		return "", "", fmt.Errorf("incorrect private key lenght (got %d, need 64)", n)
	}

	pv.Address = privKey.PubKey().Address()
	pv.PrivKey = privKey
	pv.PubKey = privKey.PubKey()

	jsonBytes, err := cdc.MarshalJSON(pv)
	if err != nil {
		return "", "", fmt.Errorf("cannot encode key to amino: (%s)", err)
	}
	var aminokf aminoKeyFile
	if err := json.Unmarshal(jsonBytes, &aminokf); err != nil {
		return "", "", err
	}
	return aminokf.Privkey.Value, aminokf.PubKey.Value, nil
}
