package main

import (
	"context"
	"fmt"
	"math/big"
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

func newDefaultElectionDescription(root types.HexBytes, censusURI string, size uint64) *vapi.ElectionDescription {
	return &vapi.ElectionDescription{
		Title:       map[string]string{"default": fmt.Sprintf("Test election %s", util.RandomHex(8))},
		Description: map[string]string{"default": "Test election description"},
		EndDate:     time.Now().Add(time.Minute * 20),

		VoteType: vapi.VoteType{
			UniqueChoices:     false,
			MaxVoteOverwrites: 1,
		},

		ElectionType: vapi.ElectionType{
			Autostart:         true,
			Interruptible:     true,
			Anonymous:         false,
			SecretUntilTheEnd: false,
			DynamicCensus:     false,
		},

		Census: vapi.CensusTypeDescription{
			RootHash: root,
			URL:      censusURI,
			Type:     "weighted",
			Size:     size,
		},

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

func newDefaultAccountMetadata(address string) *vapi.AccountMetadata {
	return &vapi.AccountMetadata{
		Name:        map[string]string{"default": "test account " + address},
		Description: map[string]string{"default": "test description"},
		Version:     "1.0",
	}
}

func (t electionBase) createAccount(faucetPkg *models.FaucetPackage, address string) error {
	log.Infof("creating Vocdoni account %s", address)
	hash, err := t.api.AccountBootstrap(faucetPkg, newDefaultAccountMetadata(address))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()

	if _, err := t.api.WaitUntilTxIsMined(ctx, hash); err != nil {
		log.Error("gave up waiting for tx %x to be mined: %s", hash, err)
		return err
	}

	// check the account
	acc, err := t.api.Account("")
	if err != nil {
		return err
	}
	if t.config.faucet != "" && acc.Balance == 0 {
		log.Error("account balance is 0")
	}
	return nil
}

func (t electionBase) addParticipantsCensus(censusType string, censusID types.HexBytes) error {
	// Add the  accounts to the census by batches
	participants := &vapi.CensusParticipants{}

	for i, voterAccount := range t.voterAccounts {
		keyAddr, err := getCensusParticipantKey(voterAccount, censusType)
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

func (t electionBase) isCensusSizeValid(censusID types.HexBytes) error {
	size, err := t.api.CensusSize(censusID)
	if err != nil {
		return err
	}
	if size != uint64(t.config.nvotes) {
		// TODO: fix log.Error call
		log.Error("census size is %d, expected %d", size, t.config.nvotes)
	}
	log.Infof("census %s size is %d", censusID.String(), size)
	return nil
}

func (t electionBase) createElection(electionDescrip *vapi.ElectionDescription) error {
	// Create a new Election
	electionID, err := t.api.NewElection(electionDescrip)
	if err != nil {
		return err
	}

	log.Infof("created new election with id %s - now wait until it starts", electionID.String())

	// Wait for the election to start
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	election, err := t.api.WaitUntilElectionStarts(ctx, electionID)
	if err != nil {
		return err
	}

	log.Debugf("election details: %+v", *election)
	return nil

}

// plaintext
func (t electionBase) generateProofs(root types.HexBytes) {
	// Generate the voting proofs (parallelized)
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

	addNaccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("generating %d voting proofs", len(accounts))
		for _, acc := range accounts {
			pr, err := t.api.CensusGenProof(root, acc.Address().Bytes())
			if err != nil {
				log.Fatal(err)
			}
			pr.KeyType = models.ProofArbo_ADDRESS
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
}

func getFaucetPkg(faucet, myAddress, faucetAuthToken string) (*models.FaucetPackage, error) {
	var (
		err       error
		faucetPkg *models.FaucetPackage
	)

	if faucet != "" {
		// Get the faucet package of bootstrap tokens
		log.Infof("getting faucet package")
		if faucet == "dev" {
			faucetPkg, err = apiclient.GetFaucetPackageFromDevService(myAddress)
		} else {
			faucetPkg, err = apiclient.GetFaucetPackageFromRemoteService(faucet+myAddress, faucetAuthToken)
		}

		if err != nil {
			return nil, err
		}
	}

	log.Debugf("faucetPackage is %x", faucetPkg)

	return faucetPkg, err
}

func getCensusParticipantKey(voterAccount *ethereum.SignKeys, censusType string) ([]byte, error) {
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
