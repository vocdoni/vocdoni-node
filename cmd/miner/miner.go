package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func newConfig() (config.VochainCfg, error) {
	var globalCfg config.VochainCfg

	// setup flags
	home, err := os.UserHomeDir()
	if err != nil {
		return globalCfg, err
	}
	userDir := home + "/.dvote"

	path := flag.String("configFilePath", userDir+"/vochain.yaml", "vochain config file path")
	dataDir := flag.String("dataDir", userDir+"/vochain/data", "sets the path indicating where to store the vochain related data")
	flag.String("p2pListen", "0.0.0.0:26656", "p2p host and port to listent")
	flag.String("rpcListen", "127.0.0.1:26657", "rpc host and port to listent")
	flag.String("genesis", "", "use alternative geneiss file")
	flag.String("keyFile", "", "user alternative key file")
	flag.String("minerKeyFile", "", "user alternative key file for mining")
	flag.Bool("seedMode", false, "act as a seed node")
	flag.StringArray("peers", []string{}, "coma separated list of p2p peers")
	flag.StringArray("seeds", []string{}, "coma separated list of p2p seed nodes")
	flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	flag.Parse()

	viper := viper.New()
	viper.SetDefault("configFilePath", *dataDir+"/vochain.yaml")
	viper.SetDefault("dataDir", *dataDir+"/vochain/data")
	viper.SetDefault("logLevel", "warn")
	viper.SetDefault("keyFile", "")
	viper.SetDefault("minerKeyFile", "")
	viper.SetDefault("p2pListen", "0.0.0.0:26656")
	viper.SetDefault("rpcListen", "0.0.0.0:26657")
	viper.SetDefault("seedMode", false)
	viper.SetDefault("genesis", "")
	viper.SetDefault("peers", []string{})
	viper.SetDefault("seeds", []string{})

	viper.SetConfigType("yaml")

	if err = viper.SafeWriteConfigAs(*path); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(userDir, os.ModePerm)
			if err != nil {
				return globalCfg, err
			}
			err = viper.WriteConfigAs(*path)
			if err != nil {
				return globalCfg, err
			}
		}
	}

	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("configFilePath", flag.Lookup("configFilePath"))
	viper.BindPFlag("p2pListen", flag.Lookup("p2pListen"))
	viper.BindPFlag("rpcListen", flag.Lookup("rpcListen"))
	viper.BindPFlag("keyFile", flag.Lookup("keyFile"))
	viper.BindPFlag("minerKeyFile", flag.Lookup("minerKeyFile"))
	viper.BindPFlag("seedMode", flag.Lookup("seedMode"))
	viper.BindPFlag("peers", flag.Lookup("peers"))
	viper.BindPFlag("seeds", flag.Lookup("seeds"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("genesis", flag.Lookup("genesis"))

	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

func main() {
	globalCfg, err := newConfig()
	log.InitLogger(globalCfg.LogLevel, "stdout")
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}
	log.Info("starting miner")

	if len(globalCfg.PublicAddr) == 0 {
		ip, err := util.PublicIP()
		if err != nil {
			log.Warn(err)
		} else {
			addrport := strings.Split(globalCfg.P2pListen, ":")
			if len(addrport) > 0 {
				globalCfg.PublicAddr = fmt.Sprintf("%s:%s", ip, addrport[len(addrport)-1])
				log.Infof("public IP address: %s", globalCfg.PublicAddr)
			}
		}
	}

	// node + app layer
	log.Debugf("initializing vochain with tendermint config %s", globalCfg.TendermintConfig)
	vnode := vochain.NewVochain(globalCfg)
	defer func() {
		vnode.Node.Stop()
		vnode.Node.Wait()
	}()

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	os.Exit(0)
}
