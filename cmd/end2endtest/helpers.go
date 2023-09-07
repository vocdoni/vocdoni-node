package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	nextBlock     = "nextBlock"
	sameBlock     = "sameBlock"
	defaultWeight = 10
	retriesSend   = retries / 2
)

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

func newTestProcess() *models.Process {
	return &models.Process{
		StartBlock: 0,
		BlockCount: 100,
		Status:     models.ProcessStatus_READY,
		EnvelopeType: &models.EnvelopeType{
			EncryptedVotes: false,
			UniqueValues:   true},
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount: 1,
			MaxValue: 1,
		},
		Mode: &models.ProcessMode{
			AutoStart:     true,
			Interruptible: true,
		},
	}
}

func (t *e2eElection) createAccount(privateKey string) (*vapi.Account, *apiclient.HTTPclient, error) {
	accountApi := t.api.Clone(privateKey)
	if acc, err := accountApi.Account(""); err == nil {
		return acc, accountApi, nil
	}

	address := accountApi.MyAddress().Hex()

	faucetPkg, err := faucetPackage(t.config.faucet, t.config.faucetAuthToken, address)
	if err != nil {
		return nil, nil, err
	}

	accountMetadata := &vapi.AccountMetadata{
		Name:        map[string]string{"default": "test account " + address},
		Description: map[string]string{"default": "test description"},
		Version:     "1.0",
	}

	log.Infof("creating Vocdoni account %s", address)
	hash, err := accountApi.AccountBootstrap(faucetPkg, accountMetadata, nil)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()

	if _, err := accountApi.WaitUntilTxIsMined(ctx, hash); err != nil {
		log.Errorf("gave up waiting for tx %x to be mined: %s", hash, err)
		return nil, nil, err
	}

	// check the account
	acc, err := accountApi.Account("")
	if err != nil {
		return nil, nil, err
	}
	if t.config.faucet != "" && acc.Balance == 0 {
		log.Error("account balance is 0")
		return nil, nil, err
	}
	return acc, accountApi, nil

}

