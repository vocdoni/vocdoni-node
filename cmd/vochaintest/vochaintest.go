package main

import (
	"bufio"
	"errors"
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
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

var ops = map[string]bool{
	"vtest":          true,
	"cspvoting":      true,
	"anonvoting":     true,
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
	withWeight := flag.Uint64("withWeight", 1, "vote with weight (same for all voters)")
	// TODO: Set to default = true once #333 is resolved
	doublePreRegister := flag.Bool("doublePreRegister", true, "send every pre-register twice")
	checkNullifiers := flag.Bool("checkNullifiers", false,
		"check that all votes are correct one-by-one (slow)")
	gateways := flag.StringSlice("gwExtra", []string{},
		"list of extra gateways to be used in addition to gwHost for sending votes")
	keysfile := flag.String("keysFile", "cache-keys.json", "file to store and reuse keys and census")

	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAvailable operation modes:\n")
		fmt.Fprintf(os.Stderr,
			"=> vtest\n\tPerforms a complete test, from creating a census to voting and validating votes\n")
		fmt.Fprintf(os.Stderr,
			"\t./test --operation=vtest --electionSize=1000 "+
				"--oracleKey=6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223\n")
		fmt.Fprintf(os.Stderr,
			"=> censusImport\n\tReads from stdin line by line"+
				" to read a list of hex public keys, creates and publishes the census\n")
		fmt.Fprintf(os.Stderr,
			"\tcat keys.txt | ./test --operation=censusImport "+
				"--gwHost wss://gw1test.vocdoni.net/dvote\n")
		fmt.Fprintf(os.Stderr,
			"=> censusGenerate\n\tGenerate a list of "+
				"private/public keys and their merkle Proofs\n")
		fmt.Fprintf(os.Stderr,
			"\t./test --operation=censusGenerate --gwHost "+
				"wss://gw1test.vocdoni.net/dvote --electionSize=10000 --keysFile=keys.json\n")
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
	case "anonvoting":
		mkTreeAnonVoteTest(*host,
			*oraclePrivKey,
			entityKey,
			*electionSize,
			*procDuration,
			*parallelCons,
			*doubleVote,
			*doublePreRegister,
			*checkNullifiers,
			*gateways,
			*keysfile,
			true,
			false)
	case "vtest":
		mkTreeVoteTest(*host,
			*oraclePrivKey,
			*electionType == "encrypted-poll",
			entityKey,
			*electionSize,
			*procDuration,
			*parallelCons,
			*withWeight,
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
		censusGenerate(*host, entityKey, *electionSize, *keysfile, 1)
	case "testtxs":
		// end-user voting is not tested here
		testAllTransactions(*host,
			*oraclePrivKey,
		)
	default:
		log.Fatal("no valid operation mode specified")
	}
}

func censusGenerate(host string, signer *ethereum.SignKeys, size int, filepath string, withWeight uint64) {
	cl, err := client.New(host)
	if err != nil {
		log.Fatal(err)
	}
	defer cl.Close()
	log.Infof("generating new keys census batch")
	keys := client.CreateEthRandomKeysBatch(size)
	weights := []*types.BigInt{}
	log.Infof("creating census with weight == %d", withWeight)
	for i := uint64(1); i <= uint64(size); i++ {
		weights = append(weights, new(types.BigInt).SetUint64(withWeight))
	}

	root, uri, err := cl.CreateCensus(signer, keys, nil, weights)
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
		if errors.Is(err, io.EOF) {
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
	root, uri, err := cl.CreateCensus(signer, nil, keys, nil)
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
	parallelCons int, withWeight uint64,
	doubleVote, checkNullifiers bool,
	gateways []string,
	keysfile string,
	useLastCensus bool,
	forceGatewaysGotCensus bool) {

	var censusKeys []*ethereum.SignKeys
	var proofs []*client.Proof
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
		censusGenerate(host, entityKey, electionSize, keysfile, withWeight)
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

	chainId, err := mainClient.GetChainID()
	if err != nil {
		log.Fatal(err)
	}
	oracleKey.VocdoniChainID = chainId
	entityKey.VocdoniChainID = chainId
	// create and top-up entity account
	if err := mainClient.CreateAccount(entityKey, "ipfs://", 0); err != nil {
		log.Fatal(err)
	}
	treasurer, err := mainClient.GetTreasurer(oracleKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := mainClient.MintTokens(oracleKey, entityKey.Address(), 10000, treasurer.Nonce); err != nil {
		log.Fatal(err)
	}
	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}
	// create and top-up oracle
	mainClient.CreateAccount(oracleKey, "ipfs://", 0)
	mainClient.MintTokens(oracleKey, oracleKey.Address(), 10000, treasurer.Nonce+1)
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}

	// Create process
	pid := client.Random(32)
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, err := mainClient.CreateProcess(
		entityKey,
		entityKey.Address().Bytes(),
		censusRoot,
		censusURI,
		pid,
		&models.EnvelopeType{EncryptedVotes: encryptedVotes},
		nil,
		models.CensusOrigin_OFF_CHAIN_TREE,
		0,
		procDuration,
		uint64(electionSize))
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
							log.Fatalf("gateway %s has an incorrect census size (got: %d expected %d)",
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

		log.Infof("all gateways retrieved the census! let's start voting")
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
		var gwProofs []*client.Proof
		// Split the voters
		if len(clients) == gw+1 {
			// if last client, add all remaining keys
			gwSigners = make([]*ethereum.SignKeys, len(censusKeys)-i)
			copy(gwSigners, censusKeys[i:])
			gwProofs = make([]*client.Proof, len(censusKeys)-i)
			copy(gwProofs, proofs[i:])
		} else {
			gwSigners = make([]*ethereum.SignKeys, p)
			copy(gwSigners, censusKeys[i:i+p])
			gwProofs = make([]*client.Proof, p)
			copy(gwProofs, proofs[i:i+p])
		}
		for _, gws := range gwSigners {
			gws.VocdoniChainID = chainId
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

	log.Infof("waiting for all votes to be validated...")
	timeDeadLine := time.Second * 200
	if electionSize > 1000 {
		timeDeadLine = time.Duration(electionSize/5) * time.Second
	}
	checkStart := time.Now()
	i = 0
	for {
		time.Sleep(time.Millisecond * 1000)
		if h, err := clients[i].GetEnvelopeHeight(pid); err != nil {
			log.Warnf("error getting envelope height: %v", err)
			i++
			if i > len(clients) {
				i = 0
			}
			continue
		} else {
			if h >= uint32(electionSize) {
				break
			}
			log.Infof("validated votes: %d", h)
		}
		if time.Since(checkStart) > timeDeadLine {
			log.Fatal("time deadline reached while waiting for results")
		}
	}

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
	log.Infof("the ENTIRE voting process took %s", maxVotingTime)
	log.Infof("checking results....")
	if r, err := mainClient.TestResults(pid, len(censusKeys), withWeight); err != nil {
		log.Fatal(err)
	} else {
		log.Infof("results: %+v", r)
	}
	log.Infof("all done!")
}

func mkTreeAnonVoteTest(host,
	oraclePrivKey string,
	entityKey *ethereum.SignKeys,
	electionSize,
	procDuration,
	parallelCons int,
	doubleVote, doublePreRegister, checkNullifiers bool,
	gateways []string,
	keysfile string,
	useLastCensus bool,
	forceGatewaysGotCensus bool) {
	// log.Init("debug", "stdout")

	var censusKeys []*ethereum.SignKeys
	var proofs []*client.Proof
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
		censusGenerate(host, entityKey, electionSize, keysfile, 1)
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

	chainId, err := mainClient.GetChainID()
	if err != nil {
		log.Fatal(err)
	}
	oracleKey.VocdoniChainID = chainId
	entityKey.VocdoniChainID = chainId

	// create and top-up entity account
	mainClient.CreateAccount(entityKey, "ipfs://", 0)
	treasurer, err := mainClient.GetTreasurer(oracleKey)
	if err != nil {
		log.Fatal(err)
	}
	mainClient.MintTokens(oracleKey, entityKey.Address(), 10000, treasurer.Nonce)
	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}
	// create and top-up oracle
	mainClient.CreateAccount(oracleKey, "ipfs://", 0)
	mainClient.MintTokens(oracleKey, oracleKey.Address(), 10000, treasurer.Nonce+1)
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}

	// Create process
	pid := client.Random(32)
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, err := mainClient.CreateProcess(
		entityKey,
		entityKey.Address().Bytes(),
		censusRoot,
		censusURI,
		pid,
		&models.EnvelopeType{Anonymous: true},
		&models.ProcessMode{AutoStart: true, Interruptible: true, PreRegister: true},
		models.CensusOrigin_OFF_CHAIN_TREE,
		5,
		procDuration,
		uint64(electionSize))
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
							log.Fatalf("gateway %s has an incorrect census size (got: %d expected %d)",
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

		log.Infof("all gateways retrieved the census! let's start voting")
	}

	// Pre-register keys zkCensusKey
	i := 0
	p := len(censusKeys) / len(clients)
	var wg sync.WaitGroup
	var proofsReadyWG sync.WaitGroup
	regKeyTimes := make([]time.Duration, len(clients))
	wg.Add(len(clients))
	proofsReadyWG.Add(len(clients))

	for gw, cl := range clients {
		var gwSigners []*ethereum.SignKeys
		var gwProofs []*client.Proof
		// Split the voters
		if len(clients) == gw+1 {
			// if last client, add all remaining keys
			gwSigners = make([]*ethereum.SignKeys, len(censusKeys)-i)
			copy(gwSigners, censusKeys[i:])
			gwProofs = make([]*client.Proof, len(censusKeys)-i)
			copy(gwProofs, proofs[i:])
		} else {
			gwSigners = make([]*ethereum.SignKeys, p)
			copy(gwSigners, censusKeys[i:i+p])
			gwProofs = make([]*client.Proof, p)
			copy(gwProofs, proofs[i:i+p])
		}
		for _, gws := range gwSigners {
			gws.VocdoniChainID = chainId
		}
		log.Infof("%s will receive %d register keys", cl.Addr, len(gwSigners))
		gw, cl := gw, cl
		go func() {
			defer wg.Done()
			if regKeyTimes[gw], err = cl.TestPreRegisterKeys(pid,
				entityKey.Address().Bytes(),
				censusRoot,
				start,
				gwSigners,
				models.CensusOrigin_OFF_CHAIN_TREE,
				nil,
				gwProofs,
				doublePreRegister,
				checkNullifiers,
				&proofsReadyWG); err != nil {
				log.Fatalf("[%s] %s", cl.Addr, err)
			}
			log.Infof("gateway %d %s has ended its job", gw, cl.Addr)
		}()
		i += p
	}

	// Wait until all pre-register keys sent and check the results
	wg.Wait()

	// Wait until al pre-register have been registered
	log.Infof("waiting for all pre-registers to be registered...")
	for {
		rollingCensusSize, err := mainClient.GetRollingCensusSize(pid)
		if err != nil {
			log.Fatal(err)
		}
		if int(rollingCensusSize) == electionSize {
			break
		}
		log.Infof("RollingCensusSize = %v / %v", rollingCensusSize, electionSize)
		time.Sleep(5 * time.Second)
	}

	maxRegKeyTime := time.Duration(0)
	for _, t := range regKeyTimes {
		if t > maxRegKeyTime {
			maxRegKeyTime = t
		}
	}
	log.Infof("the pre-register process took %s", maxRegKeyTime)

	log.Infof("Waiting for start block...")
	mainClient.WaitUntilBlock(start)

	//
	// End of pre-registration.  Now we do the voting step with a SNARK
	//

	proc, err := mainClient.GetProcessInfo(pid)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Process: %+v\n", proc)
	rollingCensusRoot := proc.RollingCensusRoot

	// Send votes
	i = 0
	p = len(censusKeys) / len(clients)
	wg = sync.WaitGroup{}
	proofsReadyWG = sync.WaitGroup{}
	votingTimes := make([]time.Duration, len(clients))
	wg.Add(len(clients))
	proofsReadyWG.Add(len(clients))

	for gw, cl := range clients {
		var gwSigners []*ethereum.SignKeys
		// Split the voters
		if len(clients) == gw+1 {
			// if last client, add all remaining keys
			gwSigners = make([]*ethereum.SignKeys, len(censusKeys)-i)
			copy(gwSigners, censusKeys[i:])
		} else {
			gwSigners = make([]*ethereum.SignKeys, p)
			copy(gwSigners, censusKeys[i:i+p])
		}
		for _, gws := range gwSigners {
			gws.VocdoniChainID = chainId
		}
		log.Infof("%s will receive %d votes", cl.Addr, len(gwSigners))
		gw, cl := gw, cl
		go func() {
			defer wg.Done()
			if votingTimes[gw], err = cl.TestSendAnonVotes(pid,
				entityKey.Address().Bytes(),
				rollingCensusRoot,
				start,
				gwSigners,
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

	log.Infof("waiting for all votes to be validated...")
	timeDeadLine := time.Second * 200
	if electionSize > 1000 {
		timeDeadLine = time.Duration(electionSize/5) * time.Second
	}
	checkStart := time.Now()
	i = 0
	for {
		time.Sleep(time.Millisecond * 1000)
		if h, err := clients[i].GetEnvelopeHeight(pid); err != nil {
			log.Warnf("error getting envelope height: %v", err)
			i++
			if i > len(clients) {
				i = 0
			}
			continue
		} else {
			if h >= uint32(electionSize) {
				break
			}
			log.Infof("validated votes: %d", h)
		}
		if time.Since(checkStart) > timeDeadLine {
			log.Fatal("time deadline reached while waiting for results")
		}
	}

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
	log.Infof("the ENTIRE voting process took %s", maxVotingTime)
	log.Infof("checking results....")
	if r, err := mainClient.TestResults(pid, len(censusKeys), 1); err != nil {
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

	chainId, err := mainClient.GetChainID()
	if err != nil {
		log.Fatal(err)
	}
	oracleKey.VocdoniChainID = chainId
	entityKey.VocdoniChainID = chainId

	// create and top-up entity account
	mainClient.CreateAccount(entityKey, "ipfs://", 0)
	treasurer, err := mainClient.GetTreasurer(oracleKey)
	if err != nil {
		log.Fatal(err)
	}
	mainClient.MintTokens(oracleKey, entityKey.Address(), 10000, treasurer.Nonce)

	// Create process
	pid := client.Random(32)
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, err := mainClient.CreateProcess(
		entityKey,
		entityKey.Address().Bytes(),
		cspKey.PublicKey(),
		"https://dumycsp.foo",
		pid,
		&models.EnvelopeType{EncryptedVotes: encryptedVotes},
		nil,
		models.CensusOrigin_OFF_CHAIN_CA,
		0,
		procDuration,
		uint64(electionSize))
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
		for _, gws := range gwSigners {
			gws.VocdoniChainID = chainId
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
	log.Infof("the ENTIRE voting process took %s", maxVotingTime)
	log.Infof("checking results....")
	if r, err := mainClient.TestResults(pid, len(voters), 1); err != nil {
		log.Fatal(err)
	} else {
		log.Infof("results: %+v", r)
	}
	log.Infof("all done!")
}

// enduser voting is not tested here
func testAllTransactions(
	host,
	oraclePrivKey string,
) {

	var err error
	oracleKey := ethereum.NewSignKeys()
	if err := oracleKey.AddHexKey(oraclePrivKey); err != nil {
		log.Fatal(err)
	}

	log.Infof("connecting to main gateway %s", host)
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
	defer mainClient.Close()

	chainId, err := mainClient.GetChainID()
	if err != nil {
		log.Fatal(err)
	}

	// create and top-up main entity account
	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	randomSigner := &ethereum.SignKeys{}
	if err := randomSigner.Generate(); err != nil {
		log.Fatal(err)
	}

	oracleKey.VocdoniChainID = chainId
	randomSigner.VocdoniChainID = chainId

	mainClient.CreateAccount(randomSigner, "ipfs://", 0)
	treasurer, err := mainClient.GetTreasurer(oracleKey)
	if err != nil {
		log.Fatal(err)
	}
	mainClient.MintTokens(oracleKey, randomSigner.Address(), 10000, treasurer.Nonce)

	// create new account
	newSigner := &ethereum.SignKeys{}
	if err := newSigner.Generate(); err != nil {
		log.Fatal(err)
	}
	newSigner.VocdoniChainID = chainId
	err = mainClient.CreateAccount(newSigner, "ipfs://xyz", 0)
	if err != nil {
		log.Fatalf("cannot create account: %v", err)
	}
	log.Infof("account %s created successfully", newSigner.Address().String())

	log.Infof("waiting for new block ...")
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}

	// treasurer can mint tokens
	treasurer, err = mainClient.GetTreasurer(oracleKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := mainClient.MintTokens(oracleKey, newSigner.Address(), 15000, treasurer.Nonce); err != nil {
		log.Fatalf("cannot mint tokens: %v", err)
	}
	log.Infof("minted 15000 tokens to account %s", newSigner.Address().String())

	// change account info
	randomSignerAccount, err := mainClient.GetAccount(randomSigner, randomSigner.Address())
	if err != nil {
		log.Fatal(err)
	}
	if err := mainClient.CreateAccount(randomSigner, "ipfs://xyz", randomSignerAccount.Nonce); err != nil {
		log.Fatalf("cannot change account info: %v", err)
	}
	log.Infof("account infoURI changed to %s", "ipfs://xyz")

	log.Infof("waiting for new block ...")
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}

	// add delegate
	randomSignerAccount, err = mainClient.GetAccount(randomSigner, randomSigner.Address())
	if err != nil {
		log.Fatal(err)
	}
	if err := mainClient.SetAccountDelegate(randomSigner, newSigner.Address(), randomSignerAccount.Nonce, false); err != nil {
		log.Fatalf("cannot add account delegate: %v", err)
	}
	log.Infof("added delegate %s for account %s", newSigner.Address().String(), randomSigner.Address().String())

	// added delegate can change account info
	newSignerAccount, err := mainClient.GetAccount(newSigner, newSigner.Address())
	if err != nil {
		log.Fatal(err)
	}
	if err := mainClient.SetAccountInfoURI(newSigner, randomSigner.Address(), "ipfs://zyx", newSignerAccount.Nonce); err != nil {
		log.Fatalf("cannot change account info: %v", err)
	}
	log.Infof("account infoURI changed to %s", "ipfs://zyx")

	// delete delegate
	log.Infof("waiting for new block ...")
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}
	randomSignerAccount, err = mainClient.GetAccount(randomSigner, randomSigner.Address())
	if err != nil {
		log.Fatal(err)
	}
	if err := mainClient.SetAccountDelegate(randomSigner, newSigner.Address(), randomSignerAccount.Nonce, true); err != nil {
		log.Fatalf("cannot delete account delegate: %v", err)
	}
	log.Infof("deleted delegate %s for account %s", newSigner.Address().String(), randomSigner.Address().String())

	// send tokens to the newly created account
	log.Infof("waiting for new block ...")
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}
	randomSignerAccount, err = mainClient.GetAccount(randomSigner, randomSigner.Address())
	if err != nil {
		log.Fatal(err)
	}
	if err := mainClient.SendTokens(randomSigner, newSigner.Address(), 1714, randomSignerAccount.Nonce); err != nil {
		log.Fatalf("cannot send tokens from %s to %s", randomSigner.Address().String(), newSigner.Address().String())
	}
	log.Infof("sent %d tokens from %s to %s", 1500, randomSigner.Address().String(), newSigner.Address().String())

	// receive tokens using a faucet payload
	faucetPkg, err := mainClient.GenerateFaucetPackage(randomSigner, newSigner.Address(), 286)
	if err != nil {
		log.Fatal(err)
	}
	newSignerAccount, err = mainClient.GetAccount(newSigner, newSigner.Address())
	if err != nil {
		log.Fatal(err)
	}
	if err := mainClient.CollectFaucet(newSigner, faucetPkg, newSignerAccount.Nonce); err != nil {
		log.Fatal(err)
	}
	log.Infof("%s claimed %d tokens from %s", newSigner.Address().String(), 500, randomSigner.Address().String())

	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		log.Fatal("cannot get current height")
	}
	for {
		time.Sleep(time.Millisecond * 500)
		if h2, err := mainClient.GetCurrentBlock(); err != nil {
			log.Warnf("error getting current height: %v", err)
			continue
		} else {
			if h2 > h {
				break
			}
		}
	}

	// check expected data
	newSignerAccount, err = mainClient.GetAccount(newSigner, newSigner.Address())
	if err != nil {
		log.Fatal(err)
	}
	if newSignerAccount.Nonce != 1 {
		log.Fatalf("expected nonce %d for account %s, got %d", 1, newSigner.Address().String(), newSignerAccount.Nonce)
	}
	if newSignerAccount.Balance != 16990 {
		log.Fatalf("expected balance %d for account %s, got %d", 16990, newSigner.Address().String(), newSignerAccount.Balance)
	}

	log.Info("all done!")
}
