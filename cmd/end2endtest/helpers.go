package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"reflect"
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
	defaultWeight = 10
	retriesSend   = retries / 2
)

type acctProof struct {
	account  *ethereum.SignKeys
	proof    *apiclient.CensusProof
	proofSIK *apiclient.CensusProof
}

func newTestElectionDescription(numChoices int) *vapi.ElectionDescription {
	choices := []vapi.ChoiceMetadata{}
	if numChoices < 2 {
		numChoices = 2
	}
	for i := 0; i < numChoices; i++ {
		choices = append(choices, vapi.ChoiceMetadata{
			Title: map[string]string{"default": fmt.Sprintf("Choice number %d", i)},
			Value: uint32(i),
		})
	}

	return &vapi.ElectionDescription{
		Title:       map[string]string{"default": fmt.Sprintf("Test election %s", util.RandomHex(8))},
		Description: map[string]string{"default": "Test election description"},
		EndDate:     time.Now().Add(time.Minute * 20),

		Questions: []vapi.Question{
			{
				Title:       map[string]string{"default": "Test question 1"},
				Description: map[string]string{"default": "Test question 1 description"},
				Choices:     choices,
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

func (t *e2eElection) addParticipantsCensus(censusID types.HexBytes, voterAccounts []*ethereum.SignKeys) error {
	participants := &vapi.CensusParticipants{}

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

func (t *e2eElection) generateProofs(csp *ethereum.SignKeys, voterAccts []*ethereum.SignKeys) error {
	var (
		wg     sync.WaitGroup
		vcount int32
	)
	// Wait for the next block to assure the SIK root is updated
	if err := t.api.WaitUntilNextBlock(); err != nil {
		return err
	}
	errorChan := make(chan error)
	t.voters = new(sync.Map)
	addNaccounts := func(accounts []*ethereum.SignKeys) {
		defer wg.Done()
		log.Infof("generating %d census proofs", len(accounts))
		for _, acc := range accounts {
			voterKey := acc.Address().Bytes()
			proof, err := func() (*apiclient.CensusProof, error) {
				if csp != nil {
					return cspGenProof(t.election.ElectionID, voterKey, csp)
				}
				return t.api.CensusGenProof(t.election.Census.CensusRoot, voterKey)
			}()
			if err != nil {
				errorChan <- err
			}
			accP := acctProof{account: acc, proof: proof}

			if t.election.VoteMode.Anonymous {
				voterPrivKey := acc.PrivateKey()
				voterApi := t.api.Clone(voterPrivKey.String())
				accP.proofSIK, err = voterApi.GenSIKProof()
				if err != nil {
					errorChan <- err
				}
			}
			t.voters.Store(acc.Public, accP)
			atomic.AddInt32(&vcount, 1)
		}
	}

	pcount := t.config.nvotes / t.config.parallelCount
	for i := 0; i < len(voterAccts); i += pcount {
		end := i + pcount
		if end > len(voterAccts) {
			end = len(voterAccts)
		}
		wg.Add(1)
		go addNaccounts(voterAccts[i:end])
	}

	go func() { // avoid blocking the main goroutine
		wg.Wait()
		close(errorChan) // close the error channel after all goroutines have finished
	}()

	for err := range errorChan {
		if err != nil {
			return fmt.Errorf("error in generateProofs: %s", err)
		}
	}

	log.Debugf("%d/%d voting proofs generated successfully", vcount, len(voterAccts))
	return nil
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

func (t *e2eElection) publishCheckCensus(censusID types.HexBytes, nAcct uint64) (types.HexBytes, string, error) {
	// Check census size
	if !t.censusSizeEquals(censusID, nAcct) {
		return nil, "", fmt.Errorf("failed census size invalid, censusID %x", censusID)
	}

	rootHash, censusURL, err := t.api.CensusPublish(censusID)
	if err != nil {
		return nil, "", fmt.Errorf("failed census size invalid, censusID %x", censusID)
	}

	log.Infof("census published with root %x", rootHash)

	// Check census size (of the published census)
	if !t.censusSizeEquals(censusID, nAcct) {
		return nil, "", fmt.Errorf("failed census size invalid, censusID %x", censusID)
	}
	return rootHash, censusURL, nil
}

// setupCensus create a new census that will have nAcct voterAccounts and participants
func (t *e2eElection) setupCensus(censusType string, nAcct int, createAccounts bool) (types.HexBytes, []*ethereum.SignKeys, error) {
	// Create a new census
	censusID, err := t.api.NewCensus(censusType)
	if err != nil {
		return nil, nil, err
	}
	log.Infof("new census created with id %s", censusID.String())

	voterAccounts := ethereum.NewSignKeysBatch(nAcct)

	// Add the accounts to the census by batches
	if err := t.addParticipantsCensus(censusID, voterAccounts); err != nil {
		return nil, nil, err
	}

	// Register the accounts in the vochain if is required
	if censusType == vapi.CensusTypeZKWeighted && createAccounts {
		if err := t.registerAnonAccts(voterAccounts); err != nil {
			return nil, nil, err
		}
	}

	return censusID, voterAccounts, nil
}

func (t *e2eElection) setupElection(ed *vapi.ElectionDescription, nvAccts int) error {
	if err := t.setupAccount(); err != nil {
		return err
	}

	censusID, voterAccts, err := t.setupCensus(ed.Census.Type, nvAccts, !ed.TempSIKs)
	if err != nil {
		return err
	}

	ed.Census.RootHash, ed.Census.URL, err = t.publishCheckCensus(censusID, uint64(nvAccts))
	if err != nil {
		return err
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

	if ed.TempSIKs {
		errorChan := make(chan error)
		wg := &sync.WaitGroup{}
		halfAccts := len(voterAccts) / 2

		// register SIK for 1/2 accounts and left the rest to vote with account registered
		for i, acc := range voterAccts[:halfAccts] {
			wg.Add(1)
			go func(i int, acc *ethereum.SignKeys) {
				defer wg.Done()
				pKey := acc.PrivateKey()
				accountApi := t.api.Clone(pKey.String())
				hash, err := accountApi.RegisterSIKForVote(electionID, nil, nil)
				if err != nil {
					log.Errorf("could not register SIK for vote, address: %s, %v", acc.AddressString(), err)
					errorChan <- err
				}
				log.Infow("sik registered for anonymous census uncreated account", "index", i, "address", acc.AddressString())
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
				defer cancel()

				if _, err := accountApi.WaitUntilTxIsMined(ctx, hash); err != nil {
					log.Errorf("gave up waiting for tx %x to be mined: %s", hash, err)
					errorChan <- err
				}
			}(i, acc)
		}

		go func() { // avoid blocking the main goroutine
			wg.Wait()
			close(errorChan) // close the error channel after all goroutines have finished
		}()

		for err := range errorChan { // receive errors from the errorChan until it's closed.
			if err != nil {
				return err
			}
		}

		// register the remaining accounts
		if err := t.registerAnonAccts(voterAccts[halfAccts:]); err != nil {
			return err
		}

	}
	if err := t.generateProofs(nil, voterAccts); err != nil {
		return err
	}

	return nil
}

func (t *e2eElection) setupElectionRaw(prc *models.Process) error {
	var voterAccounts []*ethereum.SignKeys
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
		voterAccounts = ethereum.NewSignKeysBatch(t.config.nvotes)

		if err := csp.Generate(); err != nil {
			return err
		}
		censusRoot := csp.PublicKey()
		prc.CensusRoot = censusRoot

	default:
		censusType := vapi.CensusTypeWeighted
		censusID, vAccts, err := t.setupCensus(censusType, t.config.nvotes, false)
		if err != nil {
			return err
		}
		rootHash, url, err := t.publishCheckCensus(censusID, uint64(t.config.nvotes))
		if err != nil {
			return err
		}
		prc.CensusRoot = rootHash
		prc.CensusURI = &url
		voterAccounts = vAccts
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
	if err := t.generateProofs(csp, voterAccounts); err != nil {
		return err
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

// overwriteVote allow to try to overwrite a previous vote given the index of the account,
// always waiting for a block between overwrites (otherwise results are impredictable)
func (t *e2eElection) overwriteVote(choices []int, v *apiclient.VoteData) error {
	for i := 0; i < len(choices); i++ {
		// assign the choices wanted for each overwrite vote
		v.Choices = []int{choices[i]}
		errs := t.sendVotes([]*apiclient.VoteData{v})
		for _, err := range errs {
			// check the error expected for overwrite with waitUntilNextBlock
			if !strings.Contains(err.Error(), "overwrite count reached") {
				return fmt.Errorf("unexpected overwrite error: %w", err)
			}
			log.Debug("error expected: ", err.Error())
		}
		_ = t.api.WaitUntilNextBlock()
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
		if v.Election == nil {
			v.Election = t.election
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
		return nil, fmt.Errorf("need to pass a valid URL (--faucet)")
	case "dev":
		return apiclient.GetFaucetPackageFromDevService(myAddress)
	default:
		url, err := util.BuildURL(faucet, myAddress)
		if err != nil {
			return nil, err
		}
		return apiclient.GetFaucetPackageFromRemoteService(url, faucetAuthToken)
	}
}

func matchResults(results, expectedResults [][]*types.BigInt) bool {
	return reflect.DeepEqual(results, expectedResults)
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
	api := t.api.Clone(t.config.accountPrivKeys[0])

	// Wait for all the votes to be verified
	for {
		count, err := api.ElectionVoteCount(t.election.ElectionID)
		if err != nil {
			log.Warn(err)
		}
		if count == uint32(nvotesExpected) {
			break
		}

		if err := api.WaitUntilNextBlock(); err != nil {
			return fmt.Errorf("timeout waiting for next block")
		}

		log.Infof("verified %d/%d votes", count, nvotesExpected)
		if time.Since(startTime) > t.config.timeout {
			return fmt.Errorf("timeout waiting for votes to be registered")
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

func (t *e2eElection) verifyAndEndElection(nvotesExpected int) (*vapi.ElectionResults, error) {
	var wg sync.WaitGroup
	var results *vapi.ElectionResults
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := t.verifyVoteCount(nvotesExpected); err != nil {
			errCh <- fmt.Errorf("error in verifyVoteCount %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		elres, err := t.endElectionAndFetchResults()
		if err != nil {
			errCh <- fmt.Errorf("error in endElectionAndFetchResults %w", err)
		}
		results = elres
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return &vapi.ElectionResults{}, err
		}
	}

	return results, nil
}

func (t *e2eElection) registerAnonAccts(voterAccounts []*ethereum.SignKeys) error {
	errorChan := make(chan error)
	wg := &sync.WaitGroup{}

	for i, acc := range voterAccounts {
		if i%10 == 0 {
			// Print some information about progress on large censuses
			log.Infof("creating %d anonymous census accounts...", len(voterAccounts))
		}

		wg.Add(1)
		go func(i int, acc *ethereum.SignKeys) {
			defer wg.Done()
			pKey := acc.PrivateKey()
			if _, _, err := t.createAccount(pKey.String()); err != nil &&
				!strings.Contains(err.Error(), "createAccountTx: account already exists") {
				errorChan <- err
			}
			log.Infow("anonymous census account created", "index", i, "address", acc.AddressString())
		}(i, acc) // Pass the acc variable as a parameter to avoid data race
	}

	go func() { // avoid blocking the main goroutine
		wg.Wait()
		close(errorChan) // close the error channel after all goroutines have finished
	}()

	for err := range errorChan { // receive errors from the errorChan until it's closed.
		if err != nil {
			return err
		}
	}
	// Set the first account as the default account
	if err := t.api.SetAccount(t.config.accountPrivKeys[0]); err != nil {
		return err
	}
	return nil
}
