package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	flag "github.com/spf13/pflag"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/internal"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

// how many times to retry flaky transactions
// * amount of blocks to wait for a transaction to be mined before giving up
// * how many times to retry opening a connection to an endpoint before giving up
const retries = 10

// VochainTest is the interface that tests should comply with.
// Duration of each step will be measured and reported.
//
//   - Setup() is intended to create accounts, elections, etc.
//   - Run() can be, for example, to emit votes.
//   - Teardown() can be used to clean up afterwards.
type VochainTest interface {
	Setup(api *apiclient.HTTPclient, config *config) error
	Run() error
	Teardown() error
}

type e2eElection struct {
	api      *apiclient.HTTPclient
	config   *config
	election *vapi.Election
	voters   *sync.Map // key:acc.Public, value: acctProof struct {account, proof, proofSik}
}

type operation struct {
	test VochainTest

	description, example string
}

var ops = map[string]operation{}

func opNames() (names []string) {
	for name := range ops {
		names = append(names, name)
	}
	return names
}

type config struct {
	host            string
	logLevel        string
	operation       string
	accountPrivKeys []string
	accountKeys     []*ethereum.SignKeys
	nvotes          int
	parallelCount   int
	faucet          string
	faucetAuthToken string
	timeout         time.Duration
}

func main() {
	// Report the version before loading the config or logger init, just in case something goes wrong.
	// For the sake of including the version in the log, it's also included in a log line later on.
	fmt.Fprintf(os.Stderr, "vocdoni version %q\n", internal.Version)

	c := &config{}
	flag.StringVar(&c.host, "host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	flag.StringVar(&c.logLevel, "logLevel", "info", "log level (debug, info, warn, error, fatal)")
	flag.StringVar(&c.operation, "operation", "",
		fmt.Sprintf("set operation mode: %v", opNames()))
	flag.StringSliceVarP(&c.accountPrivKeys, "accountPrivKey", "k", []string{},
		"account private key (optional)")
	flag.IntVar(&c.nvotes, "votes", 10, "number of votes to cast")
	flag.IntVar(&c.parallelCount, "parallel", 4, "number of parallel requests")
	flag.StringVar(&c.faucet, "faucet", "",
		"faucet URL for fetching tokens (if empty, default faucet URL will be used)")
	flag.StringVar(&c.faucetAuthToken, "faucetAuthToken", "",
		"(optional) token passed as Bearer when fetching faucetURL")
	flag.DurationVar(&c.timeout, "timeout", apiclient.WaitTimeout, "timeout duration of each step")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSome examples of different operation modes:\n")
		for name, op := range ops {
			fmt.Fprintf(os.Stderr, "### %s\n", name)
			fmt.Fprintf(os.Stderr, "\t"+op.description+"\n")
			fmt.Fprintf(os.Stderr, op.example+"\n")
			fmt.Fprintf(os.Stderr, "\n")
		}
		fmt.Fprintf(os.Stderr, "If the network is deployed locally using the docker test suite, ")
		fmt.Fprintf(os.Stderr, "the faucet URL might be configured as `--faucet=http://localhost:9090/v2/open/claim`\n")
	}

	flag.CommandLine.SortFlags = false
	flag.Parse()

	log.Init(c.logLevel, "stdout", nil)
	log.Infow("starting "+filepath.Base(os.Args[0]),
		"version", internal.Version,
		"operation", c.operation,
		"host", c.host,
		"parallel", c.parallelCount,
		"timeout", c.timeout,
		"votes", c.nvotes)

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

	op, found := ops[c.operation]
	if !found {
		log.Fatal("no valid operation mode specified")
	}

	api, err := NewAPIclient(c.host)
	if err != nil {
		log.Fatal(err)
	}

	startTime := time.Now()
	err = op.test.Setup(api, c)
	duration := time.Since(startTime)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("setup done", "test", c.operation, "duration", duration.String())

	startTime = time.Now()
	err = op.test.Run()
	duration = time.Since(startTime)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("run finished", "test", c.operation, "duration", duration.String())

	startTime = time.Now()
	err = op.test.Teardown()
	duration = time.Since(startTime)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("teardown done", "test", c.operation, "duration", duration.String())
}

// NewAPIclient connects to the API host and returns the handle
func NewAPIclient(host string) (*apiclient.HTTPclient, error) {
	hostURL, err := url.Parse(host)
	if err != nil {
		log.Fatal(err)
	}

	log.Debugf("connecting to %s", hostURL.String())

	token := uuid.New()
	return apiclient.NewHTTPclient(hostURL, &token)
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
