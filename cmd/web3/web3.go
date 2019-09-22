package main

import (
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"os"
	"os/user"
	"time"

	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
)

func newConfig() (config.W3Cfg, error) {
	var globalCfg config.W3Cfg
	//setup flags
	usr, err := user.Current()
	if err != nil {
		return globalCfg, err
	}
	defaultDirPath := usr.HomeDir + "/.dvote/web3"
	path := flag.String("cfgpath", defaultDirPath+"/config.yaml", "cfgpath. Specify filepath for web3 config")

	flag.String("chain", "vctestnet", "Blockchain to connect")
	flag.Int("wsPort", 0, "websockets port")
	flag.String("wsHost", "0.0.0.0", "ws host to listen on")
	flag.Int("httpPort", 9091, "http endpoint port, disabled if 0")
	flag.String("httpHost", "0.0.0.0", "http host to listen on")
	flag.String("route", "/web3", "proxy endpoint exposing web3")
	flag.String("loglevel", "warn", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")

	flag.Parse()

	viper := viper.New()
	viper.SetDefault("chainType", "vctestnet")
	viper.SetDefault("wsPort", 0)
	viper.SetDefault("wsHost", "0.0.0.0")
	viper.SetDefault("httpPort", 9091)
	viper.SetDefault("httpHost", "0.0.0.0")
	viper.SetDefault("route", "/web3")
	viper.SetDefault("logLevel", "warn")

	viper.SetConfigType("yaml")
	if *path == defaultDirPath+"/config.yaml" { //if path left default, write new cfg file if empty or if file doesn't exist.
		if err = viper.SafeWriteConfigAs(*path); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(defaultDirPath, os.ModePerm)
				if err != nil {
					return globalCfg, err
				}
				err = viper.WriteConfigAs(*path)
				if err != nil {
					return globalCfg, err
				}
			}
		}
	}

	viper.BindPFlag("chainType", flag.Lookup("chain"))
	viper.BindPFlag("wsPort", flag.Lookup("wsPort"))
	viper.BindPFlag("wsHost", flag.Lookup("wsHost"))
	viper.BindPFlag("httpPort", flag.Lookup("httpPort"))
	viper.BindPFlag("httpHost", flag.Lookup("httpHost"))
	viper.BindPFlag("route", flag.Lookup("route"))
	viper.BindPFlag("logLevel", flag.Lookup("loglevel"))

	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

/*
Example code for using web3 implementation

Testing the RPC can be performed with curl and/or websocat
 curl -X POST -H "Content-Type:application/json" --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":74}' localhost:9091
 echo '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":74}' | websocat ws://127.0.0.1:9092
*/
func main() {
	//setup config
	globalCfg, err := newConfig()
	//setup logger
	log.InitLogger(globalCfg.LogLevel, "stdout")
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	cfg, err := chain.NewConfig(globalCfg)
	if err != nil {
		log.Panic(err)
	}

	node, err := chain.Init(cfg)
	if err != nil {
		log.Panic(err)
	}

	node.Start()

	for {
		time.Sleep(1 * time.Second)
	}

}
