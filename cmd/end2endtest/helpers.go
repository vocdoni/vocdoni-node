package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	nextBlock = "nextBlock"
	sameBlock = "sameBlock"
)

type ballotData struct {
	maxValue     uint32
	maxTotalCost uint32
	costExponent uint32
	maxCount     uint32
}

func newTestElectionDescription() *vapi.ElectionDescription {
	return &vapi.ElectionDescription{
		Title:       map[string]string{"default": fmt.Sprintf("Test election %s", util.RandomHex(8))},
		Description: map[string]string{"default": "Test election description"},
		EndDate:     time.Now().Add(time.Minute * 20),

		Questions: []vapi.Question{
			{
				Title:       map[string]string{"default": "Test question 1"},
				Description: map[string]string{"default": "Test question 1 description"},
				Choices: []vapi.ChoiceMetadata{
					{
						Title: map[string]string{"default": "Yes"},
						Value: 0,
					},
					{
						Title: map[string]string{"default": "No"},
						Value: 1,
					},
				},
			},
		},
	}
}

func (t *e2eElection) createAccount(address string) (*vapi.Account, error) {
	faucetPkg, err := faucetPackage(t.config.faucet, t.config.faucetAuthToken, address)
	if err != nil {
		return nil, err
	}

	accountMetadata := &vapi.AccountMetadata{
		Name:        map[string]string{"default": "test account " + address},
		Description: map[string]string{"default": "test description"},
		Version:     "1.0",
	}

	log.Infof("creating Vocdoni account %s", address)
	hash, err := t.api.AccountBootstrap(faucetPkg, accountMetadata)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()

	if _, err := t.api.WaitUntilTxIsMined(ctx, hash); err != nil {
		log.Errorf("gave up waiting for tx %x to be mined: %s", hash, err)
		return nil, err
	}

	// check the account
	acc, err := t.api.Account("")
	if err != nil {
		return nil, err
	}
	if t.config.faucet != "" && acc.Balance == 0 {
		log.Error("account balance is 0")
		return nil, err
	}
	return acc, nil

}

func (t *e2eElection) addParticipantsCensus(censusType string, censusID types.HexBytes) error {
	participants := &vapi.CensusParticipants{}

	for i, voterAccount := range t.voterAccounts {
		keyAddr, err := censusParticipantKey(voterAccount, censusType)
		if err != nil {
			return err
		}
		participants.Participants = append(participants.Participants,
			vapi.CensusParticipant{
				Key:    keyAddr,
				Weight: (*types.BigInt)(new(big.Int).SetUint64(10)),
			})

		if i == len(t.voterAccounts)-1 || ((i+1)%vapi.MaxCensusAddBatchSize == 0) {
			if err := t.api.CensusAddParticipants(censusID, participants); err != nil {
				return err
			}

			log.Infof("added %d participants to census %s",
				len(participants.Participants), censusID.String())

			participants = &vapi.CensusParticipants{}
		}
	}
	return nil
}

func (t *e2eElection) isCensusSizeValid(censusID types.HexBytes) bool {
	size, err := t.api.CensusSize(censusID)
	if err != nil {
		log.Errorf("unable to get census size from api")
		return false
	}
	if size != uint64(t.config.nvotes) {
		log.Errorf("census size is %d, expected %d", size, t.config.nvotes)
		return false
	}
	log.Infof("census %s size is %d", censusID.String(), size)
	return true
}

func (t *e2eElection) waitUntilElectionStarts(electionID types.HexBytes) (*vapi.Election, error) {
	log.Infof("wait until the election: %s, starts", electionID.String())
	// Wait for the election to start
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*80)
	defer cancel()
	election, err := t.api.WaitUntilElectionStarts(ctx, electionID)
	if err != nil {
		return nil, err
	}
	election.ElectionID = electionID

	return election, nil
}