func (t *e2eElection) addParticipantsCensus(censusID types.HexBytes, nvoterKeys int) error {
	participants := &vapi.CensusParticipants{}

	voterAccounts := t.voterAccounts

	// if the len of t.voterAccounts is not the same as nvoterKey, means that is only necessary
	// to add to census the last nvoterKeys added to t.voterAccounts
	if len(t.voterAccounts) != nvoterKeys {
		voterAccounts = t.voterAccounts[len(t.voterAccounts)-nvoterKeys:]
	}

	for i, voterAccount := range voterAccounts {
		participants.Participants = append(participants.Participants,
			vapi.CensusParticipant{
				Key:    voterAccount.Address().Bytes(),
				Weight: (*types.BigInt)(new(big.Int).SetUint64(defaultWeight)),
			})

		if i == len(voterAccounts)-1 || ((i+1)%vapi.MaxCensusAddBatchSize == 0) {
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

// isCensusSizeValid validate the size of the census, in case like dynamic census test the size could be different to nvotes
func (t *e2eElection) censusSizeEquals(censusID types.HexBytes, sizeExpected uint64) bool {
	size, err := t.api.CensusSize(censusID)
	if err != nil {
		log.Errorf("unable to get census size from api")
		return false
	}
	if size != sizeExpected {
		log.Errorf("census size is %d, expected %d", size, sizeExpected)
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

func (t *e2eElection) generateSIKProofs(root types.HexBytes) map[string]*apiclient.CensusProof {
	type voterProof struct {
		sikproof *apiclient.CensusProof
		address  string
	}
	sikProofs := make(map[string]*apiclient.CensusProof, len(t.voterAccounts))
	proofCh := make(chan *voterProof)
	stopProofs := make(chan bool)
	go func() {
		for {
			select {
			case p := <-proofCh:
				sikProofs[p.address] = p.sikproof
			case <-stopProofs:
				return
			}
		}
	}()

	addNaccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("generating %d sik proofs", len(accounts))
		for _, acc := range accounts {
			voterProof := &voterProof{address: acc.Address().Hex()}

			var err error
			voterPrivKey := acc.PrivateKey()
			voterApi := t.api.Clone(voterPrivKey.String())
			voterProof.sikproof, err = voterApi.GenSIKProof()
			if err != nil {
				log.Warn(err)
			}
			proofCh <- voterProof
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
	log.Debugf("%d/%d sik proofs generated successfully", len(sikProofs), len(t.voterAccounts))
	stopProofs <- true

	return sikProofs
}

func (t *e2eElection) generateProofs(root types.HexBytes, isAnonymousVoting bool, csp *ethereum.SignKeys) map[string]*apiclient.CensusProof {
	type voterProof struct {
		proof   *apiclient.CensusProof
		address string
	}
	proofs := make(map[string]*apiclient.CensusProof, len(t.voterAccounts))
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

	addNaccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("generating %d census proofs", len(accounts))
		for _, acc := range accounts {
			voterProof := &voterProof{address: acc.Address().Hex()}

			var err error
			if csp != nil {
				voterProof.proof, err = cspGenProof(t.election.ElectionID, acc.Address().Bytes(), csp)
			} else {
				voterPrivKey := acc.PrivateKey()
				voterApi := t.api.Clone(voterPrivKey.String())
				voterProof.proof, err = voterApi.CensusGenProof(root, acc.Address().Bytes())
				if err != nil {
					log.Warn(err)
				}
			}
			if err != nil {
				log.Warn(err)
			}

			if !isAnonymousVoting {
				voterProof.proof.KeyType = models.ProofArbo_ADDRESS
			}
			proofCh <- voterProof
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
	log.Debugf("%d/%d census proofs generated successfully", len(proofs), len(t.voterAccounts))
	stopProofs <- true

	return proofs
}

func (t *e2eElection) setupAccount() error {
	account, accountApi, err := t.createAccount(t.config.accountPrivKeys[0])
	if err != nil {
		return err
	}
	t.api = accountApi
	log.Infof("account %s balance is %d", t.api.MyAddress().Hex(), account.Balance)
	return nil
}

// setupCensus create a new census that will have nAcct voterAccounts and participants
func (t *e2eElection) setupCensus(censusType string, nAcct int, createAccounts bool) (types.HexBytes, string, error) {
	// Create a new census
	censusID, err := t.api.NewCensus(censusType)
	if err != nil {
		return nil, "", err
	}
	log.Infof("new census created with id %s", censusID.String())

	// Generate nAcct participant accounts
	t.voterAccounts = append(t.voterAccounts, ethereum.NewSignKeysBatch(nAcct)...)

	// Register the accounts in the vochain if is required
	if censusType == vapi.CensusTypeZKWeighted && createAccounts {
		for i, acc := range t.voterAccounts {
			if i%10 == 0 {
				// Print some information about progress on large censuses
				log.Infof("creating anonymous census accounts... (%d/%d - %.0f%%)",
					i, len(t.voterAccounts), float64(i)/float64(len(t.voterAccounts))*100)
			}

			pKey := acc.PrivateKey()
			if _, _, err := t.createAccount(pKey.String()); err != nil &&
				!strings.Contains(err.Error(), "createAccountTx: account already exists") {
				return nil, "", err
			}
		}

		if err := t.api.SetAccount(t.config.accountPrivKeys[0]); err != nil {
			return nil, "", err
		}
		log.Info("anonymous census accounts created")
	}

	// Add the accounts to the census by batches
	if err := t.addParticipantsCensus(censusID, nAcct); err != nil {
		return nil, "", err
	}

	// Check census size
	if !t.censusSizeEquals(censusID, uint64(nAcct)) {
		return nil, "", err
	}

	censusRoot, censusURI, err := t.api.CensusPublish(censusID)
	if err != nil {
		return nil, "", err
	}

	log.Infof("census published with root %x", censusRoot)

	// Check census size (of the published census)
	if !t.censusSizeEquals(censusID, uint64(nAcct)) {
		return nil, "", fmt.Errorf("failed census size invalid, root %x", censusRoot)
	}

	return censusRoot, censusURI, nil
}

func (t *e2eElection) setupElection(ed *vapi.ElectionDescription) error {
	if err := t.setupAccount(); err != nil {
		return err
	}

	// if the census is not defined yet, set up a new census that will have
	// nvotes voterAccounts and participants
	if ed.Census.RootHash == nil {
		censusRoot, censusURI, err := t.setupCensus(ed.Census.Type, t.config.nvotes, true)
		if err != nil {
			return err
		}

		ed.Census.RootHash = censusRoot
		ed.Census.URL = censusURI
	}

	if ed.Census.Size == 0 {
		ed.Census.Size = func() uint64 {
			if t.config.nvotes == 0 {
				return 1 // allows to test with no voters
			}
			return uint64(t.config.nvotes)
		}()
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
	if ed.ElectionType.Anonymous && !ed.TempSIKs {
		t.sikproofs = t.generateSIKProofs(ed.Census.RootHash)
	}

	return nil
}

func (t *e2eElection) setupElectionRaw(prc *models.Process) error {
	// Set the account in the API client, so we can sign transactions
	if err := t.setupAccount(); err != nil {
		return err
	}

	if prc.MaxCensusSize == 0 {
		prc.MaxCensusSize = uint64(t.config.nvotes)
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
		censusRoot, censusURI, err := t.setupCensus(censusType, t.config.nvotes, false)
		if err != nil {
			return err
		}
		prc.CensusRoot = censusRoot
		prc.CensusURI = &censusURI
		csp = nil
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
	if prc.EnvelopeType.Anonymous {
		t.sikproofs = t.generateSIKProofs(prc.CensusRoot)
	}

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
			if !strings.Contains(err.Error(), "overwrite count reached") {
				return fmt.Errorf("unexpected overwrite error: %w", err)
			}
			log.Debug("error expected: ", err.Error())
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

// sendVotes sends a batch of votes concurrently
// (number of goroutines defined in t.config.parallelCount)
// sendVotes sends a batch of votes concurrently
// (number of goroutines defined in t.config.parallelCount)
func (t *e2eElection) sendVotes(votes []*apiclient.VoteData) map[int]error {
	var errs = make(map[int]error)
	// used to avoid infinite for loop
	var timeoutsRetry, mempoolRetry, warnRetry atomic.Uint32
	var wg sync.WaitGroup
	var queues []map[int]*apiclient.VoteData
	var mutex sync.Mutex

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
						if timeoutsRetry.Load() > retriesSend {
							mutex.Lock()
							errs[i] = err
							mutex.Unlock()
							return
						}
						timeoutsRetry.Add(1)
					case strings.Contains(err.Error(), "mempool is full"):
						log.Warn(err)
						// wait and retry
						waitErr := t.api.WaitUntilNextBlock()
						if waitErr != nil {
							if mempoolRetry.Load() > retriesSend {
								mutex.Lock()
								errs[i] = err
								mutex.Unlock()
								return
							}
							mempoolRetry.Add(1)
						}
					case strings.Contains(err.Error(), "already exists"):
						// don't retry
						delete(queue, i)
						mutex.Lock()
						errs[i] = err
						mutex.Unlock()
					default:
						if warnRetry.Load() > retriesSend {
							mutex.Lock()
							errs[i] = err
							mutex.Unlock()
							return
						}
						// any other error, print it and wait a bit
						log.Warn(err)
						time.Sleep(100 * time.Millisecond)
						warnRetry.Add(1)
					}
				}
			}
		}(queues[p])
	}
	wg.Wait()
	log.Infow("sent votes",
		"n", len(votes), "timeouts", timeoutsRetry.Load(), "errors", len(errs))
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

func matchResults(results, expectedResults [][]*types.BigInt) bool {
	// iterate over each question to check if the results match with the expected results
	for i, result := range results {
		for q, r := range result {
			if !(expectedResults[i][q].String() == r.String()) {
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
		nvotesExpected, time.Since(startTime), int(float64(nvotesExpected)/time.Since(startTime).Seconds()))

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
