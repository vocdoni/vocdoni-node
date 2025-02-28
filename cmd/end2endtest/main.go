package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

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

// specifying the flag --operation all will allow to run all tests available in
// the ops map
const allTests = "all"

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
	testFunc             func() VochainTest
	description, example string
}

var ops = map[string]operation{}

func opNames() []string {
	names := []string{allTests}
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
	parallelTests   int
	runs            int
	faucet          string
	faucetAuthToken string
	timeout         time.Duration
}

// Clone creates a deep copy of the config object.
func (c *config) Clone() *config {
	// Create a new config instance
	cloned := config{
		host:            c.host,
		logLevel:        c.logLevel,
		operation:       c.operation,
		accountPrivKeys: make([]string, len(c.accountPrivKeys)),
		accountKeys:     make([]*ethereum.SignKeys, len(c.accountKeys)),
		nvotes:          c.nvotes,
		parallelCount:   c.parallelCount,
		parallelTests:   c.parallelTests,
		runs:            c.runs,
		faucet:          c.faucet,
		faucetAuthToken: c.faucetAuthToken,
		timeout:         c.timeout,
	}

	// Copy the slice of strings
	copy(cloned.accountPrivKeys, c.accountPrivKeys)

	// Recreate each of SignKeys
	for i, key := range c.accountKeys {
		newKey := ethereum.SignKeys{}
		if err := newKey.AddHexKey(hex.EncodeToString(key.PrivateKey())); err != nil {
			log.Fatal("could not clone account private key")
		}
		cloned.accountKeys[i] = &newKey
	}

	return &cloned
}

func parseFlags(c *config) {
	flag.StringVar(&c.host, "host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	flag.StringVar(&c.logLevel, "logLevel", "info", "log level (debug, info, warn, error, fatal)")
	flag.StringVar(&c.operation, "operation", "", fmt.Sprintf("set operation mode: %v", opNames()))
	flag.StringSliceVarP(&c.accountPrivKeys, "accountPrivKey", "k", []string{}, "account private key (optional)")
	flag.IntVar(&c.nvotes, "votes", 10, "number of votes to cast")
	flag.IntVar(&c.parallelCount, "parallel", 4, "number of parallel requests")
	flag.StringVar(&c.faucet, "faucet", "dev", "faucet URL for fetching tokens (special keyword 'dev' translates into hardcoded URL for dev faucet)")
	flag.StringVar(&c.faucetAuthToken, "faucetAuthToken", "", "(optional) token passed as Bearer when fetching faucetURL")
	flag.DurationVar(&c.timeout, "timeout", apiclient.WaitTimeout*2, "timeout duration to wait for operations to complete")
	flag.IntVar(&c.parallelTests, "parallelTests", 1, "number of parallel tests to run (of the same type specified in --operation)")
	flag.IntVar(&c.runs, "runs", 1, "number of tests to run (of the same type specified in --operation)")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage of ", os.Args[0], ":\n")
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, "\nSome examples of different operation modes:\n")
		for name, op := range ops {
			fmt.Fprint(os.Stderr, "### ", name, "\n")
			fmt.Fprint(os.Stderr, "\t", op.description, "\n")
			fmt.Fprint(os.Stderr, op.example, "\n")
			fmt.Fprint(os.Stderr, "\n")
		}
		fmt.Fprint(os.Stderr, "If the network is deployed locally using the docker test suite, ")
		fmt.Fprint(os.Stderr, "the faucet URL might be configured as `--faucet=http://localhost:9090/v2/open/claim`\n")
	}

	flag.CommandLine.SortFlags = false
	flag.Parse()
}

func initializeLogger(c *config) {
	log.Init(c.logLevel, "stdout", nil)
	log.Infow("starting "+filepath.Base(os.Args[0]),
		"version", internal.Version,
		"operation", c.operation,
		"host", c.host,
		"parallel", c.parallelCount,
		"timeout", c.timeout,
		"votes", c.nvotes)
}

func createSignKeys(c *config) error {
	c.accountKeys = make([]*ethereum.SignKeys, len(c.accountPrivKeys))
	for i, key := range c.accountPrivKeys {
		ak, err := privKeyToSigner(key)
		if err != nil {
			return err
		}
		c.accountKeys[i] = ak
		log.Info("privkey ", fmt.Sprintf("%x", ak.PrivateKey()), " = account ", ak.AddressString())
	}
	return nil
}

func setupAndRun(op operation, api *apiclient.HTTPclient, c *config) error {
	log.Infow("starting setup", "test", c.operation)
	startTime := time.Now()
	testFunc := op.testFunc()
	err := testFunc.Setup(api, c)
	duration := time.Since(startTime)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("setup done", "test", c.operation, "duration", duration.String())

	startTime = time.Now()
	err = testFunc.Run()
	duration = time.Since(startTime)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("run finished", "test", c.operation, "duration", duration.String())

	startTime = time.Now()
	err = testFunc.Teardown()
	duration = time.Since(startTime)
	if err != nil {
		return err
	}
	log.Infow("teardown done", "test", c.operation, "duration", duration.String())

	return nil
}

func privKeyToSigner(key string) (*ethereum.SignKeys, error) {
	var skey *ethereum.SignKeys
	if len(key) > 0 {
		skey = ethereum.NewSignKeys()
		if err := skey.AddHexKey(key); err != nil {
			return nil, fmt.Errorf("cannot create key %s: %v", key, err)
		}
	}
	return skey, nil
}

func main() {
	fmt.Fprint(os.Stderr, "vocdoni version \"", internal.Version, "\"\n")

	mainConfig := &config{}
	parseFlags(mainConfig)
	initializeLogger(mainConfig)

	createAccount := func(c *config) (*apiclient.HTTPclient, error) {
		if len(c.accountPrivKeys) == 0 {
			c.accountPrivKeys = []string{util.RandomHex(32)}
			log.Info("no keys passed, generated random private key: ", c.accountPrivKeys)
		}
		if err := createSignKeys(c); err != nil {
			log.Fatal(err)
		}
		return apiclient.New(c.host)
	}

	runTests := func(op operation, c *config, wg *sync.WaitGroup) {
		defer wg.Done()
		api, err := createAccount(c)
		if err != nil {
			log.Fatal("could not create account: ", err)
		}
		if err := setupAndRun(op, api, c); err != nil {
			log.Fatal(err)
		}
	}

	for i := 0; i < mainConfig.runs; i++ {
		wg := &sync.WaitGroup{}
		if mainConfig.operation == allTests {
			for _, op := range ops {
				wg.Add(1)
				c := mainConfig.Clone()
				go runTests(op, c, wg)
			}
		} else {
			op, found := ops[mainConfig.operation]
			if !found {
				log.Fatal("no valid operation mode specified")
			}
			for j := 0; j < mainConfig.parallelTests; j++ {
				wg.Add(1)
				c := mainConfig.Clone()
				log.Infow("starting test", "number", j, "test", c.operation)
				go runTests(op, c, wg)
			}
		}
		wg.Wait()
	}
}
