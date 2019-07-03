package main

import (
	"os"
	"strconv"
	"strings"

	viper "github.com/spf13/viper"
	flag "github.com/spf13/pflag"

	censusmanager "gitlab.com/vocdoni/go-dvote/service/census"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/config"
)

func newConfig() (config.CensusCfg, error) {
	//setup flags
	path := flag.String("cfgpath", "./", "cfgpath. Specify filepath for gateway config file")
	flag.String("loglevel", "warn", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	flag.Parse()
	viper := viper.New()
	var globalCfg config.CensusCfg
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(*path) // path to look for the config file in
	viper.AddConfigPath(".")                      // optionally look for config in the working directory
	err := viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	viper.BindPFlag("logLevel", flag.Lookup("loglevel"))
	
	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

func main() {
	//setup config
	globalCfg, err := newConfig()
	//setup logger
	log.InitLoggerAtLevel(globalCfg.LogLevel)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	if len(os.Args) < 2 {
		log.Fatalf("Usage: " + os.Args[0] +
			" <port> <namespace>[:pubKey] [<namespace>[:pubKey]]...")
		os.Exit(2)
	}
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
		os.Exit(2)
	}
	for i := 2; i < len(os.Args); i++ {
		s := strings.Split(os.Args[i], ":")
		ns := s[0]
		pubK := ""
		if len(s) > 1 {
			pubK = s[1]
			log.Infof("Public Key authentication enabled on namespace %s\n", ns)
		}
		censusmanager.AddNamespace(ns, pubK)
		log.Infof("Starting process HTTP service on port %d for namespace %s\n",
			port, ns)
	}
	censusmanager.Listen(port, "http")

}