func (t *e2eElection) generateProofs(root types.HexBytes, isAnonymousVoting bool, csp *ethereum.SignKeys) map[string]*apiclient.CensusProof {
	type voterProof struct {
		proof   *apiclient.CensusProof
		address string
	}
	proofs := make(map[string]*apiclient.CensusProof, t.config.nvotes)
	proofCh := make(chan *voterProof)
	stopProofs := make(chan bool)
	go func() {
		for {
			select {
			case p := <-proofCh:
				proofs[p.address] = p.proof
			case <-stopProofs:
				return
			}
		}
	}()

	apiClientMtx := &sync.Mutex{}
	addNaccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("generating %d voting proofs", len(accounts))
		for _, acc := range accounts {
			voterKey := acc.Address().Bytes()

			if isAnonymousVoting {
				apiClientMtx.Lock()
				privKey := acc.PrivateKey()
				if err := t.api.SetAccount(privKey.String()); err != nil {
					apiClientMtx.Unlock()
					log.Fatal(err)
					return
				}
				voterKey = t.api.MyZkAddress().Bytes()
			}

			var pr *apiclient.CensusProof
			var err error
			if csp != nil {
				pr, err = cspGenProof(t.election.ElectionID, voterKey, csp)
			} else {
				pr, err = t.api.CensusGenProof(root, voterKey)
				if isAnonymousVoting {
					apiClientMtx.Unlock()
				}
			}

			if err != nil {
				log.Fatal(err)
			}

			if !isAnonymousVoting {
				pr.KeyType = models.ProofArbo_ADDRESS
			}
			proofCh <- &voterProof{
				proof:   pr,
				address: acc.Address().Hex(),
			}
		}
	}

	pcount := t.config.nvotes / t.config.parallelCount
	var wg sync.WaitGroup
	for i := 0; i < len(t.voterAccounts); i += pcount {
		end := i + pcount
		if end > len(t.voterAccounts) {
			end = len(t.voterAccounts)
		}
		wg.Add(1)
		go addNaccounts(t.voterAccounts[i:end], &wg)
	}

	wg.Wait()
	log.Debugf("%d/%d voting proofs generated successfully", len(proofs), len(t.voterAccounts))
	stopProofs <- true

	return proofs
}

func (t *e2eElection) setupAccount() error {
	// Set the account in the API client, so we can sign transactions
	if err := t.api.SetAccount(t.config.accountPrivKeys[0]); err != nil {
		return err
	}

	// If the account does not exist, create a new one
	// TODO: check if the account balance is low and use the faucet
	acc, err := t.api.Account("")
	if err != nil {
		acc, err = t.createAccount(t.api.MyAddress().Hex())
		if err != nil {
			return err
		}
	}
	log.Infof("account %s balance is %d", t.api.MyAddress().Hex(), acc.Balance)
	return nil
}

func (t *e2eElection) setupCensus(censusType string) (types.HexBytes, error) {
	// Create a new census
	censusID, err := t.api.NewCensus(censusType)
	if err != nil {
		return nil, err
	}
	log.Infof("new census created with id %s", censusID.String())

	// Generate 10 participant accounts
	t.voterAccounts = ethereum.NewSignKeysBatch(t.config.nvotes)

	// Add the accounts to the census by batches
	if err := t.addParticipantsCensus(censusType, censusID); err != nil {
		return nil, err
	}

	// Check census size
	if !t.isCensusSizeValid(censusID) {
		return nil, err
	}

	return censusID, nil
}

func (t *e2eElection) setupElection(ed *vapi.ElectionDescription) error {
	if err := t.setupAccount(); err != nil {
		return err
	}

	censusID, err := t.setupCensus(ed.Census.Type)
	if err != nil {
		return err
	}

	// Publish the census
	ed.Census.RootHash, ed.Census.URL, err = t.api.CensusPublish(censusID)
	if err != nil {
		return err
	}
	log.Infof("census published with root %x", ed.Census.RootHash.String())

	if ed.Census.Size == 0 {
		ed.Census.Size = func() uint64 {
			if t.config.nvotes == 0 {
				return 1 // allows to test with no voters
			}
			return uint64(t.config.nvotes)
		}()
	}

	// Check census size (of the published census)
	if !t.isCensusSizeValid(ed.Census.RootHash) {
		return err
	}

	if err := t.checkElectionPrice(ed); err != nil {
		return err
	}

	electionID, err := t.api.NewElection(ed)
	if err != nil {
		return err
	}
	log.Infof("created new election with id %s", electionID.String())

	election, err := t.waitUntilElectionStarts(electionID)
	if err != nil {
		return err
	}
	t.election = election
	t.proofs = t.generateProofs(ed.Census.RootHash, ed.ElectionType.Anonymous, nil)

	return nil
}

