package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"

	flag "github.com/spf13/pflag"

	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

func main() {
	host := flag.String("host", "https://api.vocdoni.io/v2", "API host")
	decKeys := flag.StringSlice("keys", nil, "Decryption keys (hex)")
	electionIdStr := flag.String("election", "", "Election ID")
	flag.Parse()

	decryptionkeys := []string{}
	for _, key := range *decKeys {
		decryptionkeys = append(decryptionkeys, key)
	}

	if *electionIdStr == "" {
		panic("missing election ID")
	}
	log.Init("debug", "stdout", nil)

	api, err := apiclient.New(*host)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("connected to network", "chainID", api.ChainID())

	electionId, err := hex.DecodeString(*electionIdStr)
	if err != nil {
		log.Fatal(err)
	}

	// Get election
	election, err := api.Election(electionId)
	if err != nil {
		log.Fatal(err)
	}

	data, err := json.MarshalIndent(election, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("election: %s", string(data))

	votes, err := api.Votes(electionId, 3)
	if err != nil {
		log.Fatal(err)
	}

	voteOptions := &models.ProcessVoteOptions{
		MaxCount:     election.TallyMode.MaxCount,
		MaxValue:     election.TallyMode.MaxValue,
		CostExponent: election.TallyMode.CostExponent,
	}

	envelopeType := &models.EnvelopeType{
		EncryptedVotes: election.VoteMode.EncryptedVotes,
		Anonymous:      election.VoteMode.Anonymous,
		UniqueValues:   election.VoteMode.UniqueValues,
		CostFromWeight: election.VoteMode.CostFromWeight,
	}

	results := &results.Results{
		Votes:        results.NewEmptyVotes(voteOptions),
		ProcessID:    electionId,
		Weight:       new(types.BigInt).SetUint64(0),
		VoteOpts:     voteOptions,
		EnvelopeType: envelopeType,
	}

	for _, vote := range votes {
		keys := []string{}
		for _, ki := range vote.EncryptionKeyIndexes {
			keys = append(keys, decryptionkeys[ki])
		}
		vp, err := unmarshalVote(vote.VotePackage, keys)
		if err != nil {
			log.Warnf("cannot decrypt vote: %v", err)
			continue
		}
		weight, ok := new(big.Int).SetString(vote.VoteWeight, 10)
		if !ok {
			log.Warnf("cannot parse weight: %s", vote.VoteWeight)
			continue
		}
		if err = results.AddVote(vp.Votes, weight, nil); err != nil {
			log.Warnf("addVote failed: %v", err)
			continue
		}
	}

	data, err = json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("results: %s", string(data))
}

// unmarshalVote decodes the base64 payload to a VotePackage struct type.
// If the vochain.VotePackage is encrypted the list of keys to decrypt it should be provided.
// The order of the Keys must be as it was encrypted.
// The function will reverse the order and use the decryption keys starting from the
// last one provided.
func unmarshalVote(VotePackage []byte, keys []string) (*state.VotePackage, error) {
	var vote state.VotePackage
	rawVote := bytes.Clone(VotePackage)
	// if encryption keys, decrypt the vote
	for i, key := range slices.Backward(keys) {
		priv, err := nacl.DecodePrivate(key)
		if err != nil {
			return nil, fmt.Errorf("cannot create private key cipher: (%s)", err)
		}
		if rawVote, err = priv.Decrypt(rawVote); err != nil {
			return nil, fmt.Errorf("cannot decrypt vote with index key %d: %w", i, err)
		}
	}
	if err := vote.Decode(rawVote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote: %w", err)
	}
	return &vote, nil
}
