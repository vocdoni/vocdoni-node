package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func newConfig() (*config.VochainCfg, config.Error) {
	var err error
	var cfgError config.Error
	// create base config
	globalCfg := new(config.VochainCfg)
	// get current user home dir
	home, err := os.UserHomeDir()
	if err != nil {
		cfgError = config.Error{
			Critical: true,
			Message:  fmt.Sprintf("cannot get user home directory with error: %s", err),
		}
		return nil, cfgError
	}

	// CLI flags will be used if something fails from this point
	// CLI flags have preference over the config file

	// creating flags
	flag.StringVar(&globalCfg.DataDir, "dataDir", home+"/.dvote", "directory where data is stored")
	flag.BoolVar(&globalCfg.Dev, "dev", false, "run and connect to the development network")
	globalCfg.P2PListen = *flag.String("p2pListen", "0.0.0.0:26656", "p2p host and port to listen")
	globalCfg.RPCListen = *flag.String("rpcListen", "0.0.0.0:26657", "rpc host and port to listen")
	globalCfg.Genesis = *flag.String("genesis", "", "use alternative genesis file")
	globalCfg.MinerKey = *flag.String("minerKey", "", "user alternative node key file for mining")
	globalCfg.SeedMode = *flag.Bool("seedMode", false, "act as a seed node")
	globalCfg.Peers = *flag.StringArray("peers", []string{}, "coma separated list of p2p peers")
	globalCfg.Seeds = *flag.StringArray("seeds", []string{}, "coma separated list of p2p seed nodes")
	globalCfg.LogLevel = *flag.String("logLevel", "info", "Log level (debug, info, warn, error, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	globalCfg.PublicAddr = *flag.String("publicAddr", "", "IP address where the node will be exposed, guessed automatically if empty")
	globalCfg.SaveConfig = *flag.Bool("saveConfig", false, "overwrites an existing config file with the CLI provided flags")

	// parse flags
	flag.Parse()

	if globalCfg.Dev {
		globalCfg.DataDir += "/dev"
	}

	// setting up viper
	viper := viper.New()
	viper.AddConfigPath(globalCfg.DataDir)
	viper.SetConfigName("vochain-miner")
	viper.SetConfigType("yml")

	// binding flags to viper
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("p2pListen", flag.Lookup("p2pListen"))
	viper.BindPFlag("rpcListen", flag.Lookup("rpcListen"))
	viper.BindPFlag("minerKey", flag.Lookup("minerKey"))
	viper.BindPFlag("seedMode", flag.Lookup("seedMode"))
	viper.BindPFlag("peers", flag.Lookup("peers"))
	viper.BindPFlag("seeds", flag.Lookup("seeds"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("genesis", flag.Lookup("genesis"))
	viper.BindPFlag("publicAddr", flag.Lookup("publicAddr"))
	viper.BindPFlag("saveConfig", flag.Lookup("saveConfig"))
	viper.BindPFlag("dev", flag.Lookup("dev"))

	// check if config file exists
	_, err = os.Stat(globalCfg.DataDir + "/vochain-miner.yml")
	if os.IsNotExist(err) {
		cfgError = config.Error{
			Message: fmt.Sprintf("creating new config file in %s", globalCfg.DataDir),
		}
		// creting config folder if not exists
		err = os.MkdirAll(globalCfg.DataDir, os.ModePerm)
		if err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot create data directory: %s", err),
			}
		}
		// create config file if not exists
		if err := viper.SafeWriteConfig(); err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot write config file into config dir: %s", err),
			}
		}
	} else {
		// read config file
		err = viper.ReadInConfig()
		if err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot read loaded config file in %s: %s", globalCfg.DataDir, err),
			}
		}
	}
	err = viper.Unmarshal(&globalCfg)
	if err != nil {
		cfgError = config.Error{
			Message: fmt.Sprintf("cannot unmarshal loaded config file: %s", err),
		}
	}

	globalCfg.DataDir += "/vochain"
	if globalCfg.SaveConfig {
		viper.Set("saveConfig", false)
		if err := viper.WriteConfig(); err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot overwrite config file into config dir: %s", err),
			}
		}
	}
	return globalCfg, cfgError
}

func main() {
	// creating config and init logger
	globalCfg, cfgErr := newConfig()
	if globalCfg == nil {
		panic("cannot read configuration")
	}

	fmt.Println(globalCfg.LogLevel)
	log.Init(globalCfg.LogLevel, globalCfg.LogOutput)

	// check if errors during config creation and determine if Critical
	if cfgErr.Critical && cfgErr.Message != "" {
		log.Fatalf("Critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message != "" {
		log.Warnf("non Critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message == "" {
		log.Infof("config file loaded successfully, remember CLI flags have preference")
	}

	log.Info("starting vochain miner...")

	// getting node exposed IP if not set
	if len(globalCfg.PublicAddr) == 0 {
		ip, err := util.PublicIP()
		if err != nil {
			log.Warn(err)
		} else {
			_, port, err := net.SplitHostPort(globalCfg.P2PListen)
			if err == nil {
				globalCfg.PublicAddr = fmt.Sprintf("%s:%s", ip, port)
			}
		}
	} else {
		host, port, err := net.SplitHostPort(globalCfg.P2PListen)
		if err == nil {
			globalCfg.PublicAddr = net.JoinHostPort(host, port)
		}
	}

	log.Infof("exposed host IP address: %s", globalCfg.PublicAddr)

	// node + app layer
	var vnode *vochain.BaseApplication
	if globalCfg.Dev {
		vnode = vochain.NewVochain(globalCfg, []byte(vochain.DevelopmentGenesis1))
	} else {
		vnode = vochain.NewVochain(globalCfg, []byte(vochain.TestnetGenesis1))
	}
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
