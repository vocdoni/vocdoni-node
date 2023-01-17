package main

import (
	"fmt"
	"math/big"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

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
		privKey, err := apiclient.BabyJubJubPrivKey(voterAccount)
		if err != nil {
			return nil, nil, "", err
		}
		censusKey, err := apiclient.BabyJubJubPubKey(privKey)
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
