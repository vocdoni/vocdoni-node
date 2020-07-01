package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	flag "github.com/spf13/pflag"
	"nhooyr.io/websocket"

	"gitlab.com/vocdoni/go-dvote/client"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

func main() {
	// starting test

	loglevel := flag.String("logLevel", "info", "log level")
	opmode := flag.String("operation", "vtest", "set operation mode")
	oraclePrivKey := flag.String("oracleKey", "", "hexadecimal oracle private key")
	entityPrivKey := flag.String("entityKey", "", "hexadecimal entity private key")
	host := flag.String("gwHost", "ws://127.0.0.1:9090/dvote", "gateway websockets endpoint")
	electionType := flag.String("electionType", "encrypted-poll", "encrypted-poll or poll-vote")
	electionSize := flag.Int("electionSize", 100, "election census size")
	parallelCons := flag.Int("parallelCons", 1, "parallel API connections")
	procDuration := flag.Int("processDuration", 500, "voting process duration in blocks")
	doubleVote := flag.Bool("doubleVote", true, "send every vote twice")
	gateways := flag.StringSlice("gwExtra", []string{}, "list of extra gateways to be used in addition to gwHost for sending votes")
	keysfile := flag.String("keysFile", "cache-keys.json", "file to store and reuse keys and census")

	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAvailable operation modes:\n")
		fmt.Fprintf(os.Stderr, "=> vtest\n\tPerforms a complete test, from creating a census to voting and validate votes\n")
		fmt.Fprintf(os.Stderr, "\t./test --operation=vtest --electionSize=1000 --oracleKey=6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223\n")
		fmt.Fprintf(os.Stderr, "=> censusImport\n\tReads from stdin line by line to read a list of hex public keys, creates and publishes the census\n")
		fmt.Fprintf(os.Stderr, "\tcat keys.txt | ./test --operation=censusImport --gwHost wss://gw1test.vocdoni.net/dvote\n")
	}
	flag.Parse()

	log.Init(*loglevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	// create entity key
	entityKey := ethereum.SignKeys{}
	if len(*entityPrivKey) > 0 {
		if err := entityKey.AddHexKey(*entityPrivKey); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := entityKey.Generate(); err != nil {
			log.Fatal(err)
		}
	}

	switch *opmode {
	case "vtest":
		vtest(*host, *oraclePrivKey, *electionType, &entityKey, *electionSize, *procDuration, *parallelCons, *doubleVote, *gateways, *keysfile, true)
	case "censusImport":
		censusImport(*host, &entityKey)
	default:
		log.Warnf("no valid operation mode specified")
	}
}

func censusImport(host string, signer *ethereum.SignKeys) {
	cl, err := client.New(host)
	if err != nil {
		log.Fatal(err)
	}
	defer cl.Conn.Close(websocket.StatusNormalClosure, "")
	var keys []string

	reader := bufio.NewReader(os.Stdin)
	i := 0
	var pubk string
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if len(line) < ethereum.PubKeyLength || strings.HasPrefix(string(line), "#") {
			continue
		}
		pubk = strings.Trim(string(line), "")
		log.Infof("[%d] imported key %s", i, pubk)
		keys = append(keys, pubk)
		i++
	}

	root, uri, err := cl.CreateCensus(signer, nil, keys)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Census created and published\nRoot: %s\nURI: %s", root, uri)

}

