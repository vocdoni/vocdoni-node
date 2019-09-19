package vochain

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/vocdoni/go-dvote/config"

	codec "github.com/cosmos/cosmos-sdk/codec"
	cfg "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	cmn "github.com/tendermint/tendermint/libs/common"
	tlog "github.com/tendermint/tendermint/libs/log"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	privval "github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	dbm "github.com/tendermint/tm-db"
	vlog "gitlab.com/vocdoni/go-dvote/log"
	vochain "gitlab.com/vocdoni/go-dvote/vochain/app"

	vochaintypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

// testing purposes until genesis
const testOracleAddress = "0xF904848ea36c46817096E94f932A9901E377C8a5"

// List of default Vocdoni seed nodes
var DefaultSeedNodes = []string{"121e65eb5994874d9c05cd8d584a54669d23f294@116.202.8.150:11714"}

// Start starts a new vochain validator node
func Start(globalCfg config.VochainCfg, db *dbm.GoLevelDB) (*vochain.BaseApplication, *nm.Node) {

	// create application db
	vlog.Info("initializing Vochain")

	// creating new vochain app
	app := vochain.NewBaseApplication(db)
	//flag.Parse()
	vlog.Info("creating node and application")
	node, err := newTendermint(app, globalCfg)
	if err != nil {
		vlog.Info(err)
		return app, node
	}
	node.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(2)
	}
	return app, node
}

//NewGenesis creates a new genesis file and saves it to tconfig.Genesis path
func NewGenesis(tconfig *cfg.Config, pv *privval.FilePV) error {
	vlog.Info("creating genesis file")
	genDoc := tmtypes.GenesisDoc{
		ChainID:         "0x1",
		GenesisTime:     tmtime.Now(),
		ConsensusParams: tmtypes.DefaultConsensusParams(),
	}

	list := make([]tmtypes.GenesisValidator, 0)
	list = append(list, tmtypes.GenesisValidator{
		Address: pv.GetPubKey().Address(),
		PubKey:  pv.GetPubKey(),
		Power:   10,
	})

	// create app state getting validators and oracle keys from eth
	// one oracle needs to exist
	state := &vochaintypes.State{
		Oracles:    []string{testOracleAddress},
		Validators: list,
		Processes:  make(map[string]*vochaintypes.Process, 0),
	}

	// set validators from eth smart contract
	genDoc.Validators = state.Validators

	// amino marshall state
	genDoc.AppState = codec.Cdc.MustMarshalJSON(*state)

	// save genesis
	if err := genDoc.SaveAs(tconfig.Genesis); err != nil {
		return err
	}
	vlog.Infof("genesis file: %+v", tconfig.Genesis)
	return nil
}

// we need to set init (first time validators and oracles)
func newTendermint(app *vochain.BaseApplication, localConfig config.VochainCfg) (*nm.Node, error) {
	// create node config
	var err error

	tconfig := cfg.DefaultConfig()
	tconfig.SetRoot(localConfig.DataDir)
	os.MkdirAll(localConfig.DataDir+"/config", 0755)
	os.MkdirAll(localConfig.DataDir+"/data", 0755)

	tconfig.LogLevel = localConfig.LogLevel
	tconfig.RPC.ListenAddress = "tcp://" + localConfig.RpcListen
	tconfig.P2P.ListenAddress = localConfig.P2pListen
	tconfig.P2P.ExternalAddress = localConfig.PublicAddr
	vlog.Infof("announcing external address %s", tconfig.P2P.ExternalAddress)

	if len(localConfig.Seeds) == 0 && !localConfig.SeedMode {
		tconfig.P2P.Seeds = strings.Join(DefaultSeedNodes[:], ",")
	} else {
		tconfig.P2P.Seeds = strings.Trim(strings.Join(localConfig.Seeds[:], ","), "[]")
	}
	vlog.Infof("seed nodes: %s", tconfig.P2P.Seeds)

	if len(localConfig.Peers) > 0 {
		tconfig.P2P.PersistentPeers = strings.Trim(strings.Join(localConfig.Peers[:], ","), "[]")
	}
	vlog.Infof("persistent peers: %s", tconfig.P2P.PersistentPeers)

	tconfig.P2P.AddrBookStrict = false
	tconfig.P2P.SeedMode = localConfig.SeedMode

	if localConfig.Genesis != "" {
		if isAbs := strings.HasPrefix(localConfig.Genesis, "/"); !isAbs {
			dir, err := os.Getwd()
			if err != nil {
				vlog.Fatal(err)
			}
			tconfig.Genesis = dir + "/" + localConfig.Genesis

		} else {
			tconfig.Genesis = localConfig.Genesis
		}
	} else {
		tconfig.Genesis = tconfig.GenesisFile()
	}

	if err := tconfig.ValidateBasic(); err != nil {
		return nil, errors.Wrap(err, "config is invalid")
	}

	// create logger
	logger := tlog.NewTMLogger(tlog.NewSyncWriter(os.Stdout))

	//config.LogLevel = "none"
	logger, err = tmflags.ParseLogLevel(tconfig.LogLevel, logger, cfg.DefaultLogLevel())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse log level")
	}

	// read or create private validator
	var minerKeyFile string
	if localConfig.MinerKeyFile == "" {
		minerKeyFile = tconfig.PrivValidatorKeyFile()
	} else {
		if isAbs := strings.HasPrefix(localConfig.MinerKeyFile, "/"); !isAbs {
			dir, err := os.Getwd()
			if err != nil {
				vlog.Fatal(err)
			}
			minerKeyFile = dir + "/" + localConfig.MinerKeyFile
		} else {
			minerKeyFile = localConfig.MinerKeyFile
		}
		if !cmn.FileExists(tconfig.PrivValidatorKeyFile()) {
			filePV := privval.LoadFilePVEmptyState(minerKeyFile, tconfig.PrivValidatorStateFile())
			filePV.Save()
		}
	}

	vlog.Infof("using miner key file %s", minerKeyFile)
	pv := privval.LoadOrGenFilePV(
		minerKeyFile,
		tconfig.PrivValidatorStateFile(),
	)

	// read or create node key
	var nodeKey *p2p.NodeKey
	if localConfig.KeyFile != "" {
		nodeKey, err = p2p.LoadOrGenNodeKey(localConfig.KeyFile)
		vlog.Infof("using keyfile %s", localConfig.KeyFile)
	} else {
		nodeKey, err = p2p.LoadOrGenNodeKey(tconfig.NodeKeyFile())
		vlog.Infof("using keyfile %s", tconfig.NodeKeyFile())
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to load node's key")
	}
	vlog.Infof("my vochain address: %s", nodeKey.PubKey().Address())
	vlog.Infof("my vochain ID: %s", nodeKey.ID())

	// read or create genesis file
	if cmn.FileExists(tconfig.Genesis) {
		vlog.Infof("found genesis file %s", tconfig.Genesis)
	} else {
		err := ioutil.WriteFile(tconfig.Genesis, []byte(TestnetGenesis1), 0644)
		if err != nil {
			vlog.Warn(err)
		} else {
			vlog.Infof("new testnet genesis created, stored at %s", tconfig.Genesis)
		}
	}

	// create node
	node, err := nm.NewNode(
		tconfig,
		pv,                               // the node val
		nodeKey,                          // node val key
		proxy.NewLocalClientCreator(app), // Note we use proxy.NewLocalClientCreator here to create a local client instead of one communicating through a socket or gRPC.
		nm.DefaultGenesisDocProviderFunc(tconfig),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(tconfig.Instrumentation),
		logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new Tendermint node")
	}

	return node, nil
}