func (t *e2eElection) setupElectionRaw(prc *models.Process) error {
	// Set the account in the API client, so we can sign transactions
	if err := t.setupAccount(); err != nil {
		return err
	}

	csp := &ethereum.SignKeys{}

	switch prc.CensusOrigin {
	case models.CensusOrigin_OFF_CHAIN_CA:
		t.voterAccounts = ethereum.NewSignKeysBatch(t.config.nvotes)

		if err := csp.Generate(); err != nil {
			return err
		}
		censusRoot := csp.PublicKey()
		prc.CensusRoot = censusRoot

	default:
		censusType := vapi.CensusTypeWeighted
		censusID, err := t.setupCensus(censusType)
		if err != nil {
			return err
		}

		// Publish the census
		var censusURI string
		prc.CensusRoot, censusURI, err = t.api.CensusPublish(censusID)
		if err != nil {
			return err
		}

		log.Infof("census published with root %x", prc.CensusRoot)

		prc.CensusURI = &censusURI
		csp = nil

		// Check census size (of the published census)
		if !t.isCensusSizeValid(prc.CensusRoot) {
			return err
		}
	}

	electionID, err := t.api.NewElectionRaw(prc)
	if err != nil {
		return err
	}
	log.Infof("created new electionRaw with id %s", electionID.String())

	election, err := t.waitUntilElectionStarts(electionID)
	if err != nil {
		return err
	}
	t.election = election
	prc.ProcessId = t.election.ElectionID

	t.proofs = t.generateProofs(prc.CensusRoot, prc.EnvelopeType.Anonymous, csp)

	return nil
}

func (t *e2eElection) checkElectionPrice(ed *vapi.ElectionDescription) error {
	price, err := t.api.ElectionPrice(ed)
	if err != nil {
		return fmt.Errorf("could not get election price: %w", err)
	}
	acc, err := t.api.Account("")
	if err != nil {
		return fmt.Errorf("could not fetch own account: %w", err)
	}
	if acc.Balance < price {
		return fmt.Errorf("not enough balance to create election (needed %d): %w", price, err)
	}
	log.Infow("creating new election", "price", price, "balance", acc.Balance)
	return nil
}

// overwriteVote allow to try to overwrite a previous vote given the index of the account, it can use the sameBlock or the nextBlock
func (t *e2eElection) overwriteVote(choices []int, indexAcct int, waitType string) error {
	for i := 0; i < len(choices); i++ {
		// assign the choices wanted for each overwrite vote
		errs := t.sendVotes([]*apiclient.VoteData{{
			ElectionID:   t.election.ElectionID,
			ProofMkTree:  t.proofs[t.voterAccounts[indexAcct].Address().Hex()],
			VoterAccount: t.voterAccounts[indexAcct],
			Choices:      []int{choices[i]}}})
		for _, err := range errs {
			// check the error expected for overwrite with waitUntilNextBlock
			if strings.Contains(err.Error(), "overwrite count reached") {
				log.Debug("error expected: ", err.Error())
			} else {
				return fmt.Errorf("unexpected overwrite error: %w", err)
			}
		}
		switch waitType {
		case sameBlock:
			time.Sleep(time.Second * 5)

		case nextBlock:
			_ = t.api.WaitUntilNextBlock()
		}
	}
	return nil
}

