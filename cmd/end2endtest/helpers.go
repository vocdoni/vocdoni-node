package main

import (
	"context"
	"fmt"
	"math/big"
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

type censusIdentification struct {
	id    types.HexBytes
	typeC string
}

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

func getFaucetPkg(c *config, myAddress string) (*models.FaucetPackage, error) {
	var (
		err       error
		faucetPkg *models.FaucetPackage
	)

	if c.faucet != "" {
		// Get the faucet package of bootstrap tokens
		log.Infof("getting faucet package")
		if c.faucet == "dev" {
			faucetPkg, err = apiclient.GetFaucetPackageFromDevService(myAddress)
		} else {
			faucetPkg, err = apiclient.GetFaucetPackageFromRemoteService(c.faucet+myAddress, c.faucetAuthToken)
		}

		if err != nil {
			return nil, err
		}
	}

	log.Debugf("faucetPackage is %x", faucetPkg)

	return faucetPkg, err
}

func createAccount(api *apiclient.HTTPclient, c *config, address string, faucetPkg *models.FaucetPackage) error {
	log.Infof("creating Vocdoni account %s", address)
	hash, err := api.AccountBootstrap(faucetPkg, newDefaultAccountMetadata(address))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()

	if _, err := api.WaitUntilTxIsMined(ctx, hash); err != nil {
		log.Error("gave up waiting for tx %x to be mined: %s", hash, err)
		return err
	}

	// check the account
	acc, err := api.Account("")
	if err != nil {
		return err
	}
	if c.faucet != "" && acc.Balance == 0 {
		log.Error("account balance is 0")
	}
	return nil
}

func addParticipantsToCensus(api *apiclient.HTTPclient, censusType string, voterAccounts []*ethereum.SignKeys, censusID types.HexBytes) error {
	// Add the  accounts to the census by batches
	participants := &vapi.CensusParticipants{}
	for i, voterAccount := range voterAccounts {
		keyAddr, err := getCensusParticipantKey(voterAccount, censusType)
		if err != nil {
			return err
		}
		participants.Participants = append(participants.Participants,
			vapi.CensusParticipant{
				Key:    keyAddr,
				Weight: (*types.BigInt)(new(big.Int).SetUint64(10)),
			})
		if i == len(voterAccounts)-1 || ((i+1)%vapi.MaxCensusAddBatchSize == 0) {
			if err := api.CensusAddParticipants(censusID, participants); err != nil {
				return err
			}
			log.Infof("added %d participants to census %s",
				len(participants.Participants), censusID.String())
			participants = &vapi.CensusParticipants{}
		}
	}
	return nil
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

func isCensusSizeValid(api *apiclient.HTTPclient, censusID types.HexBytes, nvotes int) error {
	size, err := api.CensusSize(censusID)
	if err != nil {
		return err
	}
	if size != uint64(nvotes) {
		log.Fatalf("census size is %d, expected %d", size, nvotes)
	}
	log.Infof("census %s size is %d", censusID.String(), size)
	return nil
}

func createElection(api *apiclient.HTTPclient, electionDescrip *vapi.ElectionDescription) error {
	// Create a new Election
	electionID, err := api.NewElection(electionDescrip)
	if err != nil {
		return err
	}

	log.Infof("created new election with id %s - now wait until it starts", electionID.String())

	// Wait for the election to start
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	election, err := api.WaitUntilElectionStarts(ctx, electionID)
	if err != nil {
		return err
	}

	log.Debugf("election details: %+v", *election)
	return nil

}
