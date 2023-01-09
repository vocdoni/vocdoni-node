package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/vocdoni/arbo"
	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func buildCensus(api *apiclient.HTTPclient, accounts []*ethereum.SignKeys, anonymous bool) (types.HexBytes, types.HexBytes, string, error) {
	nvotes := len(accounts)
	censusType := vapi.CensusTypeWeighted
	if anonymous {
		censusType = vapi.CensusTypeZKWeighted
	}

	// Create a new census
	censusID, err := api.NewCensus(censusType)
	if err != nil {
		return nil, nil, "", err
	}
	log.Infow("new census created", map[string]any{"censusID": censusID.String()})

	// Add the accounts to the census by batches
	participants := &vapi.CensusParticipants{}
	for i, voterAccount := range accounts {
		// By default use the account address as participant key into the arbo
		// merkle tree
		censusKey := voterAccount.Address().Bytes()
		if anonymous {
			// If the election is anonymous, calculate a BabyJubJub key from the
			// current voter private key to use the public part of the generated
			// key as leaf key.
			censusKey, err = calcAnonPubKey(voterAccount)
			if err != nil {
				return nil, nil, "", err
			}
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

func calcAnonPubKey(account *ethereum.SignKeys) (types.HexBytes, error) {
	privKey := babyjub.PrivateKey{}
	_, err := hex.Decode(privKey[:], account.PrivateKey())
	if err != nil {
		return nil, fmt.Errorf("error generating babyjub key")
	}

	pubKey, err := poseidon.Hash([]*big.Int{
		privKey.Public().X,
		privKey.Public().Y,
	})
	if err != nil {
		return nil, fmt.Errorf("error hashing babyjub public key")
	}

	bLen := arbo.HashFunctionPoseidon.Len()
	return arbo.BigIntToBytes(bLen, pubKey), nil
}

func TestWeightedVoting(t *testing.T) {
	host := flag.String("host", "https://api-dev.vocdoni.net/v2", "API host to connect to")
	logLevel := flag.String("logLevel", "info", "log level (debug, info, warn, error, fatal)")
	accountPrivKey := flag.String("accountPrivKey", "0xf7FB77ee1F309D9468fB6DCB71aDD0f934a33c6B", "account private key (optional)")
	nvotes := flag.Int("votes", 10, "number of votes to cast")
	parallelCount := flag.Int("parallel", 4, "number of parallel requests")
	useDevFaucet := flag.Bool("devFaucet", true, "use the dev faucet for fetching tokens")
	timeout := flag.Int("timeout", 5, "timeout in minutes")
	anonymous := flag.Bool("anonymous", false, "use for test anonymous weighted or only weighted voting")

	flag.Parse()
	log.Init(*logLevel, "stdout")

	c := qt.New(t)
	hostURL, err := url.Parse(*host)
	c.Assert(err, qt.IsNil)
	log.Debugf("connecting to %s", hostURL.String())

	token := uuid.New()
	api, err := apiclient.NewHTTPclient(hostURL, &token)
	c.Assert(err, qt.IsNil)

	// Check if account is defined
	account := *accountPrivKey
	if account == "" {
		// Generate the organization account
		account = util.RandomHex(32)
		log.Infof("new account generated, private key is %s", account)
	}

	// Set the account in the API client, so we can sign transactions
	err = api.SetAccount(account)
	c.Assert(err, qt.IsNil)

	// If the account does not exist, create a new one
	// TODO: check if the account balance is low and use the faucet
	acc, err := api.Account("")
	if err != nil {
		var faucetPkg *models.FaucetPackage
		if *useDevFaucet {
			// Get the faucet package of bootstrap tokens
			log.Infof("getting faucet package")
			faucetPkg, err = apiclient.GetFaucetPackageFromRemoteService(
				apiclient.DefaultDevelopmentFaucetURL+api.MyAddress().Hex(),
				apiclient.DefaultDevelopmentFaucetToken,
			)
			c.Assert(err, qt.IsNil)
		}
		// Create the organization account and bootstraping with the faucet package
		log.Infof("creating Vocdoni account %s", api.MyAddress().Hex())
		log.Debugf("faucetPackage is %x", faucetPkg)
		hash, err := api.AccountBootstrap(faucetPkg, &vapi.AccountMetadata{
			Name:        map[string]string{"default": "test account " + api.MyAddress().Hex()},
			Description: map[string]string{"default": "test description"},
			Version:     "1.0",
		})
		c.Assert(err, qt.IsNil)

		ensureTxIsMined(api, hash)
		acc, err = api.Account("")
		c.Assert(err, qt.IsNil)

		if *useDevFaucet && acc.Balance == 0 {
			log.Fatal("account balance is 0")
		}
	}

	log.Infof("account %s balance is %d", api.MyAddress().Hex(), acc.Balance)

	// Create a new census
	voterAccounts, err := generateAccounts(*nvotes)
	c.Assert(err, qt.IsNil)

	_, censusRoot, censusURI, err := buildCensus(api, voterAccounts, *anonymous)
	c.Assert(err, qt.IsNil)

	// Generate the voting proofs (parallelized)
	type voterProof struct {
		proof   *apiclient.CensusProof
		address string
	}
	proofs := make(map[string]*apiclient.CensusProof, *nvotes)
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
			keyType := models.ProofArbo_ADDRESS
			key := acc.Address().Bytes()
			if *anonymous {
				key, err = calcAnonPubKey(acc)
				c.Assert(err, qt.IsNil)
				keyType = models.ProofArbo_PUBKEY
			}
			pr, err := api.CensusGenProof(censusRoot, key)
			c.Assert(err, qt.IsNil)
			pr.KeyType = keyType
			proofCh <- &voterProof{
				proof:   pr,
				address: acc.Address().Hex(),
			}
		}
	}

	pcount := *nvotes / *parallelCount
	var wg sync.WaitGroup
	for i := 0; i < len(voterAccounts); i += pcount {
		end := i + pcount
		if end > len(voterAccounts) {
			end = len(voterAccounts)
		}
		wg.Add(1)
		go addNaccounts(voterAccounts[i:end], &wg)
	}

	wg.Wait()
	time.Sleep(time.Second) // wait a grace time for the last proof to be added
	log.Debugf("%d/%d voting proofs generated successfully", len(proofs), len(voterAccounts))
	stopProofs <- true

	// Create a new Election
	censusType := vapi.CensusTypeWeighted
	if *anonymous {
		censusType = vapi.CensusTypeZKWeighted
	}

	electionID, err := api.NewElection(&vapi.ElectionDescription{
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
			Anonymous:         *anonymous,
			SecretUntilTheEnd: false,
			DynamicCensus:     false,
		},

		Census: vapi.CensusTypeDescription{
			RootHash: censusRoot,
			URL:      censusURI,
			Type:     censusType,
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
	})
	c.Assert(err, qt.IsNil)

	election := ensureElectionCreated(api, electionID)
	log.Infof("created new election with id %s", electionID.String())
	log.Debugf("election details: %+v", *election)

	// Wait for the election to start
	waitUntilElectionStarts(api, electionID)

	// Send the votes (parallelized)
	startTime := time.Now()
	wg = sync.WaitGroup{}
	voteAccounts := func(accounts []*ethereum.SignKeys, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("sending %d votes", len(accounts))
		// We use maps instead of slices to have the capacity of resending votes
		// without repeating them.
		accountsMap := make(map[int]*ethereum.SignKeys, len(accounts))
		for i, acc := range accounts {
			accountsMap[i] = acc
		}
		// Send the votes
		votesSent := 0
		for {
			contextDeadlines := 0
			for i, voterAccount := range accountsMap {
				c := api.Clone(fmt.Sprintf("%x", voterAccount.PrivateKey()))
				_, err := c.Vote(&apiclient.VoteData{
					ElectionID:  electionID,
					ProofMkTree: proofs[voterAccount.Address().Hex()],
					Choices:     []int{i % 2},
				})
				// if the context deadline is reached, we don't need to print it (let's jus retry)
				if err != nil && errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
					contextDeadlines++
					continue
				} else if err != nil && !strings.Contains(err.Error(), "already exists") {
					// if the error is not "vote already exists", we need to print it
					log.Warn(err)
					continue
				}
				// if the vote was sent successfully or already exists, we remove it from the accounts map
				votesSent++
				delete(accountsMap, i)

			}
			if len(accountsMap) == 0 {
				break
			}
			log.Infof("sent %d/%d votes... got %d HTTP errors", votesSent, len(accounts), contextDeadlines)
			time.Sleep(time.Second * 5)
		}
		log.Infof("successfully sent %d votes", votesSent)
		time.Sleep(time.Second * 2)
	}

	pcount = *nvotes / *parallelCount
	for i := 0; i < len(voterAccounts); i += pcount {
		end := i + pcount
		if end > len(voterAccounts) {
			end = len(voterAccounts)
		}
		wg.Add(1)
		go voteAccounts(voterAccounts[i:end], &wg)
	}

	wg.Wait()
	log.Infof("%d votes submitted successfully, took %s. At %d votes/second",
		*nvotes, time.Since(startTime), int(float64(*nvotes)/time.Since(startTime).Seconds()))

	// Wait for all the votes to be verified
	log.Infof("waiting for all the votes to be registered...")
	for {
		count, err := api.ElectionVoteCount(electionID)
		if err != nil {
			log.Warn(err)
		}
		if count == uint32(*nvotes) {
			break
		}
		time.Sleep(time.Second * 5)
		log.Infof("verified %d/%d votes", count, *nvotes)
		if time.Since(startTime) > time.Duration(*timeout)*time.Minute {
			log.Fatalf("timeout waiting for votes to be registered")
		}
	}

	log.Infof("%d votes registered successfully, took %s (%d votes/second)",
		*nvotes, time.Since(startTime), int(float64(*nvotes)/time.Since(startTime).Seconds()))

	// Set the account back to the organization account
	err = api.SetAccount(account)
	c.Assert(err, qt.IsNil)

	// End the election by seting the status to ENDED
	log.Infof("ending election...")
	hash, err := api.SetElectionStatus(electionID, "ENDED")
	c.Assert(err, qt.IsNil)

	// Check the election status is actually ENDED
	ensureTxIsMined(api, hash)
	election, err = api.Election(electionID)
	c.Assert(err, qt.IsNil)
	c.Assert(election.Status, qt.Not(qt.Equals), "ENDED")
	log.Infof("election %s status is ENDED", electionID.String())

	// Wait for the election to be in RESULTS state
	log.Infof("waiting for election to be in RESULTS state...")
	waitUntilElectionStatus(api, electionID, "RESULTS")
	log.Infof("election %s status is RESULTS", electionID.String())

	election, err = api.Election(electionID)
	c.Assert(err, qt.IsNil)
	log.Infof("election results: %v", election.Results)
}
