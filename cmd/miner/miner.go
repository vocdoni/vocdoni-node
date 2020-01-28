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

	userDir := home + "/.dvote"

	// creating flags
	globalCfg.DataDir = *flag.String("dataDir", userDir, "directory where data is stored")
	globalCfg.P2PListen = *flag.String("p2pListen", "0.0.0.0:26656", "p2p host and port to listen")
	globalCfg.RPCListen = *flag.String("rpcListen", "0.0.0.0:26657", "rpc host and port to listen")
	globalCfg.Genesis = *flag.String("genesis", "", "use alternative genesis file")
	globalCfg.KeyFile = *flag.String("keyFile", "", "user alternative p2p node key file")
	globalCfg.MinerKeyFile = *flag.String("minerKeyFile", "", "user alternative node key file for mining")
	globalCfg.SeedMode = *flag.Bool("seedMode", false, "act as a seed node")
	globalCfg.Peers = *flag.StringArray("peers", []string{}, "coma separated list of p2p peers")
	globalCfg.Seeds = *flag.StringArray("seeds", []string{}, "coma separated list of p2p seed nodes")
	globalCfg.LogLevel = *flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	globalCfg.PublicAddr = *flag.String("publicAddr", "", "IP address where the node will be exposed, guessed automatically if empty")
	// parse flags
	flag.Parse()

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
	viper.BindPFlag("keyFile", flag.Lookup("keyFile"))
	viper.BindPFlag("minerKeyFile", flag.Lookup("minerKeyFile"))
	viper.BindPFlag("seedMode", flag.Lookup("seedMode"))
	viper.BindPFlag("peers", flag.Lookup("peers"))
	viper.BindPFlag("seeds", flag.Lookup("seeds"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("genesis", flag.Lookup("genesis"))
	viper.BindPFlag("publicAddr", flag.Lookup("publicAddr"))

	// check if config file exists
	_, err = os.Stat(globalCfg.DataDir + "/vochain-miner.yml")
	if os.IsNotExist(err) {
		cfgError = config.Error{
			Critical: false,
			Message:  fmt.Sprintf("creating new config file in %s", globalCfg.DataDir),
		}
		// creting config folder if not exists
		err = os.MkdirAll(globalCfg.DataDir, os.ModePerm)
		if err != nil {
			cfgError = config.Error{
				Critical: false,
				Message:  fmt.Sprintf("cannot create data directory (%s)", err),
			}
		}
		// create config file if not exists
		if err = viper.SafeWriteConfig(); err != nil {
			cfgError = config.Error{
				Critical: false,
				Message:  fmt.Sprintf("cannot write config file into config dir (%s)", err),
			}
		}
	} else {
		// read config file
		err = viper.ReadInConfig()
		if err != nil {
			cfgError = config.Error{
				Critical: false,
				Message:  fmt.Sprintf("cannot read loaded config file in %s (%s)", err, globalCfg.DataDir),
			}
		}
		err = viper.Unmarshal(&globalCfg)
		if err != nil {
			cfgError = config.Error{
				Critical: false,
				Message:  fmt.Sprintf("cannot unmarshal loaded config file (%s)", err),
			}
		}
	}
	globalCfg.DataDir += "/vochain"
	return globalCfg, cfgError
}

func main() {
	// creating config and init logger
	globalCfg, cfgErr := newConfig()
	if globalCfg == nil {
		panic("cannot read configuration")
	}

	fmt.Println(globalCfg.LogLevel)
	log.InitLogger(globalCfg.LogLevel, globalCfg.LogOutput)

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
			addrport := strings.Split(globalCfg.P2PListen, ":")
			if len(addrport) > 0 {
				globalCfg.PublicAddr = fmt.Sprintf("%s:%s", ip, addrport[len(addrport)-1])
			}
		}
	} else {
		addrport := strings.Split(globalCfg.P2PListen, ":")
		if len(addrport) > 0 {
			globalCfg.PublicAddr = fmt.Sprintf("%s:%s", addrport[0], addrport[1])
		}
	}

	log.Infof("exposed host IP address: %s", globalCfg.PublicAddr)

	// node + app layer
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
