package main

import (
	"os"
	"sync"
	"time"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
)

func init() {
	ops["census"] = operation{
		test:        &E2ECensus{},
		description: "Create a census to benchmark the performance of the merkle tree construction",
		example:     os.Args[0] + " --operation=census --votes=1000",
	}
}

var _ VochainTest = (*E2ECensus)(nil)

type E2ECensus struct {
	parallelCount int
	censusID      types.HexBytes
	accounts      []*ethereum.SignKeys
	api           *apiclient.HTTPclient
	batchSize     int
}

func (t *E2ECensus) Setup(api *apiclient.HTTPclient, c *config) error {
	log.Info("creating census")
	var err error
	t.parallelCount = c.parallelCount
	t.censusID, err = api.NewCensus(vapi.CensusTypeWeighted)
	if err != nil {
		return err
	}
	t.accounts = ethereum.NewSignKeysBatch(c.nvotes)
	t.api = api
	t.batchSize = 250 // TODO: make this configurable
	log.Infow("census created",
		"censusID", t.censusID,
		"type", vapi.CensusTypeWeighted,
		"bearer", api.BearerToken().String(),
	)
	return nil
}

func (*E2ECensus) Teardown() error {
	// TODO: delete census
	return nil
}

func (t *E2ECensus) Run() error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, t.parallelCount) // Semaphore pattern to limit active goroutines

	addParticipants := func(from, to int) {
		defer wg.Done() // Signal that this routine is done at the end

		batch := vapi.CensusParticipants{}
		for i := from; i < to; i++ {
			batch.Participants = append(batch.Participants, vapi.CensusParticipant{
				Key:    t.accounts[i].Address().Bytes(),
				Weight: new(types.BigInt).SetUint64(1),
			})
		}
		if err := t.api.CensusAddParticipants(t.censusID, &batch); err != nil {
			log.Errorw(err, "error adding participants")
		}
		<-sem // Release the slot for another goroutine
	}

	consumed := 0
	startTime := time.Now()
	for consumed < len(t.accounts) {
		sem <- struct{}{} // Acquire a slot in the buffer
		wg.Add(1)

		// Calculate the range this goroutine will handle
		nextConsumed := consumed + t.batchSize
		if nextConsumed > len(t.accounts) {
			nextConsumed = len(t.accounts)
		}
		consumedCopy := consumed
		go addParticipants(consumedCopy, nextConsumed)
		consumed = nextConsumed
		log.Infow("added participants", "from", consumedCopy, "to", nextConsumed)
	}

	wg.Wait()  // Wait for all goroutines to finish
	close(sem) // Close the semaphore channel
	log.Infow("census created", "took (s)", time.Since(startTime).Seconds(), "participants/second", float64(len(t.accounts))/time.Since(startTime).Seconds())
	startTime = time.Now()
	newCensusID, uri, err := t.api.CensusPublish(t.censusID)
	if err != nil {
		return err
	}
	log.Infow("census published", "censusID", newCensusID.String(), "uri", uri, "took (s)", time.Since(startTime).Seconds())
	return nil
}
