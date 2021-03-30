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

	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

var ops = map[string]bool{
	"vtest":          true,
	"cspvoting":      true,
	"censusImport":   true,
	"censusGenerate": true,
}

func opsAvailable() (opsav []string) {
	for k := range ops {
		opsav = append(opsav, k)
	}
	return opsav
}

func main() {
	// starting test

	loglevel := flag.String("logLevel", "info", "log level")
	opmode := flag.String("operation", "vtest", fmt.Sprintf("set operation mode: %v", opsAvailable()))
	oraclePrivKey := flag.String("oracleKey", "", "hexadecimal oracle private key")
	entityPrivKey := flag.String("entityKey", "", "hexadecimal entity private key")
	host := flag.String("gwHost", "http://127.0.0.1:9090/dvote", "gateway websockets endpoint")
	electionType := flag.String("electionType", "encrypted-poll", "encrypted-poll or poll-vote")
	electionSize := flag.Int("electionSize", 100, "election census size")
	parallelCons := flag.Int("parallelCons", 1, "parallel API connections")
	procDuration := flag.Int("processDuration", 500, "voting process duration in blocks")
	doubleVote := flag.Bool("doubleVote", true, "send every vote twice")
	checkNullifiers := flag.Bool("checkNullifiers", false, "check all votes are correct one by one (slow)")
	gateways := flag.StringSlice("gwExtra", []string{},
		"list of extra gateways to be used in addition to gwHost for sending votes")
	keysfile := flag.String("keysFile", "cache-keys.json", "file to store and reuse keys and census")

	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAvailable operation modes:\n")
		fmt.Fprintf(os.Stderr,
			"=> vtest\n\tPerforms a complete test, from creating a census to voting and validate votes\n")
		fmt.Fprintf(os.Stderr,
			"\t./test --operation=vtest --electionSize=1000 --oracleKey=6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223\n")
		fmt.Fprintf(os.Stderr,
			"=> censusImport\n\tReads from stdin line by line to read a list of hex public keys, creates and publishes the census\n")
		fmt.Fprintf(os.Stderr,
			"\tcat keys.txt | ./test --operation=censusImport --gwHost wss://gw1test.vocdoni.net/dvote\n")
		fmt.Fprintf(os.Stderr,
			"=> censusGenerate\n\tGenerate a list of private/public keys and its merkle Proofs\n")
		fmt.Fprintf(os.Stderr,
			"\t./test --operation=censusGenerate --gwHost wss://gw1test.vocdoni.net/dvote --electionSize=10000 --keysFile=keys.json\n")
	}
	flag.Parse()

	log.Init(*loglevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	// create entity key
	entityKey := ethereum.NewSignKeys()
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
		mkTreeVoteTest(*host,
			*oraclePrivKey,
			*electionType == "encrypted-poll",
			entityKey,
			*electionSize,
			*procDuration,
			*parallelCons,
			*doubleVote,
			*checkNullifiers,
			*gateways,
			*keysfile,
			true,
			false)
	case "cspvoting":
		cspKey := ethereum.NewSignKeys()
		if err := cspKey.Generate(); err != nil {
			log.Fatal(err)
		}
		cspVoteTest(*host,
			*oraclePrivKey,
			*electionType == "encrypted-poll",
			entityKey,
			cspKey,
			*electionSize,
			*procDuration,
			*parallelCons,
			*doubleVote,
			*checkNullifiers,
			*gateways,
		)
	case "censusImport":
		censusImport(*host, entityKey)
	case "censusGenerate":
		censusGenerate(*host, entityKey, *electionSize, *keysfile)
	default:
		log.Fatal("no valid operation mode specified")
	}
}

