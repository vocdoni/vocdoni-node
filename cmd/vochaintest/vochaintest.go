//nolint:lll
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	ethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"go.vocdoni.io/dvote/client"
	vocli "go.vocdoni.io/dvote/cmd/vocli/commands"
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
	"initaccounts":      true,
}

var (
	addressRegexp = regexp.MustCompile("Public address of the key:.*")
	pathRegexp    = regexp.MustCompile("Path of the secret key file:.*")
)

// how many times to retry flaky transactions
// * amount of blocks to wait for a transaction to be mined before giving up
// * how many times to retry opening a connection to an endpoint before giving up
const retries = 10

// testClient is a *client.Client but with additional methods
// that are useful only in tests,
// like waitUntilAccountExists or ensureProcessCreated
type testClient struct {
	*client.Client
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
	accountPrivKeys := flag.StringSlice("accountKeys", []string{}, "hexadecimal account private keys array")
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
		fmt.Fprintf(os.Stderr,
			"=> testvocli\n\tTests major vocli functionalities\n")
		fmt.Fprintf(os.Stderr,
			"\t./test --operation=testvocli --treasurerKey=6aae1d165dd9776c580b8fdaf8622e39c5f943c715e20690080bbfce2c760223"+
				"--gwHost wss://gw1test.vocdoni.net/dvote\n")
	}
	flag.Parse()
	log.Init(*loglevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	accountKeys := make([]*ethereum.SignKeys, len(*accountPrivKeys))
	for i, key := range *accountPrivKeys {
		ak, err := privKeyToSigner(key)
		if err != nil {
			log.Fatal(err)
		}
		accountKeys[i] = ak
	}

	switch *opmode {
	case "initaccounts":
		log.Infof("initializing accounts ...")
		if err := initAccounts(*treasurerPrivKey, *oraclePrivKey, accountKeys, *host); err != nil {
			log.Fatalf("cannot init accounts: %s", err)
		}
	case "anonvoting":
		mkTreeAnonVoteTest(*host,
			accountKeys[0],
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
			*electionType == "encrypted-poll",
			accountKeys[0],
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
			*electionType == "encrypted-poll",
			accountKeys[0],
			cspKey,
			*electionSize,
			*procDuration,
			*parallelCons,
			*doubleVote,
			*checkNullifiers,
			*gateways,
		)
	case "censusImport":
		censusImport(*host, accountKeys[0])
	case "censusGenerate":
		censusGenerate(*host, accountKeys[0], *electionSize, *keysfile, 1)
	case "tokentransactions":
		// end-user voting is not tested here
		testTokenTransactions(*host, *treasurerPrivKey)
	case "vocli":
		testVocli(*host, *treasurerPrivKey)
	default:
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

// newTestClient will open a connection to addr.
// If it fails, it will retry 10 times (with 1s interval)
// before giving up
func newTestClient(addr string) (*testClient, error) {
	for tries := retries; tries > 0; tries-- {
		c, err := client.New(addr)
		if err == nil {
			return &testClient{c}, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, fmt.Errorf("couldn't connect to gateway, tried %d times", retries)
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
	// Connect
	cl, err := newTestClient(host)
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
	log.Infof("Census created and published\nRoot: %x\nURI: %s", root, uri)
}

func (c *testClient) ensureTxCostEquals(signer *ethereum.SignKeys, txType models.TxType, cost uint64) error {
	for i := 0; i < retries; i++ {
		// if cost equals expected, we're done
		newTxCost, err := c.GetTransactionCost(models.TxType_SET_ACCOUNT_INFO)
		if err != nil {
			log.Warn(err)
		}
		if newTxCost == cost {
			return nil
		}

		// else, try to set
		treasurerAccount, err := c.GetTreasurer()
		if err != nil {
			return fmt.Errorf("cannot get treasurer: %w", err)
		}

		log.Infof("will set %s txcost=%d (treasurer nonce: %d)", txType, cost, treasurerAccount.Nonce)
		txhash, err := c.SetTransactionCost(
			signer,
			txType,
			cost,
			treasurerAccount.Nonce)
		if err != nil {
			log.Warn(err)
			time.Sleep(time.Second)
			continue
		}
		err = c.WaitUntilTxMined(txhash)
		if err != nil {
			log.Warn(err)
		}
	}
	return fmt.Errorf("tried to set %s txcost=%d but failed %d times", txType, cost, retries)
}

func (c *testClient) ensureProcessCreated(
	signer *ethereum.SignKeys,
	entityID common.Address,
	censusRoot []byte,
	censusURI string,
	envelopeType *models.EnvelopeType,
	mode *models.ProcessMode,
	censusOrigin models.CensusOrigin,
	startBlockIncrement int,
	duration int,
	maxCensusSize uint64,
) (uint32, []byte, error) {
	for i := 0; i < retries; i++ {
		start, pid, err := c.CreateProcess(
			signer,
			entityID.Bytes(),
			censusRoot,
			censusURI,
			envelopeType,
			mode,
			censusOrigin,
			startBlockIncrement,
			duration,
			maxCensusSize)
		if err != nil {
			log.Warnf("CreateProcess: %v", err)
			c.WaitUntilNextBlock()
			continue
		}

		p, err := c.WaitUntilProcessAvailable(pid)
		if err != nil {
			log.Infof("ensureProcessCreated: process %x not yet available: %s", pid, err)
			continue
		}
		log.Infof("ensureProcessCreated: got process %x info %+v", pid, p)
		return start, pid, nil
	}
	return 0, nil, fmt.Errorf("ensureProcessCreated: process could not be created after %d retries", retries)
}

func (c *testClient) ensureProcessEnded(signer *ethereum.SignKeys, processID types.ProcessID) error {
	for i := 0; i <= retries; i++ {
		p, err := c.GetProcessInfo(processID)
		if err != nil {
			log.Warnf("ensureProcessEnded: cannot force end process: cannot get process %x info: %s", processID, err.Error())
			c.WaitUntilNextBlock()
			continue
		}
		log.Infof("ensureProcessEnded: got process %x info %+v", processID, p)
		if p != nil && (p.Status == int32(models.ProcessStatus_ENDED) || p.Status == int32(models.ProcessStatus_RESULTS)) {
			return nil
		}

		err = c.EndProcess(signer, processID)
		if err != nil {
			log.Warnf("EndProcess(%x): %v", processID, err)
			c.WaitUntilNextBlock()
			continue
		}
		c.WaitUntilNextBlock()
	}
	return fmt.Errorf("ensureProcessEnded: process could not be ended after %d retries", retries)
}

func initAccounts(treasurer, oracle string, accountKeys []*ethereum.SignKeys, host string) error {
	var err error
	treasurerSigner, err := privKeyToSigner(treasurer)
	if err != nil {
		log.Fatal(err)
	}
	oracleSigner, err := privKeyToSigner(oracle)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("connecting to main gateway %s", host)
	// connecting to endpoint
	c, err := newTestClient(host)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	// create and top up accounts
	if err := c.ensureAccountExists(oracleSigner, nil); err != nil {
		log.Fatal(err)
	}
	log.Infof("created oracle account with address %s", oracleSigner.Address())
	if err := c.ensureAccountHasTokens(treasurerSigner, oracleSigner.Address(), 100000); err != nil {
		return fmt.Errorf("cannot mint tokens for account %s with treasurer %s: %w", oracleSigner.Address(), treasurerSigner.Address(), err)
	}
	log.Infof("minted 100000 tokens to oracle")

	for _, k := range accountKeys {
		if err := c.ensureAccountExists(k, nil); err != nil {
			return fmt.Errorf("cannot check if account exists: %w", err)
		}
		log.Infof("created entity key account with addresses: %s", k.Address())
		if err := c.ensureAccountHasTokens(treasurerSigner, k.Address(), 100000); err != nil {
			return fmt.Errorf("cannot mint tokens for account %s with treasurer %s: %w", k.Address(), treasurerSigner.Address(), err)
		}
		log.Infof("minted 100000 tokens to account %s", k.Address())
	}
	return nil
}

func (c *testClient) ensureAccountInfoEquals(account *ethereum.SignKeys, infoURI string) error {
	for i := 0; i < retries; i++ {
		// if account InfoURI is as expected, we're done
		acct, err := c.GetAccount(account.Address())
		if err != nil {
			log.Debugf("GetAccount try %d: %v", i, err)
		}
		if acct != nil && acct.InfoURI == infoURI {
			return nil
		}

		// else, set it
		// CreateOrSetAccount can return OK and then the tx be rejected anyway later.
		// So, don't panic on errors, we only care about GetAccount in the next loop
		err = c.CreateOrSetAccount(account, common.Address{}, infoURI, acct.Nonce, nil)
		if err != nil {
			log.Debugf("CreateOrSetAccount try %d: %v", i, err)
		}

		c.WaitUntilNextBlock()
	}
	return fmt.Errorf("tried to set account %s InfoURI=%s, yet didn't happen after %d blocks", account.Address(), infoURI, retries)
}

func (c *testClient) ensureAccountExists(account *ethereum.SignKeys, faucetPkg *models.FaucetPackage) error {
	for i := 0; i < retries; i++ {
		acct, err := c.GetAccount(account.Address())
		if err != nil {
			log.Debugf("GetAccount try %d: %v", i, err)
		}
		// if account exists, we're done
		if acct != nil {
			return nil
		}

		err = c.CreateOrSetAccount(account, common.Address{}, "ipfs://", 0, faucetPkg)
		if err != nil {
			if strings.Contains(err.Error(), "tx already exists in cache") {
				// don't worry then, someone else created it in a race, nevermind, job done.
			} else {
				return fmt.Errorf("cannot create account %s: %w", account.Address(), err)
			}
		}

		c.WaitUntilNextBlock()
	}
	return fmt.Errorf("cannot create account %s after %d retries", account.Address(), retries)
}

func (c *testClient) ensureAccountHasTokens(
	treasurer *ethereum.SignKeys,
	accountAddr common.Address,
	amount uint64,
) error {
	for i := 0; i < retries; i++ {
		acct, err := c.GetAccount(accountAddr)
		if err != nil {
			log.Debugf("GetAccount try %d: %v", i, err)
		}
		// if balance is enough, we're done
		if acct != nil && acct.Balance >= amount {
			return nil
		}

		// else, try to mint with treasurer
		treasurerAccount, err := c.GetTreasurer()
		if err != nil {
			return fmt.Errorf("cannot get treasurer: %w", err)
		}

		// MintTokens can return OK and then the tx be rejected anyway later.
		// So, don't panic on errors, we only care about acct.Balance in the end
		err = c.MintTokens(treasurer, accountAddr, treasurerAccount.GetNonce(), amount)
		if err != nil {
			log.Debugf("MintTokens try %d: %v", i, err)
		}

		c.WaitUntilNextBlock()
	}
	return fmt.Errorf("tried to mint %d times, yet balance still not enough", retries)
}

func mkTreeVoteTest(host string,
	encryptedVotes bool,
	entityKey *ethereum.SignKeys,
	electionSize,
	procDuration,
	parallelCons int, withWeight uint64,
	doubleVote, checkNullifiers bool,
	gateways []string,
	keysfile string,
	useLastCensus bool,
	forceGatewaysGotCensus bool,
) {
	var censusKeys []*ethereum.SignKeys
	var proofs []*client.Proof
	var err error
	var censusRoot []byte
	censusURI := ""

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
	var clients []*testClient

	mainClient, err := newTestClient(host)
	if err != nil {
		log.Fatal(err)
	}
	defer mainClient.Close()

	// Create process
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, pid, err := mainClient.ensureProcessCreated(
		entityKey,
		entityKey.Address(),
		censusRoot,
		censusURI,
		&models.EnvelopeType{EncryptedVotes: encryptedVotes},
		&models.ProcessMode{Interruptible: true},
		models.CensusOrigin_OFF_CHAIN_TREE,
		0,
		procDuration,
		uint64(electionSize),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("created process with ID: %x", pid)
	// Create the websockets connections for sending the votes
	gwList := append(gateways, host)

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cl, err := newTestClient(gwList[i%len(gwList)])
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
		}()
		i += p
	}

	// Wait until all votes sent and check the results
	wg.Wait()

	log.Infof("waiting for all votes to be validated...")
	timeDeadLine := time.Second * 400
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
	if err := mainClient.ensureProcessEnded(entityKey, pid); err != nil {
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

func mkTreeAnonVoteTest(host string,
	entityKey *ethereum.SignKeys,
	electionSize,
	procDuration,
	parallelCons int,
	doubleVote, doublePreRegister, checkNullifiers bool,
	gateways []string,
	keysfile string,
	useLastCensus bool,
	forceGatewaysGotCensus bool,
) {
	// log.Init("debug", "stdout")

	var censusKeys []*ethereum.SignKeys
	var proofs []*client.Proof
	var err error
	var censusRoot []byte
	censusURI := ""

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
	var clients []*testClient
	mainClient, err := newTestClient(host)
	if err != nil {
		log.Fatal(err)
	}
	defer mainClient.Close()

	// Create process
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, pid, err := mainClient.ensureProcessCreated(
		entityKey,
		entityKey.Address(),
		censusRoot,
		censusURI,
		&models.EnvelopeType{Anonymous: true},
		&models.ProcessMode{AutoStart: true, Interruptible: true, PreRegister: true},
		models.CensusOrigin_OFF_CHAIN_TREE,
		7,
		procDuration,
		uint64(electionSize),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("created process with ID: %x", pid)
	// Create the websockets connections for sending the votes
	gwList := append(gateways, host)

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cl, err := newTestClient(gwList[i%len(gwList)])
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
	mainClient.WaitUntilBlock(start + 2)

	//
	// End of pre-registration.  Now we do the voting step with a SNARK
	//

	proc, err := mainClient.WaitUntilProcessAvailable(pid)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Process: %+v", proc)
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
	timeDeadLine := time.Second * 400
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
	// also checks oracle permissions
	if err := mainClient.ensureProcessEnded(entityKey, pid); err != nil {
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
	host string,
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

	log.Infof("connecting to main gateway %s", host)
	// Add the first connection, this will be the main connection
	var clients []*testClient

	mainClient, err := newTestClient(host)
	if err != nil {
		log.Fatal(err)
	}
	defer mainClient.Close()

	// Create process
	log.Infof("creating process with entityID: %s", entityKey.AddressString())
	start, pid, err := mainClient.ensureProcessCreated(
		entityKey,
		entityKey.Address(),
		cspKey.PublicKey(),
		"https://dumycsp.foo",
		&models.EnvelopeType{EncryptedVotes: encryptedVotes},
		&models.ProcessMode{Interruptible: true},
		models.CensusOrigin_OFF_CHAIN_CA,
		0,
		procDuration,
		uint64(electionSize),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("created process with ID: %x", pid)
	// Create the websockets connections for sending the votes
	gwList := append(gateways, host)

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cl, err := newTestClient(gwList[i%len(gwList)])
		if err != nil {
			log.Warn(err)
			continue
		}
		defer cl.Close()
		clients = append(clients, cl)
	}

	for i := 0; i < parallelCons; i++ {
		log.Infof("opening gateway connection to %s", gwList[i%len(gwList)])
		cl, err := newTestClient(gwList[i%len(gwList)])
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
	if err := mainClient.ensureProcessEnded(entityKey, pid); err != nil {
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
	treasurerSigner, err := privKeyToSigner(treasurerPrivKey)
	if err != nil {
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
	mainClient, err := newTestClient(host)
	if err != nil {
		log.Fatal(err)
	}
	defer mainClient.Close()

	// check set transaction cost
	if err := mainClient.testSetTxCost(treasurerSigner); err != nil {
		log.Fatal(err)
	}

	// check create and set account
	if err := mainClient.testCreateAndSetAccount(treasurerSigner, mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}

	// check send tokens
	if err := mainClient.testSendTokens(treasurerSigner, mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}

	// check set account delegate
	if err := mainClient.testSetAccountDelegate(mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}

	// check collect faucet tx
	if err := mainClient.testCollectFaucet(mainSigner, otherSigner); err != nil {
		log.Fatal(err)
	}
}

func (c *testClient) testSetTxCost(treasurerSigner *ethereum.SignKeys) error {
	// get treasurer none
	treasurer, err := c.GetTreasurer()
	if err != nil {
		return err
	}
	log.Infof("treasurer fetched %s with nonce %d", common.BytesToAddress(treasurer.Address), treasurer.Nonce)

	// get current tx cost
	txCost, err := c.GetTransactionCost(models.TxType_SET_ACCOUNT_INFO)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s fetched successfully (%d)", models.TxType_SET_ACCOUNT_INFO, txCost)

	// check tx cost changes
	err = c.ensureTxCostEquals(treasurerSigner, models.TxType_SET_ACCOUNT_INFO, 1000)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s changed successfully from %d to %d", models.TxType_SET_ACCOUNT_INFO, txCost, 1000)

	// get new treasurer nonce
	treasurer2, err := c.GetTreasurer()
	if err != nil {
		return err
	}
	log.Infof("treasurer nonce changed successfully from %d to %d", treasurer.Nonce, treasurer2.Nonce)

	// set tx cost back to default
	err = c.ensureTxCostEquals(treasurerSigner, models.TxType_SET_ACCOUNT_INFO, 10)
	if err != nil {
		return fmt.Errorf("cannot set transaction cost: %v", err)
	}

	return nil
}

func (c *testClient) testCreateAndSetAccount(treasurer, signer, signer2 *ethereum.SignKeys) error {
	// create account without faucet package
	if err := c.ensureAccountExists(signer, nil); err != nil {
		return err
	}

	// mint tokens to signer
	if err := c.ensureAccountHasTokens(treasurer, signer.Address(), 1000000); err != nil {
		return fmt.Errorf("cannot mint tokens for account %s: %v", signer.Address(), err)
	}
	log.Infof("minted 1000000 tokens to %s", signer.Address())

	// check account created
	acc, err := c.GetAccount(signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("account %s successfully created: %+v", signer.Address(), acc)

	// now try set own account info
	err = c.ensureAccountInfoEquals(signer, "ipfs://XXX")
	if err != nil {
		return err
	}

	// check account info changed
	acc, err = c.GetAccount(signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	if acc.InfoURI != "ipfs://XXX" {
		return fmt.Errorf("expected account infoURI to be %s got %s", "ipfs://XXX", acc.InfoURI)
	}
	log.Infof("account %s infoURI successfully changed to %+v", signer.Address(), acc.InfoURI)

	// create account with faucet package
	faucetPkg, err := c.GenerateFaucetPackage(signer, signer2.Address(), 5000, rand.Uint64())
	if err != nil {
		return fmt.Errorf("cannot generate faucet package %v", err)
	}

	if err := c.ensureAccountExists(signer2, faucetPkg); err != nil {
		return err
	}

	// check account created
	acc2, err := c.GetAccount(signer2.Address())
	if err != nil {
		return err
	}
	if acc2 == nil {
		return vochain.ErrAccountNotExist
	}
	// check balance added from payload
	if acc2.Balance != 5000 {
		return fmt.Errorf("expected balance for account %s is %d but got %d", signer2.Address(), 5000, acc2.Balance)
	}
	log.Infof("account %s (%+v) successfully created with payload signed by %s", signer2.Address(), acc2, signer.Address())
	return nil
}

func (c *testClient) testSendTokens(treasurerSigner, signer, signer2 *ethereum.SignKeys) error {
	txCost, err := c.GetTransactionCost(models.TxType_SEND_TOKENS)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s is %d", models.TxType_SEND_TOKENS, txCost)
	acc, err := c.GetAccount(signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", signer.Address(), acc.Nonce, acc.Balance)
	// try send tokens
	txhash, err := c.SendTokens(signer,
		signer2.Address(),
		acc.Nonce,
		100)
	if err != nil {
		return fmt.Errorf("cannot send tokens: %v", err)
	}

	err = c.WaitUntilTxMined(txhash)
	if err != nil {
		return err
	}

	acc2, err := c.GetAccount(signer2.Address())
	if err != nil {
		return err
	}
	if acc2 == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched to account %s with nonce %d and balance %d", signer2.Address(), acc2.Nonce, acc2.Balance)
	if acc2.Balance != 5100 {
		log.Fatalf("expected %s to have balance %d got %d", signer2.Address(), 5100, acc2.Balance)
	}
	acc3, err := c.GetAccount(signer.Address())
	if err != nil {
		return err
	}
	if acc3 == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", signer.Address(), acc3.Nonce, acc3.Balance)
	if acc.Balance-(100+txCost) != acc3.Balance {
		log.Fatalf("expected %s to have balance %d got %d", signer.Address(), acc.Balance-(100+txCost), acc3.Balance)
	}
	if acc.Nonce+1 != acc3.Nonce {
		log.Fatalf("expected %s to have balance %d got %d", signer.Address(), acc.Nonce+1, acc3.Nonce)
	}
	return nil
}

func (c *testClient) testSetAccountDelegate(signer, signer2 *ethereum.SignKeys) error {
	txCostAdd, err := c.GetTransactionCost(models.TxType_ADD_DELEGATE_FOR_ACCOUNT)
	if err != nil {
		return err
	}
	txCostDel, err := c.GetTransactionCost(models.TxType_DEL_DELEGATE_FOR_ACCOUNT)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s is %d", models.TxType_ADD_DELEGATE_FOR_ACCOUNT, txCostAdd)
	log.Infof("tx cost of %s is %d", models.TxType_DEL_DELEGATE_FOR_ACCOUNT, txCostDel)

	acc, err := c.GetAccount(signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and delegates %v", signer.Address(), acc.Nonce, acc.DelegateAddrs)
	// add delegate
	txhash, err := c.SetAccountDelegate(signer,
		signer2.Address(),
		true,
		acc.Nonce)
	if err != nil {
		return fmt.Errorf("cannot set account delegate: %v", err)
	}

	err = c.WaitUntilTxMined(txhash)
	if err != nil {
		return err
	}

	acc, err = c.GetAccount(signer.Address())
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
	acc, err = c.GetAccount(signer.Address())
	if err != nil {
		return err
	}
	if acc == nil {
		return vochain.ErrAccountNotExist
	}
	txhash, err = c.SetAccountDelegate(signer,
		signer2.Address(),
		false,
		acc.Nonce)
	if err != nil {
		return fmt.Errorf("cannot set account delegate: %v", err)
	}

	err = c.WaitUntilTxMined(txhash)
	if err != nil {
		return err
	}

	acc, err = c.GetAccount(signer.Address())
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

func (c *testClient) testCollectFaucet(from, to *ethereum.SignKeys) error {
	// get tx cost
	txCost, err := c.GetTransactionCost(models.TxType_COLLECT_FAUCET)
	if err != nil {
		return err
	}
	log.Infof("tx cost of %s is %d", models.TxType_COLLECT_FAUCET, txCost)
	// fetch from account
	accFrom, err := c.GetAccount(from.Address())
	if err != nil {
		return err
	}
	if accFrom == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", from.Address(), accFrom.Nonce, accFrom.Balance)

	// fetch to account
	accTo, err := c.GetAccount(to.Address())
	if err != nil {
		return err
	}
	if accTo == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched to account %s with nonce %d and balance %d", to.Address(), accTo.Nonce, accFrom.Balance)

	// generate faucet pkg
	faucetPkg, err := c.GenerateFaucetPackage(from, to.Address(), 10, uint64(util.RandomInt(0, 10000000)))
	if err != nil {
		return err
	}

	// collect faucet tx
	txhash, err := c.CollectFaucet(to, accTo.Nonce, faucetPkg)
	if err != nil {
		return fmt.Errorf("error on collect faucet tx: %v", err)
	}

	// wait until tx is mined
	err = c.WaitUntilTxMined(txhash)
	if err != nil {
		return fmt.Errorf("CollectFaucet tx was never mined: %v", err)
	}

	// check values changed correctly on both from and to accounts
	accFrom, err = c.GetAccount(from.Address())
	if err != nil {
		return err
	}
	if accFrom == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched from account %s with nonce %d and balance %d", from.Address(), accFrom.Nonce, accFrom.Balance)

	// fetch to account
	accTo, err = c.GetAccount(to.Address())
	if err != nil {
		return err
	}
	if accTo == nil {
		return vochain.ErrAccountNotExist
	}
	log.Infof("fetched to account %s with nonce %d and balance %d", to.Address(), accTo.Nonce, accTo.Balance)

	return nil
}

func testVocli(url, treasurerPrivKey string) {
	vocli.SetupLogPackage = false

	log.Infof("connecting to main gateway %s", url)
	c, err := newTestClient(url)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	parseImportOutput := func(stdout string) (address, keyPath string) {
		a := strings.TrimSpace(addressRegexp.FindString(stdout))
		a2 := strings.Split(a, " ")
		newAddr := a2[len(a2)-1]

		k := strings.TrimSpace(pathRegexp.FindString(stdout))
		k2 := strings.Split(k, " ")
		keyPath = k2[len(k2)-1]
		return newAddr, keyPath
	}
	generateKeyAndReturnAddress := func(url string, stdArgs []string) (address, keyPath string, err error) {
		_, stdout, _, err := executeCommand(vocli.RootCmd, append([]string{"keys", "new", fmt.Sprintf("-u=%s", url)}, stdArgs...), "", false)
		if err != nil {
			return
		}
		address, keyPath = parseImportOutput(stdout)
		_, _, _, err = executeCommand(vocli.RootCmd, append([]string{"account", "set", keyPath, "ipfs://", fmt.Sprintf("-u=%s", url)}, stdArgs...), "", false)
		if err != nil {
			return
		}
		out, err := c.vocliWaitUntilAccountExists(address, stdArgs)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("account %s info: %s", address, out)
		return
	}

	dir, err := ioutil.TempDir("", "testvoclihome")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	stdArgs := []string{"--password=password", fmt.Sprintf("--home=%s", dir), fmt.Sprintf("-u=%s", url)}

	log.Info("vocli keys import alicePrivateKey")
	if err := ioutil.WriteFile(path.Join(dir, "alicePrivateKey"), []byte(treasurerPrivKey), 0o700); err != nil {
		log.Fatal(err)
	}
	_, stdout, _, err := executeCommand(vocli.RootCmd, append([]string{"keys", "import", path.Join(dir, "/alicePrivateKey")}, stdArgs...), "", true)
	if err != nil {
		log.Fatal(err)
	}
	alice, aliceKeyPath := parseImportOutput(stdout)
	if !strings.Contains(stdout, dir) {
		log.Fatalf("vocli import should report that the imported key was stored in directory %s", dir)
	}
	log.Infof("alice key path: %s", aliceKeyPath)

	log.Info("vocli keys list")
	_, stdout, _, err = executeCommand(vocli.RootCmd, append([]string{"keys", "list"}, stdArgs...), "", true)
	if err != nil {
		log.Fatal(err)
	}
	if !strings.Contains(stdout, dir) {
		log.Fatalf("vocli list should have found and shown a key in dir %s", dir)
	}
	log.Infof("key list: %s", stdout)

	log.Info("vocli account set alice")
	_, stdout, _, err = executeCommand(vocli.RootCmd, append([]string{"account", "set", aliceKeyPath, "ipfs://aliceinwonderland"}, stdArgs...), "", true)
	if err != nil && !strings.Contains(err.Error(), "tx already exists in cache") {
		log.Fatal(err)
	}
	if !strings.Contains(stdout, fmt.Sprintf("created/updated on chain %s", url)) {
		log.Fatalf("stdout should mention that alice's account was created on the chain, but instead: %s", stdout)
	}

	out, err := c.vocliWaitUntilAccountExists(alice, stdArgs)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("account %s fetched: %s", alice, out)

	log.Info("vocli account mint alice (this lets one reuse node states across test runs, because subsequent SetAccountInfoTxs are not free")
	err = c.vocliEnsureAccountHasBalance(aliceKeyPath, alice, "1000", stdArgs)
	if err != nil {
		log.Fatal(err)
	}

	func() {
		log.Info("vocli keys changepassword (a new key)")
		_, stdout, _, err := executeCommand(vocli.RootCmd, append([]string{"keys", "new", fmt.Sprintf("-u=%s", url)}, stdArgs...), "", true)
		if err != nil {
			log.Fatal(err)
		}
		_, keyPath := parseImportOutput(stdout)

		_, _, _, err = executeCommand(vocli.RootCmd, []string{"keys", "changepassword", keyPath, "--password=password"}, "fdsa\n", true)
		if err != nil {
			log.Fatal(err)
		}
		k, err := ioutil.ReadFile(keyPath)
		if err != nil {
			log.Fatal(err)
		}
		if _, err = ethkeystore.DecryptKey(k, "fdsa"); err != nil {
			log.Fatal(err)
		}
	}()
	func() {
		log.Info("vocli mint alice -> newAccount")
		var newAccount string
		if newAccount, _, err = generateKeyAndReturnAddress(url, stdArgs); err != nil {
			log.Fatal(err)
		}
		err = c.vocliEnsureAccountHasBalance(aliceKeyPath, newAccount, "100", stdArgs)
		if err != nil {
			log.Fatal(err)
		}
	}()
	func() {
		log.Info("vocli send a -> b")
		a, aKeyPath, err := generateKeyAndReturnAddress(url, stdArgs)
		if err != nil {
			log.Fatal(err)
		}
		b, _, err := generateKeyAndReturnAddress(url, stdArgs)
		if err != nil {
			log.Fatal(err)
		}
		// generateKeyAndReturnAddress only returns after the account was effectively mined.
		err = c.vocliEnsureAccountHasBalance(aliceKeyPath, a, "1000", stdArgs)
		if err != nil {
			log.Fatal(err)
		}
		_, _, _, err = executeCommand(vocli.RootCmd, append([]string{"send", aKeyPath, b, "98"}, stdArgs...), "", true)
		if err != nil {
			log.Fatal(err)
		}
		_, err = c.vocliWaitUntilAccountInfoContains(b, "balance:98", stdArgs)
		if err != nil {
			log.Fatalf("send a -> b failed, balance still not 98 (%v)", err)
		}
	}()
	func() {
		log.Info("vocli genfaucet/claimfaucet a -> b")
		a, aKeyPath, err := generateKeyAndReturnAddress(url, stdArgs)
		if err != nil {
			log.Fatal(err)
		}
		b, bKeyPath, err := generateKeyAndReturnAddress(url, stdArgs)
		if err != nil {
			log.Fatal(err)
		}
		err = c.vocliEnsureAccountHasBalance(aliceKeyPath, a, "1000", stdArgs)
		if err != nil {
			log.Fatal(err)
		}

		_, stdout, _, err = executeCommand(vocli.RootCmd, append([]string{"genfaucet", aKeyPath, b, "800"}, stdArgs...), "", true) // leave plenty of spare coins for differing txCosts
		if err != nil {
			log.Fatal(err)
		}

		_, stdout, _, err = executeCommand(vocli.RootCmd, append([]string{"claimfaucet", bKeyPath, strings.TrimSpace(stdout)}, stdArgs...), "", true)
		if err != nil {
			log.Fatal(err)
		}
	}()
	func() {
		log.Info("vocli txcost get NewProcess")
		// get the initial txcosts. this is not used by the test, just for the
		// human to read
		if _, _, _, err := executeCommand(vocli.RootCmd, append([]string{"txcost", "get"}, stdArgs...), "", true); err != nil {
			log.Fatal(err)
		}
		_, _, _, err = executeCommand(vocli.RootCmd, append([]string{"txcost", "set", aliceKeyPath, "RegisterKey", "50"}, stdArgs...), "", true)
		if err != nil {
			log.Fatal(err)
		}
		stdout, err = c.vocliRepeatCmdUntilStdoutContains(append([]string{"txcost", "get"}, stdArgs...), "RegisterKey 50")
		if err != nil {
			log.Fatal(err)
		}
		finalTxCosts := strings.Split(strings.TrimSpace(stdout), "\n")
		log.Infof("new tx cost: %+v", finalTxCosts)
	}()
}

func (c *testClient) vocliEnsureAccountHasBalance(keyPath, account, amount string, stdArgs []string) error {
	for i := 0; i < retries; i++ {
		_, stdout, stderr, err := executeCommand(vocli.RootCmd, append([]string{"mint", keyPath, account, amount}, stdArgs...), "", true)
		if err != nil {
			log.Fatal(err)
		}
		if strings.Contains(stdout, "invalid nonce") || strings.Contains(stderr, "invalid nonce") {
			// race happened, try again
			time.Sleep(time.Second)
			continue
		}
		_, err = c.vocliWaitUntilAccountInfoContains(account, "balance:", stdArgs)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("mint failed: %s balance is still 0 (gave up after %d retries)", account, retries)
}

func (c *testClient) vocliRepeatCmdUntilStdoutContains(args []string, s string) (stdout string, err error) {
	for i := 0; i < retries; i++ {
		_, stdout, _, err := executeCommand(vocli.RootCmd, args, "", true)
		if err == nil && strings.Contains(stdout, s) {
			return stdout, nil
		}
		if err != nil {
			log.Debug(err)
		}
		c.WaitUntilNextBlock()
	}
	return "", fmt.Errorf("gave up waiting after %d blocks", retries)
}
func (c *testClient) vocliWaitUntilAccountInfoContains(address string, s string, stdArgs []string) (stdout string, err error) {
	return c.vocliRepeatCmdUntilStdoutContains(append([]string{"account", "info", address}, stdArgs...), s)
}

// vocliWaitUntilAccountExists executes "vocli account info <address>" until success or timeout
// often, the SetAccountInfoTx doesn't get mined even in 2, 3x the block period
// so keep polling until we finally confirm the account has been created
func (c *testClient) vocliWaitUntilAccountExists(address string, stdArgs []string) (stdout string, err error) {
	stdout, err = c.vocliWaitUntilAccountInfoContains(address, "infoURI:", stdArgs)
	if err != nil {
		return "", fmt.Errorf("account %s still doesn't exist: %v", address, err)
	}
	return stdout, nil
}

func executeCommand(root *cobra.Command, args []string, input string, verbose bool) (*cobra.Command, string, string, error) {
	// setup stdout/stderr redirection.
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	vocli.Stdout = stdoutBuf
	vocli.Stderr = stderrBuf

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		return root, "", "", err
	}
	stdinWriter.Write([]byte(input))
	vocli.Stdin = stdinReader

	// cobra.Command.SetOut/SetErr() is actually useless for most purposes as it
	// only includes the usage messages printed to stdout/stderr. For full
	// stdout/stderr capture it's still best to use vocli.Stdout/Stderr. Let's
	// just mock it out anyway.
	root.SetOut(stdoutBuf)
	root.SetErr(stderrBuf)
	root.SetArgs(args)
	c, err := root.ExecuteC()

	if verbose {
		log.Debug("vocli ", strings.Join(args, " "))
		log.Debugf("stdout(%s)\tstderr(%s)", stdoutBuf, stderrBuf)
	}

	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.SetArgs([]string{})
	return c, stdoutBuf.String(), stderrBuf.String(), err
}
