package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/pkg/errors"
	"github.com/spf13/viper"

	cfg "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	cmn "github.com/tendermint/tendermint/libs/common"
	tlog "github.com/tendermint/tendermint/libs/log"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	dbm "github.com/tendermint/tm-db"
	vlog "gitlab.com/vocdoni/go-dvote/log"
	vochain "gitlab.com/vocdoni/go-dvote/vochain/app"
	testtypes "gitlab.com/vocdoni/go-dvote/vochain/test"
)

var configFile string
var appdbName string
var appdbDir string
var appName string

func init() {
	flag.StringVar(&configFile, "config", "/home/jordi/vocdoni/go-dvote/vochain/config/config.toml", "Path to config.toml")
	flag.StringVar(&appdbName, "appdbname", "vochaindb", "Application database name")
	flag.StringVar(&appdbDir, "appdbdir", "/home/jordi/vocdoni/go-dvote/vochain/data/appdb", "Path where the application database will be located")
}

func main() {
	db, err := dbm.NewGoLevelDBWithOpts(appdbName, appdbDir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open badger db: %v", err)
		os.Exit(1)
	}
	defer db.Close()
	vlog.Info("NewBaseApp")

	app := vochain.NewBaseApplication(db)

	flag.Parse()
	vlog.Info("newTendermint")
	node, err := newTendermint(*app, configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(2)
	}
	vlog.Info("NodeStart")
	node.Start()
	defer func() {
		node.Stop()
		node.Wait()
	}()

	n := 0
	for n < 10 {
		n++
		time.Sleep(3 * time.Second)
		//txbytes := []byte(`{"method": "voteTx", "args": {"nullifier": "0", "payload": "0", "censusProof": "0"}}`)
		txbytes := []byte(`{"method": "newProcessTx", "args": { "entityId": "0", "entityResolver": "0", "metadataHash": "0", "mkRoot": "0", "initBlock": "0", "numberOfBlocks": "0", "encryptionKeys": "0"  }}`)
		req := abci.RequestCheckTx{Tx: txbytes}
		go vlog.Infof("%+v", app.CheckTx(req))
		req2 := abci.RequestDeliverTx{Tx: txbytes}
		go vlog.Infof("%+v", app.DeliverTx(req2))
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	os.Exit(0)

}

func newTendermint(app vochain.BaseApplication, configFile string) (*nm.Node, error) {
	// read config
	config := cfg.DefaultConfig()
	config.RootDir = filepath.Dir(filepath.Dir(configFile))
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, errors.Wrap(err, "viper failed to read config file")
	}
	if err := viper.Unmarshal(config); err != nil {
		return nil, errors.Wrap(err, "viper failed to unmarshal config")
	}
	if err := config.ValidateBasic(); err != nil {
		return nil, errors.Wrap(err, "config is invalid")
	}

	newpath := filepath.Join(config.RootDir, "data")
	os.MkdirAll(newpath, os.ModePerm)

	// create logger
	logger := tlog.NewTMLogger(tlog.NewSyncWriter(os.Stdout))
	var err error
	logger, err = tmflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse log level")
	}

	// read or create private validator
	pv := privval.LoadOrGenFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)

	// read or create node key
	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		return nil, errors.Wrap(err, "failed to load node's key")
	}

	// read genesis file or create it with previously generated keys
	//appState := fmt.Sprintf("{censusManagers:['%s','%s','%s']}", testtypes.ValidatorsPubK1, testtypes.ValidatorsPubK2, testtypes.ValidatorsPubK3)
	//appStateJSON, err := codec.Cdc.MarshalJSON(appState)

	genFile := config.GenesisFile()
	if cmn.FileExists(genFile) {
		vlog.Info("Found genesis file", "path", genFile)
	} else {
		vlog.Info("Creating genesis file")
		genDoc := tmtypes.GenesisDoc{
			ChainID:         testtypes.ChainID,
			GenesisTime:     tmtime.Now(),
			ConsensusParams: tmtypes.DefaultConsensusParams(),
		}
		key := pv.GetPubKey()
		genDoc.Validators = []tmtypes.GenesisValidator{
			{
				Address: key.Address(),
				PubKey:  key,
				Power:   10,
			},
		}
		//genDoc.AppState = appStateJSON

		if err := genDoc.SaveAs(genFile); err != nil {
			panic(fmt.Sprintf("Cannot load or generate genesis file: %v", err))
		}
		logger.Info("Generated genesis file", "path", genFile)
		vlog.Info("genesis file: %+v", genFile)
	}

	// create node
	node, err := nm.NewNode(
		config,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(&app), // Note we use proxy.NewLocalClientCreator here to create a local client instead of one communicating through a socket or gRPC.
		nm.DefaultGenesisDocProviderFunc(config),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(config.Instrumentation),
		logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new Tendermint node")
	}

	return node, nil
}
