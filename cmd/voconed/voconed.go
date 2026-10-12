package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.vocdoni.io/dvote/api/faucet"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/internal"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vocone"
)

// defaultFaucetBalance is the genesis balance of the faucet account.
const defaultFaucetBalance = 1000000

// VoconeConfig contains the basic configuration for the voconed
type VoconeConfig struct {
	logLevel, dir, keymanager, path, chainID string
	port, blockSeconds, blockSize            int
	txCosts                                  uint64
	fundedAccounts                           []string
	enableFaucetWithAmount                   uint64
	faucetKey                                string
	faucetBalance                            uint64
	adminToken                               string
	importGenesis                            string
	importFundedAccounts                     []string
}

func main() {
	// Report the version before loading the config or logger init, just in case something goes wrong.
	// For the sake of including the version in the log, it's also included in a log line later on.
	fmt.Fprintf(os.Stderr, "vocdoni version %q\n", internal.Version)

	config := VoconeConfig{}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	flag.StringVar(&config.dir, "dir", filepath.Join(home, ".voconed"), "storage data directory")
	flag.StringVar(&config.keymanager, "keymanager", "", "key manager private hexadecimal key")
	flag.StringVar(&config.logLevel, "logLevel", "info", "log level (info, debug, warn, error)")
	flag.StringVar(&config.chainID, "chainID", "vocone", "defines the chainID")
	flag.IntVar(&config.port, "port", 9090, "network port for the HTTP API")
	flag.StringVar(&config.path, "urlPath", "/v2", "HTTP path for the API rest")
	flag.IntVar(&config.blockSeconds, "blockPeriod", int(vocone.DefaultBlockTimeTarget.Seconds()), "block time target in seconds")
	flag.IntVar(&config.blockSize, "blockSize", vocone.DefaultTxsPerBlock, "max number of transactions per block")
	setTxCosts := flag.Bool("setTxCosts", false, "if true, transaction costs are set to the value of txCosts flag")
	flag.Uint64Var(&config.txCosts, "txCosts", vocone.DefaultTxCosts, "transaction costs for all types")
	flag.Uint64Var(&config.enableFaucetWithAmount, "enableFaucet", 0, "enable faucet API service for the given amount")
	flag.StringVar(&config.faucetKey, "faucetKey", "",
		"faucet account private hexadecimal key (generated and saved in the config file if empty)")
	flag.Uint64Var(&config.faucetBalance, "faucetBalance", defaultFaucetBalance,
		"faucet account balance at genesis (only applied when the chain is created)")
	flag.StringVar(&config.adminToken, "adminToken", "",
		"bearer token for the admin API endpoints (admin endpoints are disabled if empty)")

	flag.StringVar(&config.importGenesis, "importGenesis", "",
		"genesis.json of an existing chain whose state (vcstate) and indexer were copied into --dir; "+
			"the chain is continued from its last height instead of applying a new genesis (only used once, "+
			"when --dir holds no vocone chain yet)")

	flag.StringSliceVar(&config.importFundedAccounts, "importFundedAccounts", []string{},
		"list of accounts (address:amount,...) whose balance is increased by amount when the chain is imported "+
			"with --importGenesis; no other tokens are minted on import")

	flag.StringSliceVar(&config.fundedAccounts, "fundedAccounts", []string{},
		"list of pre-funded accounts (address:balance,address:balance,...), only applied when the chain is created")
	flag.CommandLine.SortFlags = false
	flag.Parse()

	pviper := viper.New()
	pviper.SetConfigName("voconed")
	pviper.SetConfigType("yml")
	pviper.SetEnvPrefix("VOCONED")
	pviper.AutomaticEnv()
	pviper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set FlagVars first
	if err := pviper.BindPFlag("dir", flag.Lookup("dir")); err != nil {
		log.Fatal(err)
	}
	config.dir = pviper.GetString("dir")

	pviper.AddConfigPath(config.dir)
	_ = pviper.ReadInConfig()

	if err := pviper.BindPFlag("keymanager", flag.Lookup("keymanager")); err != nil {
		panic(err)
	}
	config.keymanager = pviper.GetString("keymanager")

	if err := pviper.BindPFlag("chainID", flag.Lookup("chainID")); err != nil {
		panic(err)
	}
	config.chainID = pviper.GetString("chainID")

	if err := pviper.BindPFlag("logLevel", flag.Lookup("logLevel")); err != nil {
		panic(err)
	}
	config.logLevel = pviper.GetString("logLevel")

	if err := pviper.BindPFlag("port", flag.Lookup("port")); err != nil {
		panic(err)
	}
	config.port = pviper.GetInt("port")

	if err := pviper.BindPFlag("urlPath", flag.Lookup("urlPath")); err != nil {
		panic(err)
	}
	config.path = pviper.GetString("urlPath")

	if err := pviper.BindPFlag("enableFaucetWithAmount", flag.Lookup("enableFaucet")); err != nil {
		panic(err)
	}
	config.enableFaucetWithAmount = pviper.GetUint64("enableFaucetWithAmount")

	if err := pviper.BindPFlag("blockPeriod", flag.Lookup("blockPeriod")); err != nil {
		panic(err)
	}
	config.blockSeconds = pviper.GetInt("blockPeriod")

	if err := pviper.BindPFlag("blockSize", flag.Lookup("blockSize")); err != nil {
		panic(err)
	}
	config.blockSize = pviper.GetInt("blockSize")

	if err := pviper.BindPFlag("txCosts", flag.Lookup("txCosts")); err != nil {
		panic(err)
	}
	config.txCosts = pviper.GetUint64("txCosts")

	if err := pviper.BindPFlag("setTxCosts", flag.Lookup("setTxCosts")); err != nil {
		panic(err)
	}
	*setTxCosts = pviper.GetBool("setTxCosts")

	if err := pviper.BindPFlag("faucetKey", flag.Lookup("faucetKey")); err != nil {
		panic(err)
	}
	config.faucetKey = pviper.GetString("faucetKey")

	if err := pviper.BindPFlag("faucetBalance", flag.Lookup("faucetBalance")); err != nil {
		panic(err)
	}
	config.faucetBalance = pviper.GetUint64("faucetBalance")

	if err := pviper.BindPFlag("adminToken", flag.Lookup("adminToken")); err != nil {
		panic(err)
	}
	config.adminToken = pviper.GetString("adminToken")

	if err := pviper.BindPFlag("fundedAccounts", flag.Lookup("fundedAccounts")); err != nil {
		panic(err)
	}
	config.fundedAccounts = pviper.GetStringSlice("fundedAccounts")

	_, err = os.Stat(filepath.Join(config.dir, "voconed.yml"))
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(config.dir, os.ModePerm); err != nil {
				panic(err)
			}
			if err := pviper.SafeWriteConfig(); err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}
	if err = pviper.Unmarshal(&config); err != nil {
		panic(err)
	}

	// generate a new management key if not provided and save it
	if config.keymanager == "" {
		fmt.Println("generating new random key for key manager")
		mngKey := ethereum.SignKeys{}
		if err := mngKey.Generate(); err != nil {
			panic(err)
		}
		config.keymanager = hex.EncodeToString(mngKey.PrivateKey())
		pviper.Set("keymanager", config.keymanager)
	}

	// importing an existing chain: no account is funded, so the faucet needs the key of an
	// account that already holds funds in the imported state
	importing := config.importGenesis != "" && !fileExists(filepath.Join(config.dir, vocone.GenesisFile))
	if importing && config.enableFaucetWithAmount > 0 && config.faucetKey == "" {
		fmt.Fprintln(os.Stderr, "importing a chain with the faucet enabled requires --faucetKey "+
			"(the key of an account funded in the imported state); a generated key would hold no funds")
		os.Exit(1)
	}

	// generate a new faucet key if the faucet is enabled and no key is provided, and save it
	// so the faucet account is the same across restarts
	if config.enableFaucetWithAmount > 0 && config.faucetKey == "" {
		fmt.Println("generating new random key for the faucet")
		faucetKey := ethereum.SignKeys{}
		if err := faucetKey.Generate(); err != nil {
			panic(err)
		}
		config.faucetKey = hex.EncodeToString(faucetKey.PrivateKey())
		pviper.Set("faucetKey", config.faucetKey)
	}

	if err := pviper.WriteConfig(); err != nil {
		panic(err)
	}

	// start the program
	log.Init(config.logLevel, "stdout", nil)
	log.Infow("starting "+filepath.Base(os.Args[0]), "version", internal.Version)

	log.Infof("using data directory at %s", config.dir)

	mngKey := ethereum.SignKeys{}
	if err := mngKey.AddHexKey(config.keymanager); err != nil {
		log.Fatal(err)
	}

	var faucetAccount *ethereum.SignKeys
	if config.enableFaucetWithAmount > 0 {
		faucetAccount = &ethereum.SignKeys{}
		if err := faucetAccount.AddHexKey(config.faucetKey); err != nil {
			log.Fatalf("invalid faucet key: %v", err)
		}
	}

	if importing {
		data, err := os.ReadFile(config.importGenesis)
		if err != nil {
			log.Fatalf("cannot read genesis to import: %v", err)
		}
		doc, err := vocone.ParseImportGenesis(data)
		if err != nil {
			log.Fatalf("cannot parse genesis to import: %v", err)
		}
		res, err := vocone.ImportChain(config.dir, doc)
		if err != nil {
			log.Fatalf("cannot import chain: %v", err)
		}
		log.Infow("chain imported, the genesis was not applied",
			"chainID", res.ChainID,
			"stateHeight", res.StateHeight,
			"firstHeight", res.FirstHeight,
			"indexerHeight", res.IndexerHeight,
			"indexerBlocks", res.IndexerBlockCount,
		)
	} else {
		if config.importGenesis != "" {
			log.Infow("data directory already holds a chain, ignoring importGenesis", "dir", config.dir)
		}
		if len(config.importFundedAccounts) > 0 {
			log.Warnw("ignoring importFundedAccounts, they are only applied when the chain is imported")
		}
	}

	vc, err := vocone.NewVocone(config.dir, &mngKey)
	if err != nil {
		log.Fatal(err)
	}
	if importing {
		funded, err := parseFundedAccounts(config.importFundedAccounts)
		if err != nil {
			log.Fatal(err)
		}
		for _, acc := range funded {
			addr := common.BytesToAddress(acc.Address)
			if err := vc.MintBalance(addr, acc.Balance); err != nil {
				log.Fatal(err)
			}
			log.Warnw("minted tokens on import", "address", addr.Hex(), "amount", acc.Balance)
		}
	}
	vc.SetBlockSize(config.blockSize)
	vc.App.SetBlockTimeTarget(time.Second * time.Duration(config.blockSeconds))
	vc.AdminToken = config.adminToken

	if vc.Initialized() {
		// The genesis (chainID, funded accounts, faucet balance) is only applied when the
		// chain is created; on restart the persisted chain is resumed as is.
		if vc.App.ChainID() != config.chainID {
			log.Warnw("ignoring chainID flag, using the chainID of the existing chain",
				"flag", config.chainID, "chainID", vc.App.ChainID())
		}
		if len(config.fundedAccounts) > 0 {
			log.Warnw("ignoring funded accounts, they are only applied when the chain is created")
		}
		if base := vc.ImportBase(); base > 0 {
			log.Infow("running an imported chain, older blocks are served by the indexer", "blockStoreBase", base)
		}
	} else {
		appState := vocone.DefaultGenesisAppState(config.blockSize)
		funded, err := parseFundedAccounts(config.fundedAccounts)
		if err != nil {
			log.Fatal(err)
		}
		for _, acc := range funded {
			log.Infof("funding account %x with balance %d", acc.Address, acc.Balance)
		}
		appState.Accounts = append(appState.Accounts, funded...)
		if faucetAccount != nil {
			log.Infof("funding faucet account %s with balance %d", faucetAccount.Address().Hex(), config.faucetBalance)
			appState.Accounts = append(appState.Accounts, genesis.Account{
				Address: faucetAccount.Address().Bytes(),
				Balance: config.faucetBalance,
			})
		}
		if err := vc.InitChain(config.chainID, appState); err != nil {
			log.Fatal(err)
		}
	}
	log.Infof("using chainID: %s", vc.App.ChainID())

	if *setTxCosts {
		if err := vc.SetBulkTxCosts(config.txCosts, true); err != nil {
			log.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	startDone := make(chan struct{})
	go func() {
		defer close(startDone)
		if err := vc.Start(ctx); err != nil {
			log.Fatal(err)
		}
	}()
	uAPI, err := vc.EnableAPI("0.0.0.0", config.port, config.path)
	if err != nil {
		log.Fatal(err)
	}

	vc.Router.ExposePrometheusEndpoint("/metrics")
	metrics.NewCounter(fmt.Sprintf("vocdoni_info{version=%q,mode=%q,chain=%q}",
		internal.Version, "vocone", vc.App.ChainID())).Set(1)

	// enable faucet if requested, the faucet account is funded at genesis
	if faucetAccount != nil {
		acc, err := vc.App.State.GetAccount(faucetAccount.Address(), true)
		if err != nil {
			log.Fatal(err)
		}
		if acc == nil || acc.Balance == 0 {
			log.Warnw("faucet account has no funds, claims will fail", "address", faucetAccount.Address().Hex())
		}
		log.Infof("faucet account %s, faucet amount %d", faucetAccount.Address().Hex(), config.enableFaucetWithAmount)
		if err := faucet.AttachFaucetAPI(faucetAccount,
			config.enableFaucetWithAmount,
			uAPI.Endpoint,
			"/open/claim",
		); err != nil {
			log.Fatal(err)
		}
	}

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Warnf("received SIGTERM, exiting at %s", time.Now().Format(time.RFC850))
	cancel()
	// Wait for the block production loop to exit before closing resources.
	<-startDone
	if err := vc.Close(); err != nil {
		log.Warnw("error closing vocone", "err", err)
	}
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseFundedAccounts parses a list of address:amount entries.
func parseFundedAccounts(list []string) ([]genesis.Account, error) {
	accounts := make([]genesis.Account, 0, len(list))
	for _, entry := range list {
		data := strings.Split(entry, ":")
		if len(data) != 2 {
			return nil, fmt.Errorf("invalid funded account %s, please specify addr:balance", entry)
		}
		if !common.IsHexAddress(data[0]) {
			return nil, fmt.Errorf("invalid funded account address %s", data[0])
		}
		balance, err := strconv.ParseUint(data[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid balance amount %s", data[1])
		}
		accounts = append(accounts, genesis.Account{Address: common.HexToAddress(data[0]).Bytes(), Balance: balance})
	}
	return accounts, nil
}
