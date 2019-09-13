package vochain

import (
	"fmt"
	"os"
	"strings"

	//"time"

	//abci "github.com/tendermint/tendermint/abci/types"

	"github.com/pkg/errors"
	"gitlab.com/vocdoni/go-dvote/config"

	codec "github.com/cosmos/cosmos-sdk/codec"
	//abci "github.com/tendermint/tendermint/abci/types"
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

	//test "gitlab.com/vocdoni/go-dvote/vochain/test"
	vochaintypes "gitlab.com/vocdoni/go-dvote/vochain/types"
)

// Start starts a new vochain validator node
func Start(globalCfg config.VochainCfg, db *dbm.GoLevelDB) (*vochain.BaseApplication, *nm.Node) {
	// PUT ON GATEWAY CONFIG
	/*
		flag.StringVar(&configFile, "config", "/home/jordi/vocdoni/go-dvote/vochain/config/config.toml", "Path to config.toml")
		flag.StringVar(&appdbName, "appdbname", "vochaindb", "Application database name")
		flag.StringVar(&appdbDir, "appdbdir", "/home/jordi/vocdoni/go-dvote/vochain/data", "Path where the application database will be located")
	*/
	// create application db
	vlog.Info("Initializing Vochain")

	// creating new vochain app
	app := vochain.NewBaseApplication(db)
	//flag.Parse()
	vlog.Info("Creating node and application")
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
	tconfig.P2P.PersistentPeers = strings.Trim(strings.Join(localConfig.Peers[:], ","), "[]")
	tconfig.P2P.Seeds = strings.Trim(strings.Join(localConfig.Peers[:], ","), "[]")
	tconfig.P2P.AddrBookStrict = false
	tconfig.P2P.SeedMode = localConfig.SeedMode

	if localConfig.Genesis != "" {
		vlog.Infof("using custom genesis file %s", localConfig.Genesis)
		tconfig.Genesis = localConfig.Genesis
	}
	vlog.Infof("using keyfile %s", tconfig.NodeKeyFile())

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
	pv := privval.LoadOrGenFilePV(
		tconfig.PrivValidatorKeyFile(),
		tconfig.PrivValidatorStateFile(),
	)

	// read or create node key
	nodeKey, err := p2p.LoadOrGenNodeKey(tconfig.NodeKeyFile())
	if err != nil {
		return nil, errors.Wrap(err, "failed to load node's key")
	}

	// read or create genesis file
	genFile := tconfig.GenesisFile()
	if cmn.FileExists(genFile) {
		vlog.Infof("found genesis file %s", genFile)
	} else {
		vlog.Info("creating genesis file")
		genDoc := tmtypes.GenesisDoc{
			ChainID:         "0x1",
			GenesisTime:     tmtime.Now(),
			ConsensusParams: tmtypes.DefaultConsensusParams(),
		}

		// create app state getting validators and oracle keys from eth
		// one oracle needs to exist
		state := &vochaintypes.State{
			Oracles:    getOraclesFromEth(), // plus existing oracle ?
			Validators: getValidatorsFromEth(*pv),
			Processes:  make(map[string]*vochaintypes.Process, 0),
		}

		// set validators from eth smart contract
		genDoc.Validators = state.Validators

		// amino marshall state
		genDoc.AppState = codec.Cdc.MustMarshalJSON(*state)

		// save genesis
		if err := genDoc.SaveAs(genFile); err != nil {
			panic(fmt.Sprintf("cannot load or generate genesis file: %v", err))
		}
		logger.Info("generated genesis file", "path", genFile)
		vlog.Infof("genesis file: %+v", genFile)
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

// temp function
func getValidatorsFromEth(nodeKey privval.FilePV) []tmtypes.GenesisValidator {
	// TODO oracle doing stuff
	// oracle returns a list of validators... then
	list := make([]tmtypes.GenesisValidator, 0)
	list = append(list, tmtypes.GenesisValidator{

		Address: nodeKey.GetPubKey().Address(),
		PubKey:  nodeKey.GetPubKey(),
		Power:   10,
	},
	)
	return list
}

// temp func
func getOraclesFromEth() []string {
	// TODO oracle doing stuff
	// oracle returns a list of trusted oracles
	//"0xF904848ea36c46817096E94f932A9901E377C8a5"
	list := make([]string, 0)
	list = append(list, "0xF904848ea36c46817096E94f932A9901E377C8a5")
	vlog.Infof("Oracles return: %v", list[0])
	return list
}
