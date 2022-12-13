package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vocone"
	"go.vocdoni.io/proto/build/go/models"
)

// VoconeConfig contains the basic configuration for the voconed
type VoconeConfig struct {
	logLevel, dir, keymanager, path, treasurer, chainID string
	port, blockSeconds, blockSize                       int
	txCosts                                             uint64
	disableIpfs                                         bool
	fundedAccounts                                      []string
}

func main() {
	config := VoconeConfig{}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	flag.StringVar(&config.dir, "dir", filepath.Join(home, ".voconed"), "storage data directory")
	flag.StringVar(&config.keymanager, "keymanager", "", "key manager private hexadecimal key")
	flag.StringVar(&config.treasurer, "treasurer", "", "treasurer address")
	flag.StringVar(&config.logLevel, "logLevel", "info", "log level (info, debug, warn, error)")
	flag.StringVar(&config.chainID, "chainID", "vocone", "defines the chainID")
	flag.IntVar(&config.port, "port", 9095, "network port for the HTTP API")
	flag.StringVar(&config.path, "urlPath", "/api", "HTTP path for the API rest")
	flag.IntVar(&config.blockSeconds, "blockPeriod", int(vocone.DefaultBlockTimeTarget.Seconds()), "block time target in seconds")
	flag.IntVar(&config.blockSize, "blockSize", int(vocone.DefaultTxsPerBlock), "max number of transactions per block")
	setTxCosts := flag.Bool("setTxCosts", false, "if true, transaction costs are set to the value of txCosts flag")
	flag.Uint64Var(&config.txCosts, "txCosts", vocone.DefaultTxCosts, "transaction costs for all types")
	flag.BoolVar(&config.disableIpfs, "disableIpfs", false, "disable built-in IPFS node")
	flag.StringSliceVar(&config.fundedAccounts, "fundedAccounts", []string{},
		"list of pre-funded accounts (address:balance,address:balance,...)")
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
	viper.BindPFlag("keymanager", flag.Lookup("keymanager"))
	config.keymanager = viper.GetString("keymanager")
	viper.BindPFlag("chainID", flag.Lookup("chainID"))
	config.chainID = viper.GetString("chainID")
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
	viper.BindPFlag("txCosts", flag.Lookup("txCosts"))
	config.txCosts = viper.GetUint64("txCosts")

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

	mngKey := ethereum.SignKeys{}
	mngKey.Generate()
	if config.keymanager != "" {
		if err := mngKey.AddHexKey(config.keymanager); err != nil {
			log.Fatal(err)
		}
	}

	vc, err := vocone.NewVocone(config.dir, &mngKey)
	if err != nil {
		log.Fatal(err)
	}
	vc.SetChainID(config.chainID)
	log.Infof("using chainID: %s", config.chainID)

	// set treasurer address if provided
	if len(config.treasurer) > 0 {
		log.Infof("setting treasurer %s", config.treasurer)
		if err := vc.SetTreasurer(common.HexToAddress(config.treasurer)); err != nil {
			log.Fatal(err)
		}
	}
	// set transaction costs
	if *setTxCosts {
		log.Infof("setting tx costs to %d", config.txCosts)
		if err := vc.SetBulkTxCosts(config.txCosts, true); err != nil {
			log.Fatal(err)
		}
	}

	// set funded accounts if any
	if len(config.fundedAccounts) > 0 {
		for _, addr := range config.fundedAccounts {
			data := strings.Split(addr, ":")
			if len(data) != 2 {
				log.Fatalf("invalid funded account %s, please specify addr:balance", addr)
			}
			balance, err := strconv.ParseUint(data[1], 10, 64)
			if err != nil {
				log.Fatalf("invalid balance amount %s", data[1])
			}
			a := common.HexToAddress(data[0])
			log.Infof("funding account %s with balance %d", a.Hex(), balance)
			vc.CreateAccount(a, &state.Account{
				Account: models.Account{
					Balance: balance,
				},
			})
		}
	}

	vc.SetBlockTimeTarget(time.Second * time.Duration(config.blockSeconds))
	vc.SetBlockSize(config.blockSize)
	go vc.Start()
	vc.EnableAPI("0.0.0.0", config.port, config.path)

	select {}
}