// ballotVotes from a default list of 10 vote values that exceed the max value, max total cost and is not unique will
func ballotVotes(b ballotData, nvotes int) ([][]int, [][]*types.BigInt) {
	votes := make([][]int, 0, nvotes)
	var resultsField1, resultsField2 []*types.BigInt

	// initial 10 votes to be sent by the ballot test
	var v = [][]int{
		{0, 0}, {0, 7}, {5, 7}, {2, 0}, {2, 3},
		{1, 6}, {0, 8}, {6, 5}, {2, 7}, {6, 4},
	}

	// less than 10 votes
	if nvotes < 10 {
		// default results with zero values on field 1 for less than 10 votes
		resultsField1 = votesToBigInt(make([]uint64, b.maxValue+1)...)
		// default results with zero values on field 2 for less than 10 votes
		resultsField2 = votesToBigInt(make([]uint64, b.maxValue+1)...)
	} else {
		// greater o equal than 10 votes
		for i := 0; i < nvotes/10; i++ {
			votes = append(votes, v...)
		}
		// default results on field 1 for 10 votes
		resultsField1 = votesToBigInt(0, 10, 20, 0, 0, 0, 10)
		// default results on field 2 for 10 votes
		resultsField2 = votesToBigInt(10, 0, 0, 10, 10, 0, 10)

		// nvotes split 10, for example for 44 nvotes, nvoteDid10 will be 4
		// and that number will be multiplied by each default result to obtain the results for 40 votes
		nvotesDiv10 := new(types.BigInt).SetUint64(uint64(nvotes / 10))

		for i := 0; i <= int(b.maxValue); i++ {
			newvalField1 := new(types.BigInt).Mul(resultsField1[i], nvotesDiv10)
			newvalField2 := new(types.BigInt).Mul(resultsField2[i], nvotesDiv10)

			resultsField1[i] = newvalField1
			resultsField2[i] = newvalField2
		}
	}

	// remainVotes check if exists remain votes to add and count, note that if nvotes < 10 raminVote will be nvotes
	remainVotes := nvotes % 10
	if remainVotes != 0 {
		votes = append(votes, v[:remainVotes]...)
		// update expected results
		for i := 0; i < remainVotes; i++ {
			isValidTotalCost := math.Pow(float64(v[i][0]), float64(b.costExponent))+
				math.Pow(float64(v[i][1]), float64(b.costExponent)) <= float64(b.maxTotalCost)
			isValidValues := v[i][0] <= int(b.maxValue) && v[i][1] <= int(b.maxValue)
			isUniqueValues := v[i][0] != v[i][1]

			if isValidTotalCost && isValidValues && isUniqueValues {
				newvalField1 := new(types.BigInt).Add(resultsField1[v[i][0]], new(types.BigInt).SetUint64(10))
				newvalField2 := new(types.BigInt).Add(resultsField2[v[i][1]], new(types.BigInt).SetUint64(10))

				resultsField1[v[i][0]] = newvalField1
				resultsField2[v[i][1]] = newvalField2
			}
		}
	}

	expectedResults := [][]*types.BigInt{resultsField1, resultsField2}

	log.Debug("vote values generated", votes)
	log.Debug("results expected", expectedResults)
	return votes, expectedResults
}

