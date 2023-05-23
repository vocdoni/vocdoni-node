package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	nextBlock = "nextBlock"
	sameBlock = "sameBlock"
)

type voteInfo struct {
	voterAccount *ethereum.SignKeys
	choice       []int
	keys         []vapi.Key
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

func (t *e2eElection) createElection(electionDescrip *vapi.ElectionDescription) (*vapi.Election, error) {
	price, err := t.api.ElectionPrice(electionDescrip)
	if err != nil {
		return nil, fmt.Errorf("could not get election price: %w", err)
	}
	acc, err := t.api.Account("")
	if err != nil {
		return nil, fmt.Errorf("could not fetch own account: %w", err)
	}
	if acc.Balance < price {
		return nil, fmt.Errorf("not enough balance to create election (needed %d): %w", price, err)
	}
	log.Infow("creating new election", "price", price, "balance", acc.Balance)

	electionID, err := t.api.NewElection(electionDescrip)
	if err != nil {
		return nil, err
	}
	log.Infof("created new election with id %s - now wait until it starts", electionID.String())

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

func (t *e2eElection) generateProofs(root types.HexBytes, isAnonymousVoting bool) map[string]*apiclient.CensusProof {
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

			pr, err := t.api.CensusGenProof(root, voterKey)
			if isAnonymousVoting {
				apiClientMtx.Unlock()
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
	time.Sleep(time.Second) // wait a grace time for the last proof to be added
	log.Debugf("%d/%d voting proofs generated successfully", len(proofs), len(t.voterAccounts))
	stopProofs <- true

	return proofs
}

func (t *e2eElection) setupElection(ed *vapi.ElectionDescription) error {
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

	// Create a new census
	censusID, err := t.api.NewCensus(ed.Census.Type)
	if err != nil {
		return err
	}
	log.Infof("new census created with id %s", censusID.String())

	// Generate 10 participant accounts
	t.voterAccounts = ethereum.NewSignKeysBatch(t.config.nvotes)

	// Add the accounts to the census by batches
	if err := t.addParticipantsCensus(ed.Census.Type, censusID); err != nil {
		return err
	}

	// Check census size
	if !t.isCensusSizeValid(censusID) {
		return err
	}

	// Publish the census
	root, censusURI, err := t.api.CensusPublish(censusID)
	if err != nil {
		return err
	}
	log.Infof("census published with root %s", root.String())
	ed.Census.RootHash = root
	ed.Census.URL = censusURI

	if ed.Census.Size == 0 {
		ed.Census.Size = func() uint64 {
			if t.config.nvotes == 0 {
				return 1 // allows to test with no voters
			}
			return uint64(t.config.nvotes)
		}()
	}

	// Check census size (of the published census)
	if !t.isCensusSizeValid(root) {
		return err
	}

	t.election, err = t.createElection(ed)
	if err != nil {
		return err
	}

	log.Infof("created new election with id %s", t.election.ElectionID.String())

	// Wait for the election to start
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	t.election, err = t.api.WaitUntilElectionStarts(ctx, t.election.ElectionID)
	if err != nil {
		return err
	}

	t.proofs = t.generateProofs(root, ed.ElectionType.Anonymous)

	return nil
}

// overwriteVote allow to try to overwrite a previous vote given the index of the account, it can use the sameBlock or the nextBlock
func (t *e2eElection) overwriteVote(choices []int, indexAcct int, waitType string) (int, error) {
	acc := t.voterAccounts[indexAcct]
	contextDeadlines := 0

	for i := 0; i < len(choices); i++ {
		// assign the choices wanted for each overwrite vote
		choice := []int{choices[i]}
		ctxDeadLine, err := t.sendVote(voteInfo{voterAccount: acc, choice: choice}, nil)
		if err != nil {
			// check the error expected for overwrite with waitUntilNextBlock
			if strings.Contains(err.Error(), "overwrite count reached") {
				log.Debug("error expected: ", err.Error())
			} else {
				return 0, errors.New("expected overwrite error")
			}
		}
		contextDeadlines += ctxDeadLine
		switch waitType {
		case sameBlock:
			time.Sleep(time.Second * 5)

		case nextBlock:
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
			t.api.WaitUntilNextBlock(ctx)
		}
	}
	return contextDeadlines, nil
}

// sendVote send one vote using the api client without waiting for the next block
func (t *e2eElection) sendVote(v voteInfo, apiClientMtx *sync.Mutex) (int, error) {
	var contextDeadline int

	api := t.api
	if t.election.VoteMode.Anonymous {
		apiClientMtx.Lock()
		privKey := v.voterAccount.PrivateKey()
		if err := t.api.SetAccount(privKey.String()); err != nil {
			apiClientMtx.Unlock()
			return 0, err
		}
	} else {
		api = t.api.Clone(fmt.Sprintf("%x", v.voterAccount.PrivateKey()))
	}

	if _, err := api.Vote(&apiclient.VoteData{
		ElectionID:  t.election.ElectionID,
		ProofMkTree: t.proofs[v.voterAccount.Address().Hex()],
		Choices:     v.choice,
		Keys:        v.keys},
	); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
			contextDeadline = 1
		} else if strings.Contains(err.Error(), "reached") {
			// this error is expected on overwrite or maxCensusSize test
			return 0, err
		} else if !strings.Contains(err.Error(), "already exists") {
			// if the error is not "vote already exists", we need to print it
			log.Warn(err)
		}
	}

	if t.election.VoteMode.Anonymous {
		apiClientMtx.Unlock()
	}
	return contextDeadline, nil
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

// matchResult compare the expected vote results base in the overwrite applied, with the actual result returned by the API
func matchResult(results [][]*types.BigInt, expectedResult [][]string) bool {
	// only has 1 question
	for q := range results[0] {
		if !(expectedResult[0][q] == results[0][q].String()) {
			return false
		}
	}
	return true
}
