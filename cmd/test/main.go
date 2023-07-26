package main

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type test struct {
	client   *apiclient.HTTPclient
	admin    *ethereum.SignKeys
	accounts []*ethereum.SignKeys
}

func printableAccount(acc *ethereum.SignKeys) []interface{} {
	pKey := acc.PrivateKey()
	signature, _ := acc.SIKsignature()
	sik, _ := acc.AccountSIK(nil)

	return []interface{}{
		"address",
		acc.AddressString(),
		"privKey",
		pKey.String(),
		"signature",
		fmt.Sprintf("%x", signature),
		"sik",
		fmt.Sprintf("%x", sik),
	}
}

func (t *test) registerSIK(acc *ethereum.SignKeys, weight, electionId, censusProof []byte) error {
	privKey := acc.PrivateKey()
	client := t.client.Clone(privKey.String())
	sik, err := acc.AccountSIK(nil)
	if err != nil {
		return err
	}

	txPayload, err := proto.Marshal(&models.Tx{
		Payload: &models.Tx_RegisterSik{
			RegisterSik: &models.RegisterSIKTx{
				Sik:        sik,
				ElectionId: electionId,
				CensusProof: &models.Proof{
					Payload: &models.Proof_Arbo{
						Arbo: &models.ProofArbo{
							Type:            models.ProofArbo_POSEIDON,
							Siblings:        censusProof,
							AvailableWeight: weight,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	hash, _, err := client.SignAndSendTx(&models.SignedTx{Tx: txPayload})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if _, err := client.WaitUntilTxIsMined(ctx, hash); err != nil {
		return err
	}
	return nil
}

func (t *test) createAccount(acc *ethereum.SignKeys) (*apiclient.HTTPclient, error) {
	pKey := acc.PrivateKey()
	client := t.client.Clone(pKey.String())

	faucetEndpoint := fmt.Sprintf("http://0.0.0.0:9090/v2/faucet/dev/%s", acc.AddressString())
	faucetPkg, err := apiclient.GetFaucetPackageFromRemoteService(faucetEndpoint, "")
	if err != nil {
		return nil, err
	}
	accountMetadata := &api.AccountMetadata{
		Name:        map[string]string{"default": "test account " + acc.AddressString()},
		Description: map[string]string{"default": "test description"},
		Version:     "1.0",
	}
	hash, err := client.AccountBootstrap(faucetPkg, accountMetadata, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if _, err := client.WaitUntilTxIsMined(ctx, hash); err != nil {
		return nil, err
	}
	return client, nil
}

func main() {
	log.Init("info", "stdout", nil)

	// Init client
	token := uuid.New()
	host, err := url.Parse("http://0.0.0.0:9090/v2")
	if err != nil {
		log.Fatal(err)
	}
	t := &test{}
	t.client, err = apiclient.NewHTTPclient(host, &token)
	if err != nil {
		log.Fatal(err)
	}

	// Create admin account
	t.admin = ethereum.NewSignKeys()
	if err := t.admin.Generate(); err != nil {
		log.Fatal(err)
	}
	if t.client, err = t.createAccount(t.admin); err != nil {
		log.Fatal(err)
	}
	log.Infow("admin account created", printableAccount(t.admin)...)

	// Mock accounts and create census
	t.accounts = ethereum.NewSignKeysBatch(10)
	if err := t.accounts[4].AddHexKey("0xc027f699648bdb42c46a9e81ca7874f762d3bfe3fea78693463f80c8e6d077a3"); err != nil {
		log.Fatal(err)
	}
	participants := &api.CensusParticipants{}
	for _, acc := range t.accounts {
		participants.Participants = append(participants.Participants,
			api.CensusParticipant{
				Key:    acc.Address().Bytes(),
				Weight: (*types.BigInt)(new(big.Int).SetUint64(1)),
			},
		)
	}
	censusId, err := t.client.NewCensus(api.CensusTypeZKWeighted)
	if err != nil {
		log.Fatal(err)
	}
	if err := t.client.CensusAddParticipants(censusId, participants); err != nil {
		log.Fatal(err)
	}
	censusRoot, censusURI, err := t.client.CensusPublish(censusId)
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("census created and published",
		"censusRoot", censusRoot.String(),
		"censusUri", censusURI)

	// Create an election
	electionData := &api.ElectionDescription{
		Title:       map[string]string{"default": fmt.Sprintf("Test election %s", util.RandomHex(8))},
		Description: map[string]string{"default": "Test election description"},
		EndDate:     time.Now().Add(time.Minute * 20),
		ElectionType: api.ElectionType{
			Autostart:     true,
			Interruptible: true,
			Anonymous:     true,
		},
		Census: api.CensusTypeDescription{
			Type:     api.CensusTypeZKWeighted,
			URL:      censusURI,
			RootHash: censusRoot,
			Size:     uint64(len(t.accounts)),
		},
		VoteType: api.VoteType{MaxVoteOverwrites: 1},
		Questions: []api.Question{
			{
				Title:       map[string]string{"default": "Test question 1"},
				Description: map[string]string{"default": "Test question 1 description"},
				Choices: []api.ChoiceMetadata{
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
	electionId, err := t.client.NewElection(electionData)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*80)
	defer cancel()
	election, err := t.client.WaitUntilElectionStarts(ctx, electionId)
	if err != nil {
		log.Fatal(err)
	}
	election.ElectionID = electionId
	log.Infow("election created", "electionId", electionId.String())

	// Take a random voter, get his census proof and register his sik to vote
	randomVoter := t.accounts[4]
	censusProof, err := t.client.CensusGenProof(censusId, randomVoter.Address().Bytes())
	if err != nil {
		log.Fatal(err)
	}
	if err := t.registerSIK(
		randomVoter,
		arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), big.NewInt(1)),
		electionId,
		censusProof.Proof,
	); err != nil && !strings.Contains(err.Error(), "the sik could not be changed yet") {
		log.Fatal(err)
	}
	log.Infow("voter prepared", printableAccount(randomVoter)...)

	// Calculate sik proof
	voterPrivKey := randomVoter.PrivateKey()
	voterClient := t.client.Clone(voterPrivKey.String())
	sikProof, err := voterClient.GenSIKProof()
	if err != nil {
		log.Fatal(err)
	}

	// Load the circuit and generate the proof
	circuitInputs, err := circuit.GenerateCircuitInput(randomVoter, nil,
		electionId, censusProof.Root, sikProof.Root, censusProof.Siblings,
		sikProof.Siblings, big.NewInt(1), big.NewInt(1))
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("circuit inputs generated", "inputs", circuitInputs.String())

	// Compose the vote tx and send it
	voteId, err := voterClient.Vote(&apiclient.VoteData{
		ElectionID:   electionId,
		ProofMkTree:  censusProof,
		ProofSIKTree: sikProof,
		Choices:      []int{0},
		VoterAccount: randomVoter,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Infow("vote emitted", "nullifier", voteId)

	// Ending the election and getting the results
	if _, err := t.client.SetElectionStatus(electionId, "ENDED"); err != nil {
		log.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*40)
	defer cancel()
	results, err := t.client.WaitUntilElectionResults(ctx, electionId)
	if err != nil {
		log.Fatal(err)
	}

	log.Infow("election finished and results published", "results", results.Results)
}