// sendVotes sends a batch of votes concurrently
// (number of goroutines defined in t.config.parallelCount)
func (t *e2eElection) sendVotes(votes []*apiclient.VoteData) (errs map[int]error) {
	errs = make(map[int]error)
	var timeouts atomic.Uint32
	var wg sync.WaitGroup
	var queues []map[int]*apiclient.VoteData
	for p := 0; p < t.config.parallelCount; p++ {
		queues = append(queues, make(map[int]*apiclient.VoteData, len(votes)))
	}
	for i, v := range votes {
		if v.ElectionID == nil {
			v.ElectionID = t.election.ElectionID
		}
		queues[i%t.config.parallelCount][i] = v
	}

	for p := 0; p < t.config.parallelCount; p++ {
		wg.Add(1)
		go func(queue map[int]*apiclient.VoteData) {
			defer wg.Done()
			for len(queue) > 0 {
				log.Infow("thread sending votes", "queue", len(queue))
				for i, vote := range queue {
					_, err := t.api.Vote(vote)
					switch {
					case err == nil:
						delete(queue, i)
					case errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err):
						// if the context deadline is reached, no need to print it, just retry
						timeouts.Add(1)
					case strings.Contains(err.Error(), "mempool is full"):
						log.Warn(err)
						// wait and retry
						_ = t.api.WaitUntilNextBlock()
					case strings.Contains(err.Error(), "already exists") ||
						strings.Contains(err.Error(), "overwrite count reached"):
						// don't retry
						delete(queue, i)
						errs[i] = err
					default:
						// any other error, print it and wait a bit
						log.Warn(err)
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
		}(queues[p])
	}
	wg.Wait()
	log.Infow("sent votes",
		"n", len(votes), "timeouts", timeouts.Load(), "failed", len(errs))
	return errs
}

func faucetPackage(faucet, faucetAuthToken, myAddress string) (*models.FaucetPackage, error) {
	switch faucet {
	case "":
		return nil, fmt.Errorf("need to pass a valid --faucet")
	case "dev":
		return apiclient.GetFaucetPackageFromDevService(myAddress)
	default:
		return apiclient.GetFaucetPackageFromRemoteService(faucet+myAddress, faucetAuthToken)
	}
}

func censusParticipantKey(voterAccount *ethereum.SignKeys, censusType string) ([]byte, error) {
	var key []byte

	switch censusType {
	case vapi.CensusTypeWeighted:
		key = voterAccount.Address().Bytes()
	case vapi.CensusTypeZKWeighted:
		zkAddr, err := zk.AddressFromSignKeys(voterAccount)
		if err != nil {
			return nil, err
		}
		key = zkAddr.Bytes()
	}
	return key, nil
}

func matchResults(results, expectedResults [][]*types.BigInt) bool {
	// iterate over each question to check if the results match with the expected results
	for i := 0; i < len(results); i++ {
		for q := range results[i] {
			if !(expectedResults[i][q].String() == results[i][q].String()) {
				return false
			}
		}
	}
	return true
}

func votesToBigInt(votes ...uint64) []*types.BigInt {
	vBigInt := make([]*types.BigInt, len(votes))
	for i, v := range votes {
		vBigInt[i] = new(types.BigInt).SetUint64(v)
	}
	return vBigInt
}

func cspGenProof(pid, voterKey []byte, csp *ethereum.SignKeys) (*apiclient.CensusProof, error) {
	bundle := &models.CAbundle{
		ProcessId: pid,
		Address:   voterKey,
	}
	bundleBytes, err := proto.Marshal(bundle)
	if err != nil {
		return nil, err
	}

	signature, err := csp.SignEthereum(bundleBytes)
	if err != nil {
		return nil, err
	}

	caProof := &models.ProofCA{
		Bundle:    bundle,
		Type:      models.ProofCA_ECDSA,
		Signature: signature,
	}
	caProofBytes, err := proto.Marshal(caProof)
	if err != nil {
		return nil, err
	}

	return &apiclient.CensusProof{Proof: caProofBytes}, nil
}

func (t *e2eElection) verifyVoteCount(nvotesExpected int) error {
	startTime := time.Now()

	// Wait for all the votes to be verified
	for {
		count, err := t.api.ElectionVoteCount(t.election.ElectionID)
		if err != nil {
			log.Warn(err)
		}
		if count == uint32(nvotesExpected) {
			break
		}

		if err := t.api.WaitUntilNextBlock(); err != nil {
			return fmt.Errorf("timeout waiting for next block")
		}

		log.Infof("verified %d/%d votes", count, nvotesExpected)
		if time.Since(startTime) > t.config.timeout {
			log.Fatalf("timeout waiting for votes to be registered")
		}
	}

	log.Infof("%d votes registered successfully, took %s (%d votes/second)",
		t.config.nvotes, time.Since(startTime), int(float64(nvotesExpected)/time.Since(startTime).Seconds()))

	return nil
}

func (t *e2eElection) endElectionAndFetchResults() (*vapi.ElectionResults, error) {
	// Set the account back to the organization account
	api := t.api.Clone(t.config.accountPrivKeys[0])

	// End the election by setting the status to ENDED
	log.Infof("ending election...")
	if _, err := api.SetElectionStatus(t.election.ElectionID, "ENDED"); err != nil {
		return nil, fmt.Errorf("cannot set election status to ENDED %w", err)
	}

	// Wait for the election to be in RESULTS state
	ctx, cancel := context.WithTimeout(context.Background(), t.config.timeout*3)
	defer cancel()

	results, err := api.WaitUntilElectionResults(ctx, t.election.ElectionID)
	if err != nil {
		return nil, fmt.Errorf("error waiting for election publish final results %w", err)
	}
	return results, nil
}