func vtest(host, oraclePrivKey, electionType string, entityKey *ethereum.SignKeys, electionSize, procDuration,
	parallelCons int, doubleVote bool, gateways []string, keysfile string, useLastCensus bool) {

	var censusKeys []*ethereum.SignKeys
	var err error
	censusRoot := ""
	censusURI := ""
	isCensusNew := false
	censusKeys, censusRoot, censusURI, err = client.LoadKeysBatch(keysfile)
	if err != nil || len(censusKeys) != electionSize {
		log.Infof("generating new keys census batch")
		censusKeys = client.CreateEthRandomKeysBatch(electionSize)
		isCensusNew = true
	} else {
		log.Infof("loaded cache census %s with size %d", censusRoot, len(censusKeys))
	}

	oracleKey := ethereum.SignKeys{}
	if err := oracleKey.AddHexKey(oraclePrivKey); err != nil {
		log.Fatal(err)
	}

	// Create census
	log.Infof("connecting to main gateway %s", host)

	var clients []*client.Client

	// Add the first connection, this will be the main connection
	var mainClient *client.Client

	for tries := 10; tries > 0; tries-- {
		mainClient, err = client.New(host)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer mainClient.Conn.Close(websocket.StatusNormalClosure, "")

	if !useLastCensus || isCensusNew {
		censusRoot, censusURI, err = mainClient.CreateCensus(entityKey, censusKeys, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("created new census %s of size %d", censusRoot, len(censusKeys))
	}
	// Save census to a cache file to be reused in the future
	if keysfile != "" && isCensusNew {
		log.Infof("saving keys batch and census to %s", keysfile)
		if err := client.SaveKeysBatch(keysfile, censusRoot, censusURI, censusKeys); err != nil {
			log.Warnf("could not save keys batch: (%s)", err)
		}
	}

	// Create process
	pid := client.RandomHex(32)
	log.Infof("creating process with entityID: %s", entityKey.EthAddrString())
	start, err := mainClient.CreateProcess(&oracleKey, entityKey.EthAddrString(), censusRoot, censusURI, pid, electionType, procDuration)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("created process with ID: %s", pid)
	encrypted := types.ProcessIsEncrypted[electionType]

	// Create the websockets connections for sending the votes
	gwList := append(gateways, host)

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cl, err := client.New(gwList[i%len(gwList)])
		if err != nil {
			log.Warn(err)
			continue
		}
		defer cl.Conn.Close(websocket.StatusNormalClosure, "")
		clients = append(clients, cl)
	}

	// Make sure all gateways have the census
	log.Infof("waiting for gateways to import the census")

	workingGateways := make(map[string]bool)
	for _, cl := range clients {
		workingGateways[cl.Addr] = true
	}

	gatewaysWithCensus := make(map[string]bool)
	for len(gatewaysWithCensus) < len(workingGateways) {
		for _, cl := range clients {
			if _, ok := gatewaysWithCensus[cl.Addr]; !ok {
				if size, err := cl.CensusSize(censusRoot); err == nil {
					if size != int64(electionSize) {
						log.Fatalf("gateway %s has an incorrect census size (got:%d expected%d)", cl.Addr, size, electionSize)
					}
					log.Infof("gateway %s got the census!", cl.Addr)
					gatewaysWithCensus[cl.Addr] = true
				} else {
					log.Debug(err)
				}
			}
			time.Sleep(time.Second * 2)
		}
	}

	log.Infof("all gateways got the census! let's start voting")
	// Send votes
	i := 0
	p := len(censusKeys) / len(clients)
	var wg sync.WaitGroup
	var proofsReadyWG sync.WaitGroup
	votingTimes := make([]time.Duration, len(clients))

	for gw, cl := range clients {
		var signers []*ethereum.SignKeys
		if len(clients) == gw+1 {
			// if last client, add all remaining keys
			signers = make([]*ethereum.SignKeys, len(censusKeys)-i)
			copy(signers[:], censusKeys[i:])
		} else {
			signers = make([]*ethereum.SignKeys, p)
			copy(signers[:], censusKeys[i:i+p])
		}
		log.Infof("%s will receive %d votes", cl.Addr, len(signers))
		gw, cl := gw, cl
		wg.Add(1)
		proofsReadyWG.Add(1)
		go func() {
			defer wg.Done()
			if votingTimes[gw], err = cl.TestSendVotes(pid, entityKey.EthAddrString(), censusRoot, start, signers, encrypted, doubleVote, &proofsReadyWG); err != nil {
				log.Fatalf("[%s] %s", cl.Addr, err)
			}
			log.Infof("gateway %d %s has ended its job", gw, cl.Addr)
		}()
		i += p
	}

	// Wait until all votes sent and check the results
	log.Info("all vote work started, waiting for gateways to finalize")
	wg.Wait()

	log.Infof("canceling process in order to fetch the results")
	if err := mainClient.CancelProcess(&oracleKey, pid); err != nil {
		log.Fatal(err)
	}
	maxVotingTime := time.Duration(0)
	for _, t := range votingTimes {
		if t > maxVotingTime {
			maxVotingTime = t
		}
	}
	log.Infof("the voting process took %s", maxVotingTime)
	log.Infof("checking results....")
	if r, err := mainClient.TestResults(pid, len(censusKeys)); err != nil {
		log.Fatal(err)
	} else {
		log.Infof("results: %+v", r)
	}
	log.Infof("all done!")
}
