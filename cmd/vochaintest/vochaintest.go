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

	"github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"

	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

var ops = map[string]bool{
	"vtest":             true,
	"cspvoting":         true,
	"anonvoting":        true,
	"censusImport":      true,
	"censusGenerate":    true,
	"tokentransactions": true,
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
	treasurerPrivKey := flag.String("treasurerKey", "", "hexadecimal treasurer private key")
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
		fmt.Fprintf(os.Stderr,
			"=> tokentransactions\n\tTests all token "+
				"related transactions\n")
		fmt.Fprintf(os.Stderr,
			"\t./test --operation=tokentransactions --gwHost "+
				"wss://gw1test.vocdoni.net/dvote\n")
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

	oracleKey := ethereum.NewSignKeys()
	if len(*oraclePrivKey) > 0 {
		if err := oracleKey.AddHexKey(*oraclePrivKey); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := oracleKey.Generate(); err != nil {
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
	case "tokentransactions":
		// end-user voting is not tested here
		if len(*treasurerPrivKey) == 0 {
			log.Fatal("missing required treasurerKey parameter")
		}

		treasurerKey := ethereum.NewSignKeys()
		if err := treasurerKey.AddHexKey(*treasurerPrivKey); err != nil {
			log.Fatal(err)
		}

		// set accounts
		// oracle is also the treasurer
		if err := initAccounts(treasurerKey, oracleKey, entityKey, *host); err != nil {
			log.Fatalf("cannot init accounts: %w", err)
		}

		testTokenTransactions(*host, *treasurerPrivKey)
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

func waitUntilNextBlock(mainClient *client.Client) error {
	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height: %w", err)
	}
	mainClient.WaitUntilBlock(h + 1)
	return nil
}

func initAccounts(treasurer, oracle, mainSigner *ethereum.SignKeys, host string) error {
	var err error
	log.Infof("connecting to main gateway %s", host)
	// connecting to endpoint
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

	retries := 5
	// create and top up accounts
	if err := ensureAccountExists(mainClient, oracle, retries); err != nil {
		log.Fatal(err)
	}
	if err := ensureAccountExists(mainClient, mainSigner, retries); err != nil {
		log.Fatal(err)
	}

	if err := ensureAccountHasTokens(mainClient, treasurer, oracle.Address(), 10000, retries); err != nil {
		log.Fatalf("cannot mint tokens for account %s with treasurer %s: %w", oracle.Address(), treasurer.Address(), err)
	}
	if err := ensureAccountHasTokens(mainClient, treasurer, mainSigner.Address(), 10000, retries); err != nil {
		log.Fatalf("cannot mint tokens for account %s with treasurer %s: %w", mainSigner.Address(), treasurer.Address(), err)
	}

	return nil
}

func ensureAccountExists(mainClient *client.Client, account *ethereum.SignKeys, retries int) error {
	for i := 0; i <= retries; i++ {
		// if account exists, we're done
		if acct, _ := mainClient.GetAccount(nil, account.Address()); acct != nil {
			return nil
		}
		if i == retries { // that was the last chance, break and fail immediately
			break
		}

		if err := mainClient.CreateOrSetAccount(account, common.Address{}, "ipfs://", 0, nil); err != nil {
			if strings.Contains(err.Error(), "tx already exists in cache") {
				// don't worry then, someone else created it in a race, nevermind, job done.
			} else {
				return fmt.Errorf("cannot create account %s: %s", account.Address(), err)
			}
		}

		waitUntilNextBlock(mainClient)
	}
	return fmt.Errorf("cannot create account %s after %d retries", account.Address(), retries)
}

func ensureAccountHasTokens(mainClient *client.Client, treasurer *ethereum.SignKeys, accountAddr common.Address, amount uint64, retries int) error {
	for i := 0; i <= retries; i++ {
		// if balance is enough, we're done
		if acct, _ := mainClient.GetAccount(nil, accountAddr); acct != nil && acct.Balance >= amount {
			return nil
		}
		if i == retries { // that was the last chance, break and fail immediately
			break
		}

		// else, try to mint with treasurer
		treasurerAccount, err := mainClient.GetTreasurer(treasurer)
		if err != nil {
			return fmt.Errorf("cannot get treasurer: %w", err)
		}

		// MintTokens can return OK and then the tx be rejected anyway later.
		// So, ignore errors, we only care that accountHasTokens in the end
		_ = mainClient.MintTokens(treasurer, accountAddr, treasurerAccount.GetNonce(), amount)

		waitUntilNextBlock(mainClient)
	}
	return fmt.Errorf("tried to mint %d times, yet balance still not enough", retries)
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

	// Create process
	pid := client.Random(32)
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, err := mainClient.CreateProcess(
		oracleKey,
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

	// Create process
	pid := client.Random(32)
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, err := mainClient.CreateProcess(
		oracleKey,
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

	// Create process
	pid := client.Random(32)
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, err := mainClient.CreateProcess(
		oracleKey,
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
func testTokenTransactions(
	host,
	treasurerPrivKey string,
) {
	var err error
	treasurerSigner := ethereum.NewSignKeys()
	if err := treasurerSigner.AddHexKey(treasurerPrivKey); err != nil {
		log.Fatal(err)
	}

	// create main signer
	mainSigner := &ethereum.SignKeys{}
	if err := mainSigner.Generate(); err != nil {
		log.Fatal(err)
	}

	// create other signer
	otherSigner := &ethereum.SignKeys{}
	if err := otherSigner.Generate(); err != nil {
		log.Fatal(err)
	}

	log.Infof("connecting to main gateway %s", host)
	// add the first connection, this will be the main connection
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

	// check set transaction cost
	if err := testSetTxCost(mainClient, treasurerSigner); err != nil {
		log.Fatal(err)
	}

	// check create and set account
	if err := testCreateAndSetAccount(mainClient, treasurerSigner, mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}

	// check send tokens
	if err := testSendTokens(mainClient, treasurerSigner, mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}

	// check set account delegate
	if err := testSetAccountDelegate(mainClient, mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}

	// check collect faucet tx
	if err := testCollectFaucet(mainClient, mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}
}

func testSetTxCost(mainClient *client.Client, treasurerSigner *ethereum.SignKeys) error {
	// get treasurer
	treasurer, err := mainClient.GetTreasurer(treasurerSigner)
	if err != nil {
		return err
	}
	log.Infof("treasurer fetched %s with nonce %d", common.BytesToAddress(treasurer.Address), treasurer.Nonce)

	// get current tx cost
	txCost, err := mainClient.GetTransactionCost(treasurerSigner, models.TxType_SET_ACCOUNT_INFO)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s fetched successfully (%d)", models.TxType_SET_ACCOUNT_INFO, txCost)

	// set tx cost
	if err := mainClient.SetTransactionCost(treasurerSigner,
		models.TxType_SET_ACCOUNT_INFO,
		1000,
		treasurer.Nonce); err != nil {
		return fmt.Errorf("cannot set transaction cost: %v", err)
	}

	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)
	// check tx cost changed and treasurer nonce incremented
	treasurer2, err := mainClient.GetTreasurer(treasurerSigner)
	if err != nil {
		return err
	}
	newTxCost, err := mainClient.GetTransactionCost(treasurerSigner, models.TxType_SET_ACCOUNT_INFO)
	if err != nil {
		return err
	}
	if newTxCost != 1000 {
		return fmt.Errorf("newProcessTx cost expected to be %d got %d", 1000, newTxCost)
	}
	log.Infof("tx cost of %s changed successfully from %d to %d", models.TxType_SET_ACCOUNT_INFO, txCost, newTxCost)
	log.Infof("treasurer nonce changed successfully from %d to %d", treasurer.Nonce, treasurer2.Nonce)
	return nil
}

func testCreateAndSetAccount(mainClient *client.Client, treasurer, signer, signer2 *ethereum.SignKeys) error {
	// get current tx cost
	txCost, err := mainClient.GetTransactionCost(signer, models.TxType_SET_ACCOUNT_INFO)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s fetched successfully (%d)", models.TxType_SET_ACCOUNT_INFO, txCost)

	// create account without faucet package
	if err := mainClient.CreateOrSetAccount(signer,
		common.Address{},
		"ipfs://",
		0,
		nil); err != nil {
		return fmt.Errorf("cannot create account: %v", err)
	}
	// get current block
	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)
	// mint tokens for the created account
	treasurerAcc, err := mainClient.GetTreasurer(signer)
	if err != nil {
		return fmt.Errorf("cannot get treasurer %v", err)
	}

	// mint tokens to signer
	if err := mainClient.MintTokens(treasurer, signer.Address(), treasurerAcc.Nonce, 10000); err != nil {
		return fmt.Errorf("cannot mint tokens for account %s: %v", signer.Address(), err)
	}
	log.Infof("minted 10000 tokens to %s", signer.Address())
	// get current block
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)
	// check account created
	acc, err := mainClient.GetAccount(signer, signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("account %s succesfully created: %+v", signer.Address(), acc)
	// try set own account info
	if err := mainClient.CreateOrSetAccount(signer,
		common.Address{},
		"ipfs://XXX",
		0,
		nil); err != nil {
		return fmt.Errorf("cannot set account info: %v", err)
	}
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)
	// check account info changed
	acc, err = mainClient.GetAccount(signer, signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	if acc.InfoURI != "ipfs://XXX" {
		return fmt.Errorf("expected account infoURI to be %s got %s", "ipfs://XXX", acc.InfoURI)
	}
	log.Infof("account %s infoURI succesfully changed to %+v", signer.Address(), acc.InfoURI)
	// create account with faucet package
	faucetPkg, err := mainClient.GenerateFaucetPackage(signer, signer2.Address(), 500, rand.Uint64())
	if err != nil {
		return fmt.Errorf("cannot generate faucet package %v", err)
	}
	if err := mainClient.CreateOrSetAccount(signer2,
		common.Address{},
		"ipfs://",
		0,
		faucetPkg); err != nil {
		return fmt.Errorf("cannot create account: %v", err)
	}
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)
	// check account created
	acc2, err := mainClient.GetAccount(signer2, signer2.Address())
	if err != nil {
		return err
	}
	if acc2 == nil {
		return vochain.ErrAccountNotExist
	}
	// check balance added from payload
	if acc2.Balance != 500 {
		return fmt.Errorf("expected balance for account %s is %d but got %d", signer2.Address(), 500, acc2.Balance)
	}
	log.Infof("account %s (%+v) succesfully created with payload signed by %s", signer2.Address(), acc2, signer.Address())
	return nil
}

func testSendTokens(mainClient *client.Client, treasurerSigner, signer, signer2 *ethereum.SignKeys) error {
	txCost, err := mainClient.GetTransactionCost(treasurerSigner, models.TxType_SEND_TOKENS)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s is %d", models.TxType_SEND_TOKENS, txCost)
	acc, err := mainClient.GetAccount(signer, signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", signer.Address(), acc.Nonce, acc.Balance)
	// try send tokens
	if err := mainClient.SendTokens(signer,
		signer2.Address(),
		acc.Nonce,
		100); err != nil {
		return fmt.Errorf("cannot set account info: %v", err)
	}
	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)
	acc2, err := mainClient.GetAccount(signer2, signer2.Address())
	if err != nil {
		return err
	}
	if acc2 == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", signer2.Address(), acc2.Nonce, acc2.Balance)
	if acc2.Balance != 600 {
		log.Fatalf("expected %s to have balance %d got %d", signer2.Address(), 600, acc2.Balance)
	}
	acc3, err := mainClient.GetAccount(signer, signer.Address())
	if err != nil {
		return err
	}
	if acc3 == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", signer.Address(), acc3.Nonce, acc3.Balance)
	if acc.Balance-110 != acc3.Balance {
		log.Fatalf("expected %s to have balance %d got %d", signer.Address(), acc.Balance-110, acc3.Balance)
	}
	if acc.Nonce+1 != acc3.Nonce {
		log.Fatalf("expected %s to have balance %d got %d", signer.Address(), acc.Nonce+1, acc3.Nonce)
	}
	return nil
}

func testSetAccountDelegate(mainClient *client.Client, signer, signer2 *ethereum.SignKeys) error {
	txCostAdd, err := mainClient.GetTransactionCost(signer, models.TxType_ADD_DELEGATE_FOR_ACCOUNT)
	if err != nil {
		return err
	}
	txCostDel, err := mainClient.GetTransactionCost(signer, models.TxType_DEL_DELEGATE_FOR_ACCOUNT)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s is %d", models.TxType_ADD_DELEGATE_FOR_ACCOUNT, txCostAdd)
	log.Infof("tx cost of %s is %d", models.TxType_DEL_DELEGATE_FOR_ACCOUNT, txCostDel)

	acc, err := mainClient.GetAccount(signer, signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and delegates %v", signer.Address(), acc.Nonce, acc.DelegateAddrs)
	// add delegate
	if err := mainClient.SetAccountDelegate(signer,
		signer2.Address(),
		true,
		acc.Nonce); err != nil {
		return fmt.Errorf("cannot set account delegate: %v", err)
	}
	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)
	acc, err = mainClient.GetAccount(signer, signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched account %s with nonce %d and delegates %v", signer.Address(), acc.Nonce, acc.DelegateAddrs)
	if len(acc.DelegateAddrs) != 1 {
		log.Fatalf("expected %s to have 1 delegate got %d", signer.Address(), len(acc.DelegateAddrs))
	}
	addedDelegate := common.BytesToAddress(acc.DelegateAddrs[0])
	if addedDelegate != signer2.Address() {
		log.Fatalf("expeted delegate to be %s got %s", signer2.Address(), addedDelegate)
	}
	// delete delegate
	acc, err = mainClient.GetAccount(signer, signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	if err := mainClient.SetAccountDelegate(signer,
		signer2.Address(),
		false,
		acc.Nonce); err != nil {
		return fmt.Errorf("cannot set account delegate: %v", err)
	}
	h, err = mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)
	acc, err = mainClient.GetAccount(signer, signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched account %s with nonce %d and delegates %v", signer.Address(), acc.Nonce, acc.DelegateAddrs)
	if len(acc.DelegateAddrs) != 0 {
		log.Fatalf("expected %s to have 0 delegates got %d", signer.Address(), len(acc.DelegateAddrs))
	}
	return nil
}

func testCollectFaucet(mainClient *client.Client, from, to *ethereum.SignKeys) error {
	// get tx cost
	txCost, err := mainClient.GetTransactionCost(from, models.TxType_COLLECT_FAUCET)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s is %d", models.TxType_COLLECT_FAUCET, txCost)
	// fetch from account
	accFrom, err := mainClient.GetAccount(from, from.Address())
	if err != nil {
		return err
	}
	if accFrom == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", from.Address(), accFrom.Nonce, accFrom.Balance)

	// fetch to account
	accTo, err := mainClient.GetAccount(to, to.Address())
	if err != nil {
		return err
	}
	if accTo == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched to account %s with nonce %d and balance %d", to.Address(), accTo.Nonce, accFrom.Balance)

	// generate faucet pkg
	faucetPkg, err := mainClient.GenerateFaucetPackage(from, to.Address(), 10, uint64(util.RandomInt(0, 10000000)))
	if err != nil {
		return err
	}

	// collect faucet tx
	if err := mainClient.CollectFaucet(to, accTo.Nonce, faucetPkg); err != nil {
		return fmt.Errorf("error on collect faucet tx: %v", err)
	}

	// wait until tx is mined
	h, err := mainClient.GetCurrentBlock()
	if err != nil {
		return fmt.Errorf("cannot get current height")
	}
	mainClient.WaitUntilBlock(h + 2)

	// check values changed corretly on both from and to accounts
	accFrom, err = mainClient.GetAccount(from, from.Address())
	if err != nil {
		return err
	}
	if accFrom == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", from.Address(), accFrom.Nonce, accFrom.Balance)

	// fetch to account
	accTo, err = mainClient.GetAccount(to, to.Address())
	if err != nil {
		return err
	}
	if accTo == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched to account %s with nonce %d and balance %d", to.Address(), accTo.Nonce, accTo.Balance)

	return nil
}
