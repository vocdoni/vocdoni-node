package mock

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tlog "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proxy"
	"gitlab.com/vocdoni/go-dvote/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	pvm "github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	cmn "github.com/tendermint/tm-cmn/common"
)

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

func StartInProcess(ctx *Context, dbName string, dbDir string) (*node.Node, error) {
	cfg := ctx.Config

	app := NewCounterApplication(false, dbName, dbDir)

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
	TrapSignal(func() {
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
