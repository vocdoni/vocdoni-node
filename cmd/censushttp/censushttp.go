package main

import (
	"os"
	"os/user"
	"strconv"
	"strings"

	viper "github.com/spf13/viper"
	flag "github.com/spf13/pflag"

	censusmanager "gitlab.com/vocdoni/go-dvote/service/census"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/config"
)

func newConfig() (config.CensusCfg, error) {
	var globalCfg config.CensusCfg
	//setup flags
	usr, err := user.Current()
	if err != nil {
		return globalCfg, err
	}
	defaultDirPath := usr.HomeDir + "/.dvote/censushttp"
	//setup flags
	path := flag.String("cfgpath", defaultDirPath+"/config.yaml", "cfgpath. Specify filepath for censushttp config")
	flag.String("loglevel", "warn", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	

	viper := viper.New()
	viper.SetDefault("loglevel", "warn")
	flag.Parse()
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

	viper.BindPFlag("logLevel", flag.Lookup("loglevel"))
	
	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

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
