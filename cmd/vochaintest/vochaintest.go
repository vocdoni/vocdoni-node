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
	procDuration := flag.Int("processDuration", 5, "voting process duration in blocks")
	gateways := flag.StringSlice("gwExtra", []string{}, "list of extra gateways to be used in addition to gwHost for sending votes")

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
		vtest(*host, *oraclePrivKey, *electionType, &entityKey, *electionSize, *procDuration, *parallelCons, *gateways)
	case "censusImport":
		censusImport(*host, &entityKey)
	default:
		log.Warnf("no valid operation mode specified")
	}
}

func censusImport(host string, signer *ethereum.SignKeys) {
	con, err := client.NewAPIConnection(host, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer con.Conn.Close(websocket.StatusNormalClosure, "")
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
			panic(err)
		}
		if len(line) < ethereum.PubKeyLength || strings.HasPrefix(string(line), "#") {
			continue
		}
		pubk = strings.Trim(string(line), "")
		log.Infof("[%d] imported key %s", i, pubk)
		keys = append(keys, pubk)
		i++
	}

	root, uri := con.CreateCensus(signer, nil, keys)
	log.Infof("Census created and published\nRoot: %s\nURI: %s", root, uri)

}

func vtest(host, oraclePrivKey, electionType string, entityKey *ethereum.SignKeys, electionSize, procDuration, parallelCons int, gateways []string) {
	censusKeys := client.CreateEthRandomKeysBatch(electionSize)
	oracleKey := ethereum.SignKeys{}
	if err := oracleKey.AddHexKey(oraclePrivKey); err != nil {
		log.Fatal(err)
	}

	// Create census
	log.Infof("connecting to main gateway %s", host)

	var conns []*client.APIConnection

	// Add the first connection, this will be the main connection
	var mainCon *client.APIConnection
	var err error
	for tries := 13; tries > 0; tries-- {
		mainCon, err = client.NewAPIConnection(host, 0)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer mainCon.Conn.Close(websocket.StatusNormalClosure, "")

	censusRoot, censusURI := mainCon.CreateCensus(entityKey, censusKeys, nil)
	log.Infof("creaed census %s of size %d", censusRoot, len(censusKeys))

	// Create process
	pid := client.RandomHex(32)
	log.Infof("creating process with entityID: %s", entityKey.EthAddrString())
	start, err := mainCon.CreateProcess(&oracleKey, entityKey.EthAddrString(), censusRoot, censusURI, pid, electionType, procDuration)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("created process with ID: %s", pid)
	encrypted := types.ProcessIsEncrypted[electionType]

	// Create the websockets connections for sending the votes
	gwList := append(gateways, host)

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cgw, err := client.NewAPIConnection(gwList[i%len(gwList)], i+1)
		if err != nil {
			log.Warn(err)
			continue
		}
		defer cgw.Conn.Close(websocket.StatusNormalClosure, "")
		conns = append(conns, cgw)
	}

	// Make sure all gateways have the census
	gwsWithCensus := []string{mainCon.Addr}
	for _, con := range conns {
		found := false
		for _, gw := range gwsWithCensus {
			if gw == con.Addr {
				found = true
				break
			}
		}
		if !found {
			log.Infof("importing census to gateway %s", con.Addr)
			gwsWithCensus = append(gwsWithCensus, con.Addr)
			if root, err := con.ImportCensus(entityKey, censusURI); err != nil {
				log.Fatal(err)
			} else {
				if root != censusRoot {
					log.Fatalf("imported census root does not match (%s != %s)", root, censusRoot)
				}
			}
		}
	}

	// Send votes
	i := 0
	p := len(censusKeys) / len(conns)
	var wg sync.WaitGroup
	votingTimes := make([]time.Duration, len(conns))
	for gw, con := range conns {
		signers := make([]*ethereum.SignKeys, p)
		copy(signers[:], censusKeys[i:i+p])
		log.Infof("voters from %d to %d will be sent to %s", i, i+p-1, con.Addr)
		gw, con := gw, con
		wg.Add(1)
		go func() {
			defer wg.Done()
			if votingTimes[gw], err = con.SendVotes(pid, entityKey.EthAddrString(), censusRoot, start, int64(procDuration), signers, encrypted); err != nil {
				log.Fatal(err)
			}
			log.Infof("gateway %d has ended its job", con.ID)
		}()
		i += p
	}

	// Wait until all votes sent and check the results
	wg.Wait()
	if false {
		log.Infof("canceling process in order to fetch the results")
		if err := mainCon.CancelProcess(&oracleKey, pid); err != nil {
			log.Fatal(err)
		}
	}
	maxVotingTime := time.Duration(0)
	for _, t := range votingTimes {
		if t > maxVotingTime {
			maxVotingTime = t
		}
	}
	log.Infof("the voting process took %s", maxVotingTime)
	log.Infof("checking results....")
	if r, err := mainCon.Results(pid, len(censusKeys), start, int64(procDuration)); err != nil {
		log.Fatal(err)
	} else {
		log.Infof("results: %+v", r)
	}
	log.Infof("all done!")
}
