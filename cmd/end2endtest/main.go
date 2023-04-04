package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/internal"
	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

// how many times to retry flaky transactions
// * amount of blocks to wait for a transaction to be mined before giving up
// * how many times to retry opening a connection to an endpoint before giving up
const retries = 10

type operation struct {
	fn func(c config)

	name, description, example string
}

var ops = []operation{
	{
		fn:          testTokenTransactions,
		name:        "tokentransactions",
		description: "Tests all token related transactions",
		example: os.Args[0] + " --operation=tokentransactions " +
			"--host http://127.0.0.1:9090/v2",
	},
	{
		fn:          mkTreeAnonVoteTest,
		name:        "anonvoting",
		description: "Performs a complete test of anonymous election, from creating a census to voting and validating votes",
		example: os.Args[0] + " --operation=anonvoting --votes=1000 " +
			"--oracleKey=6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223",
	},
}

func opNames() (names []string) {
	for _, op := range ops {
		names = append(names, op.name)
	}
	return names
}

type config struct {
	host             string
	logLevel         string
	operation        string
	accountPrivKeys  []string
	accountKeys      []*ethereum.SignKeys
	treasurerPrivKey string
	nvotes           int
	parallelCount    int
	faucet           string
	faucetAuthToken  string
	timeout          time.Duration
}

func main() {
	// Report the version before loading the config or logger init, just in case something goes wrong.
	// For the sake of including the version in the log, it's also included in a log line later on.
	fmt.Fprintf(os.Stderr, "vocdoni version %q\n", internal.Version)

	c := config{}
	flag.StringVar(&c.host, "host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	flag.StringVar(&c.logLevel, "logLevel", "info", "log level (debug, info, warn, error, fatal)")
	flag.StringVar(&c.operation, "operation", "vtest",
		fmt.Sprintf("set operation mode: %v", opNames()))
	flag.StringSliceVarP(&c.accountPrivKeys, "accountPrivKey", "k", []string{},
		"account private key (optional)")
	flag.StringVar(&c.treasurerPrivKey, "treasurerPrivKey", "", "treasurer private key")
	flag.IntVar(&c.nvotes, "votes", 10, "number of votes to cast")
	flag.IntVar(&c.parallelCount, "parallel", 4, "number of parallel requests")
	flag.StringVar(&c.faucet, "faucet", "dev",
		"faucet URL for fetching tokens (special keyword 'dev' translates into hardcoded URL for dev faucet)")
	flag.StringVar(&c.faucetAuthToken, "faucetAuthToken", "",
		"(optional) token passed as Bearer when fetching faucetURL")
	flag.DurationVar(&c.timeout, "timeout", 5*time.Minute, "timeout duration")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSome examples of different operation modes:\n")
		for _, op := range ops {
			fmt.Fprintf(os.Stderr, "### %s\n", op.name)
			fmt.Fprintf(os.Stderr, "\t"+op.description+"\n")
			fmt.Fprintf(os.Stderr, op.example+"\n")
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	flag.CommandLine.SortFlags = false
	flag.Parse()

	log.Init(c.logLevel, "stdout")
	log.Infow("starting "+filepath.Base(os.Args[0]), "version", internal.Version)

	if len(c.accountPrivKeys) == 0 {
		c.accountPrivKeys = []string{util.RandomHex(32)}
		log.Infof("no keys passed, generated random private key: %s", c.accountPrivKeys)
	}

	c.accountKeys = make([]*ethereum.SignKeys, len(c.accountPrivKeys))
	for i, key := range c.accountPrivKeys {
		ak, err := privKeyToSigner(key)
		if err != nil {
			log.Fatal(err)
		}
		c.accountKeys[i] = ak
		log.Infof("privkey %x = account %s", ak.PrivateKey(), ak.AddressString())
	}

	found := false
	for _, op := range ops {
		if op.name == c.operation {
			op.fn(c)
			found = true
		}
	}

	if !found {
		log.Fatal("no valid operation mode specified")
	}

}

func privKeyToSigner(key string) (*ethereum.SignKeys, error) {
	var skey *ethereum.SignKeys
	if len(key) > 0 {
		skey = ethereum.NewSignKeys()
		if err := skey.AddHexKey(key); err != nil {
			return nil, fmt.Errorf("cannot create key %s with err %s", key, err)
		}
	}
	return skey, nil
}

func getFaucetPackage(c config, account string) (*models.FaucetPackage, error) {
	if c.faucet == "" {
		return nil, fmt.Errorf("need to pass a valid --faucet")
	}
	if c.faucet == "dev" {
		return apiclient.GetFaucetPackageFromDevService(account)
	} else {
		return apiclient.GetFaucetPackageFromRemoteService(c.faucet+account,
			c.faucetAuthToken)
	}
}
