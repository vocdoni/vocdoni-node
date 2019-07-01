package main

import (
	"github.com/spf13/viper"
	flag "github.com/spf13/pflag"

	"time"

	"github.com/vocdoni/go-dvote/config"
	"github.com/vocdoni/go-dvote/chain"
	"github.com/vocdoni/go-dvote/log"
)

func newConfig() (config.W3Cfg, error) {
	//setup flags
	flag.String("loglevel", "info", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	flag.Parse()
	viper := viper.New()
	var globalCfg config.W3Cfg
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/go-dvote/cmd/web3/") // path to look for the config file in
	viper.AddConfigPath(".")                      // optionally look for config in the working directory
	err := viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	viper.BindPFlags(flag.CommandLine)
	
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
	log.InitLoggerAtLevel(globalCfg.LogLevel)
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
