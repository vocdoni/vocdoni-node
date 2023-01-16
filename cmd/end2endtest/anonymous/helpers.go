package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/vocdoni/arbo"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

func ensureTxIsMined(api *apiclient.HTTPclient, txHash types.HexBytes) {
	for startTime := time.Now(); time.Since(startTime) < 40*time.Second; {
		_, err := api.TransactionReference(txHash)
		if err == nil {
			return
		}
		time.Sleep(4 * time.Second)
	}
	log.Fatalf("tx %s not mined", txHash.String())
}

func generateAccounts(number int) ([]*ethereum.SignKeys, error) {
	accounts := make([]*ethereum.SignKeys, number)
	for i := 0; i < number; i++ {
		accounts[i] = &ethereum.SignKeys{}
		if err := accounts[i].Generate(); err != nil {
			return nil, err
		}
	}
	return accounts, nil
}

func ensureElectionCreated(api *apiclient.HTTPclient, electionID types.HexBytes) *vapi.Election {
	for startTime := time.Now(); time.Since(startTime) < time.Second*40; {
		election, _ := api.Election(electionID)
		if election != nil {
			return election
		}
		time.Sleep(time.Second * 4)
	}
	log.Fatalf("election %s not created", electionID.String())
	return nil
}

func waitUntilHeight(api *apiclient.HTTPclient, height uint32) {
	for {
		info, err := api.ChainInfo()
		if err != nil {
			log.Warn(err)
		} else {
			if *info.Height > height {
				break
			}
		}
		time.Sleep(time.Second * 4)
	}
}

func waitUntilElectionStarts(api *apiclient.HTTPclient, electionID types.HexBytes) {
	election, err := api.Election(electionID)
	if err != nil {
		log.Fatal(err)
	}
	startHeight, err := api.DateToHeight(election.StartDate)
	if err != nil {
		log.Fatal(err)
	}
	waitUntilHeight(api, startHeight+1) // add a block to be sure
}

func waitUntilElectionStatus(api *apiclient.HTTPclient, electionID types.HexBytes, status string) {
	for startTime := time.Now(); time.Since(startTime) < time.Second*300; {
		election, err := api.Election(electionID)
		if err != nil {
			log.Fatal(err)
		}
		if election.Status == status {
			return
		}
		time.Sleep(time.Second * 5)
	}
	log.Fatalf("election status %s not reached", status)
}

func buildCensusZk(api *apiclient.HTTPclient, accounts []*ethereum.SignKeys) (types.HexBytes, types.HexBytes, string, error) {
	nvotes := len(accounts)

	// Create a new census
	censusID, err := api.NewCensus(vapi.CensusTypeZKWeighted)
	if err != nil {
		return nil, nil, "", err
	}
	log.Infow("new census created", map[string]any{"censusID": censusID.String()})

	// Add the accounts to the census by batches
	participants := &vapi.CensusParticipants{}
	for i, voterAccount := range accounts {
		// Calculate a BabyJubJub key from the current voter private key to
		// use the public part of the generated key as leaf key.
		censusKey, err := calcAnonPubKey(voterAccount)
		if err != nil {
			return nil, nil, "", err
		}

		// Create the participants with the correct key and a weight and send to
		// the api
		participants.Participants = append(participants.Participants,
			vapi.CensusParticipant{
				Key:    censusKey,
				Weight: (*types.BigInt)(new(big.Int).SetUint64(10)),
			})
		if i == nvotes-1 || ((i+1)%vapi.MaxCensusAddBatchSize == 0) {
			if err := api.CensusAddParticipants(censusID, participants); err != nil {
				return nil, nil, "", err
			}
			log.Infow("new participants added to census", map[string]any{
				"censusID":        censusID.String(),
				"newParticipants": len(participants.Participants),
			})
			participants = &vapi.CensusParticipants{}
		}
	}

	// Check census size
	size, err := api.CensusSize(censusID)
	if err != nil {
		return nil, nil, "", err
	}
	if size != uint64(nvotes) {
		return nil, nil, "", fmt.Errorf("census size is %d, expected %d", size, nvotes)
	}
	log.Infow("finish census participants registration", map[string]any{
		"censusID":        censusID.String(),
		"newParticipants": len(participants.Participants),
		"censusSize":      size,
	})

	// Publish the census
	censusRoot, censusURI, err := api.CensusPublish(censusID)
	if err != nil {
		return nil, nil, "", err
	}
	log.Infow("census published", map[string]any{
		"censusID":   censusID.String(),
		"censusRoot": censusRoot.String(),
	})

	// Check census size (of the published census)
	size, err = api.CensusSize(censusRoot)
	if err != nil {
		return nil, nil, "", err
	}
	if size != uint64(nvotes) {
		return nil, nil, "", fmt.Errorf("published census size is %d, expected %d", size, nvotes)
	}

	return censusID, censusRoot, censusURI, nil
}

func calcAnonPrivKey(account *ethereum.SignKeys) (babyjub.PrivateKey, error) {
	privKey := babyjub.PrivateKey{}
	_, strKey := account.HexString()
	if _, err := hex.Decode(privKey[:], []byte(strKey)); err != nil {
		return babyjub.PrivateKey{}, fmt.Errorf("error generating babyjub key: %w", err)
	}

	return privKey, nil
}

func calcAnonPubKey(account *ethereum.SignKeys) (types.HexBytes, error) {
	privKey := babyjub.PrivateKey{}
	_, strKey := account.HexString()
	if _, err := hex.Decode(privKey[:], []byte(strKey)); err != nil {
		return nil, fmt.Errorf("error generating babyjub key: %w", err)
	}

	pubKey, err := poseidon.Hash([]*big.Int{
		privKey.Public().X,
		privKey.Public().Y,
	})
	if err != nil {
		return nil, fmt.Errorf("error hashing babyjub public key: %w", err)
	}

	return arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), pubKey), nil
}
