package vocone

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.vocdoni.io/dvote/client"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

func TestVocone(t *testing.T) {
	//log.Init("info", "stdout")
	dir := t.TempDir()

	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		t.Fatal(err)
	}
	vc, err := NewVocone(dir, &oracle)
	if err != nil {
		t.Fatal(err)
	}
	vc.SetBlockTimeTarget(time.Millisecond * 500)
	go vc.Start()
	port := 13000 + util.RandomInt(0, 2000)
	vc.EnableAPI("127.0.0.1", port, "/dvote")
	time.Sleep(time.Second * 2) // TODO: find a more smart way to wait until everything is ready
	if err := testCSPvote(&oracle, fmt.Sprintf("http://127.0.0.1:%d/dvote", port)); err != nil {
		t.Fatal(err)
	}
}

func testCSPvote(oracle *ethereum.SignKeys, url string) error {
	cli, err := client.New(url)
	if err != nil {
		return err
	}
	cspKey := ethereum.SignKeys{}
	cspKey.Generate()
	entityID := util.RandomBytes(20)
	censusRoot := cspKey.PublicKey()
	processID := util.RandomBytes(32)
	envelope := new(models.EnvelopeType)
	censusOrigin := models.CensusOrigin_OFF_CHAIN_CA
	duration := 100
	censusSize := 10
	startBlock, err := cli.CreateProcess(
		oracle,
		entityID,
		censusRoot,
		"",
		processID,
		envelope,
		nil,
		censusOrigin,
		0,
		duration,
		uint64(censusSize),
	)
	if err != nil {
		return err
	}

	voterKeys := util.CreateEthRandomKeysBatch(censusSize)
	proofs, err := cli.GetCSPproofBatch(voterKeys, &cspKey, processID)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	elapsedTime, err := cli.TestSendVotes(
		processID,
		entityID,
		cspKey.PublicKey(),
		startBlock,
		voterKeys,
		censusOrigin,
		&cspKey,
		proofs,
		false,
		false,
		true,
		&wg,
	)
	wg.Wait()
	if err != nil {
		return err
	}
	fmt.Printf("voting took %s\n", elapsedTime)

	if err := cli.EndProcess(oracle, processID); err != nil {
		return err
	}
	if _, err := cli.TestResults(processID, len(voterKeys), 1); err != nil {
		return err
	}
	return nil
}
