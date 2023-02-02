package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

type operation struct {
	name, description, example string
}

var ops = []operation{
	{
		name:        "vtest",
		description: "Performs a complete test, from creating a census to voting and validating votes",
		example: os.Args[0] + " --operation=vtest --electionSize=1000 " +
			"--oracleKey=6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223",
	},
	{
		name:        "tokentransactions",
		description: "Tests all token related transactions",
		example: os.Args[0] + " --operation=tokentransactions " +
			"--host http://127.0.0.1:9090/v2",
	},
}

func opNames() (names []string) {
	for _, op := range ops {
		names = append(names, op.name)
	}
	return names
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	circuit.SetBaseDir(filepath.Join(home, ".vochain", *circuit.BaseDir))

	host := flag.String("host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	logLevel := flag.String("logLevel", "info", "log level (debug, info, warn, error, fatal)")
	operation := flag.String("operation", "vtest", fmt.Sprintf("set operation mode: %v", opNames()))
	accountPrivKeys := flag.StringSliceP("accountPrivKey", "k", []string{}, "account private key (optional)")
	treasurerPrivKey := flag.String("treasurerPrivKey", "", "treasurer private key")
	nvotes := flag.Int("votes", 10, "number of votes to cast")
	parallelCount := flag.Int("parallel", 4, "number of parallel requests")
	faucet := flag.String("faucet", "dev", "faucet URL for fetching tokens (special keyword 'dev' translates into hardcoded URL for dev faucet)")
	faucetAuthToken := flag.String("faucetAuthToken", "", "(optional) token passed as Bearer when fetching faucetURL")
	timeout := flag.Duration("timeout", 5*time.Minute, "timeout duration")
	// dataDir := flag.String("dataDir", home+"/.vocdoni", "")

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

	log.Init(*logLevel, "stdout")

	rand.Seed(time.Now().UnixNano())

	if len(*accountPrivKeys) == 0 {
		accountPrivKeys = &[]string{util.RandomHex(32)}
		log.Infof("new account generated, private key is %s", *accountPrivKeys)
	}

	accountKeys := make([]*ethereum.SignKeys, len(*accountPrivKeys))
	for i, key := range *accountPrivKeys {
		ak, err := privKeyToSigner(key)
		if err != nil {
			log.Fatal(err)
		}
		accountKeys[i] = ak
	}

	switch *operation {
	case "vtest":
		accountPrivateKey := hex.EncodeToString(accountKeys[0].PrivateKey())
		mkTreeVoteTest(*host,
			accountPrivateKey,
			*nvotes,
			*parallelCount,
			*faucet,
			*faucetAuthToken,
			*timeout)
	case "anonvoting":
		accountPrivateKey := hex.EncodeToString(accountKeys[0].PrivateKey())
		mkTreeAnonVoteTest(*host,
			accountPrivateKey,
			*nvotes,
			*parallelCount,
			*faucet,
			*faucetAuthToken,
			*timeout)
	case "tokentransactions":
		accountPrivateKey := hex.EncodeToString(accountKeys[0].PrivateKey())
		testTokenTransactions(*host,
			*treasurerPrivKey,
			accountPrivateKey)
	default:
		log.Fatal("no valid operation mode specified")
	}
}
