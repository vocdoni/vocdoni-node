package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vocone"
)

// VoconeConfig contains the basic configuration for the voconed
type VoconeConfig struct {
	logLevel, dir, oracle, path   string
	port, blockSeconds, blockSize int
}

func main() {
	config := VoconeConfig{}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	flag.StringVar(&config.dir, "dir", filepath.Join(home, ".voconed"), "storage data directory")
	flag.StringVar(&config.oracle, "oracle", "", "oracle private hexadecimal key")
	flag.StringVar(&config.logLevel, "logLevel", "info", "log level (info, debug, warn, error)")
	flag.IntVar(&config.port, "port", 9095, "network port for the HTTP API")
	flag.StringVar(&config.path, "urlPath", "/dvote", "HTTP path for the API rest")
	flag.IntVar(&config.blockSeconds, "blockPeriod", int(vocone.DefaultBlockTimeTarget.Seconds()), "block time target in seconds")
	flag.IntVar(&config.blockSize, "blockSize", int(vocone.DefaultTxsPerBlock), "max number of transactions per block")
	flag.CommandLine.SortFlags = false
	flag.Parse()

	viper := viper.New()
	viper.SetConfigName("voconed")
	viper.SetConfigType("env")
	viper.SetEnvPrefix("VOCONED")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set FlagVars first
	viper.BindPFlag("dir", flag.Lookup("dir"))
	config.dir = viper.GetString("dir")
	viper.BindPFlag("oracle", flag.Lookup("oracle"))
	config.oracle = viper.GetString("oracle")
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	config.logLevel = viper.GetString("logLevel")
	viper.BindPFlag("port", flag.Lookup("port"))
	config.port = viper.GetInt("port")
	viper.BindPFlag("urlPath", flag.Lookup("urlPath"))
	config.path = viper.GetString("urlPath")
	viper.BindPFlag("blockPeriod", flag.Lookup("blockPeriod"))
	config.blockSeconds = viper.GetInt("blockPeriod")
	viper.BindPFlag("blockSize", flag.Lookup("blockSize"))
	config.blockSize = viper.GetInt("blockSize")

	viper.AddConfigPath(config.dir)

	_, err = os.Stat(filepath.Join(config.dir, "voconed.env"))
	if os.IsNotExist(err) {
		if err = os.MkdirAll(config.dir, os.ModePerm); err != nil {
			panic(err)
		}
		if err := viper.SafeWriteConfig(); err != nil {
			panic(err)
		}
	} else {
		err = viper.ReadInConfig()
		if err != nil {
			panic(err)
		}
	}
	if err = viper.Unmarshal(&config); err != nil {
		panic(err)
	}

	log.Init(config.logLevel, "stdout")
	log.Infof("using data directory at %s", config.dir)

	oracle := ethereum.SignKeys{}
	if config.oracle == "" {
		if err := oracle.Generate(); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := oracle.AddHexKey(config.oracle); err != nil {
			log.Fatal(err)
		}
	}

	vc, err := vocone.NewVocone(config.dir, &oracle)
	if err != nil {
		log.Fatal(err)
	}

	vc.SetBlockTimeTarget(time.Second * time.Duration(config.blockSeconds))
	vc.SetBlockSize(config.blockSize)
	go vc.Start()
	vc.EnableAPI("0.0.0.0", config.port, config.path)

	select {}
}