func censusGenerate(host string, signer *ethereum.SignKeys, size int, filepath string) {
	cl, err := client.New(host)
	if err != nil {
		log.Fatal(err)
	}
	defer cl.Close()
	log.Infof("generating new keys census batch")
	keys := client.CreateEthRandomKeysBatch(size)
	root, uri, err := cl.CreateCensus(signer, keys, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("census created and published\nRoot: %x\nURI: %s", root, uri)
	proofs, err := cl.GetMerkleProofBatch(keys, root, false)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.SaveKeysBatch(filepath, root, uri, keys, proofs); err != nil {
		log.Fatalf("cannot save keys file %s: (%s)", filepath, err)
	}
	log.Infof("keys batch created and saved into %s", filepath)
}

func censusImport(host string, signer *ethereum.SignKeys) {
	var cl *client.Client
	var err error

	// Connect
	for tries := 10; tries > 0; tries-- {
		cl, err = client.New(host)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer cl.Close()

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
		if len(line) < ethereum.PubKeyLengthBytes || strings.HasPrefix(string(line), "#") {
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

func mkTreeVoteTest(host,
	oraclePrivKey string,
	encryptedVotes bool,
	entityKey *ethereum.SignKeys,
	electionSize,
	procDuration,
	parallelCons int,
	doubleVote, checkNullifiers bool,
	gateways []string,
	keysfile string,
	useLastCensus bool,
	forceGatewaysGotCensus bool) {

	var censusKeys []*ethereum.SignKeys
	var proofs [][]byte
	var err error
	var censusRoot []byte
	censusURI := ""

	oracleKey := ethereum.NewSignKeys()
	if err := oracleKey.AddHexKey(oraclePrivKey); err != nil {
		log.Fatal(err)
	}

	// Try to reuse previous census and keys
	censusKeys, proofs, censusRoot, censusURI, err = client.LoadKeysBatch(keysfile)
	if err != nil || len(censusKeys) < electionSize || len(proofs) < electionSize {
		censusGenerate(host, entityKey, electionSize, keysfile)
		censusKeys, proofs, censusRoot, censusURI, err = client.LoadKeysBatch(keysfile)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Infof("loaded cache census %x with size %d", censusRoot, len(censusKeys))
		if len(censusKeys) > electionSize {
			log.Infof("truncating census from %d to %d", len(censusKeys), electionSize)
			censusKeys = censusKeys[:electionSize]
			proofs = proofs[:electionSize]
		}
	}

	log.Infof("connecting to main gateway %s", host)
	// Add the first connection, this will be the main connection
	var mainClient *client.Client
	var clients []*client.Client

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
	defer mainClient.Close()

	// Create process
	pid := client.Random(32)
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, err := mainClient.CreateProcess(
		oracleKey,
		entityKey.Address().Bytes(),
		censusRoot,
		censusURI,
		pid, &models.EnvelopeType{EncryptedVotes: encryptedVotes},
		models.CensusOrigin_OFF_CHAIN_TREE,
		procDuration)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("created process with ID: %x", pid)
	// Create the websockets connections for sending the votes
	gwList := append(gateways, host)

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cl, err := client.New(gwList[i%len(gwList)])
		if err != nil {
			log.Warn(err)
			continue
		}
		defer cl.Close()
		clients = append(clients, cl)
	}

	if forceGatewaysGotCensus {
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
						if size < int64(electionSize) {
							log.Fatalf("gateway %s has an incorrect census size (got:%d expected%d)",
								cl.Addr, size, electionSize)
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
	}

	// Send votes
	i := 0
	p := len(censusKeys) / len(clients)
	var wg sync.WaitGroup
	var proofsReadyWG sync.WaitGroup
	votingTimes := make([]time.Duration, len(clients))
	wg.Add(len(clients))
	proofsReadyWG.Add(len(clients))

	for gw, cl := range clients {
		var gwSigners []*ethereum.SignKeys
		var gwProofs [][]byte
		// Split the voters
		if len(clients) == gw+1 {
			// if last client, add all remaining keys
			gwSigners = make([]*ethereum.SignKeys, len(censusKeys)-i)
			copy(gwSigners, censusKeys[i:])
			gwProofs = make([][]byte, len(censusKeys)-i)
			copy(gwProofs, proofs[i:])
		} else {
			gwSigners = make([]*ethereum.SignKeys, p)
			copy(gwSigners, censusKeys[i:i+p])
			gwProofs = make([][]byte, p)
			copy(gwProofs, proofs[i:i+p])
		}
		log.Infof("%s will receive %d votes", cl.Addr, len(gwSigners))
		gw, cl := gw, cl
		go func() {
			defer wg.Done()
			if votingTimes[gw], err = cl.TestSendVotes(pid,
				entityKey.Address().Bytes(),
				censusRoot,
				start,
				gwSigners,
				models.CensusOrigin_OFF_CHAIN_TREE,
				nil,
				gwProofs,
				encryptedVotes,
				doubleVote,
				checkNullifiers,
				&proofsReadyWG); err != nil {
				log.Fatalf("[%s] %s", cl.Addr, err)
			}
			log.Infof("gateway %d %s has ended its job", gw, cl.Addr)
		}()
		i += p
	}

	// Wait until all votes sent and check the results
	wg.Wait()

	log.Infof("ending process in order to fetch the results")
	if err := mainClient.EndProcess(oracleKey, pid); err != nil {
		log.Fatal(err)
	}
	maxVotingTime := time.Duration(0)
	for _, t := range votingTimes {
		if t > maxVotingTime {
			maxVotingTime = t
		}
	}
	log.Infof("the TOTAL voting process took %s", maxVotingTime)
	log.Infof("checking results....")
	if r, err := mainClient.TestResults(pid, len(censusKeys)); err != nil {
		log.Fatal(err)
	} else {
		log.Infof("results: %+v", r)
	}
	log.Infof("all done!")
}

func cspVoteTest(
	host,
	oraclePrivKey string,
	encryptedVotes bool,
	entityKey,
	cspKey *ethereum.SignKeys,
	electionSize,
	procDuration,
	parallelCons int,
	doubleVote, checkNullifiers bool,
	gateways []string,
) {

	var voters []*ethereum.SignKeys
	var err error

	log.Infof("generating signing keys")
	for i := 0; i < electionSize; i++ {
		voters = append(voters, ethereum.NewSignKeys())
		if err := voters[i].Generate(); err != nil {
			log.Fatal(err)
		}
	}

	oracleKey := ethereum.NewSignKeys()
	if err := oracleKey.AddHexKey(oraclePrivKey); err != nil {
		log.Fatal(err)
	}

	log.Infof("connecting to main gateway %s", host)
	// Add the first connection, this will be the main connection
	var mainClient *client.Client
	var clients []*client.Client

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
	defer mainClient.Close()

	// Create process
	pid := client.Random(32)
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, err := mainClient.CreateProcess(
		oracleKey,
		entityKey.Address().Bytes(),
		cspKey.PublicKey(),
		"https://dumycsp.foo",
		pid, &models.EnvelopeType{EncryptedVotes: encryptedVotes},
		models.CensusOrigin_OFF_CHAIN_CA,
		procDuration)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("created process with ID: %x", pid)
	// Create the websockets connections for sending the votes
	gwList := append(gateways, host)

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cl, err := client.New(gwList[i%len(gwList)])
		if err != nil {
			log.Warn(err)
			continue
		}
		defer cl.Close()
		clients = append(clients, cl)
	}

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cl, err := client.New(gwList[i%len(gwList)])
		if err != nil {
			log.Warn(err)
			continue
		}
		defer cl.Close()
		clients = append(clients, cl)
	}

	// Send votes
	i := 0
	p := len(voters) / len(clients)
	var wg sync.WaitGroup
	var proofsReadyWG sync.WaitGroup
	votingTimes := make([]time.Duration, len(clients))
	wg.Add(len(clients))
	proofsReadyWG.Add(len(clients))

	for gw, cl := range clients {
		var gwSigners []*ethereum.SignKeys
		// Split the voters
		if len(clients) == gw+1 {
			// if last client, add all remaining keys
			gwSigners = make([]*ethereum.SignKeys, len(voters)-i)
			copy(gwSigners, voters[i:])
		} else {
			gwSigners = make([]*ethereum.SignKeys, p)
			copy(gwSigners, voters[i:i+p])
		}
		log.Infof("%s will receive %d votes", cl.Addr, len(gwSigners))
		gw, cl := gw, cl
		go func() {
			defer wg.Done()
			if votingTimes[gw], err = cl.TestSendVotes(
				pid,
				entityKey.Address().Bytes(),
				cspKey.PublicKey(),
				start,
				gwSigners,
				models.CensusOrigin_OFF_CHAIN_CA,
				cspKey,
				nil,
				encryptedVotes,
				doubleVote,
				checkNullifiers,
				&proofsReadyWG); err != nil {
				log.Fatalf("[%s] %s", cl.Addr, err)
			}
			log.Infof("gateway %d %s has ended its job", gw, cl.Addr)
		}()
		i += p
	}

	// Wait until all votes sent and check the results
	wg.Wait()

	log.Infof("ending process in order to fetch the results")
	if err := mainClient.EndProcess(oracleKey, pid); err != nil {
		log.Fatal(err)
	}
	maxVotingTime := time.Duration(0)
	for _, t := range votingTimes {
		if t > maxVotingTime {
			maxVotingTime = t
		}
	}
	log.Infof("the TOTAL voting process took %s", maxVotingTime)
	log.Infof("checking results....")
	if r, err := mainClient.TestResults(pid, len(voters)); err != nil {
		log.Fatal(err)
	} else {
		log.Infof("results: %+v", r)
	}
	log.Infof("all done!")
}
