package vocone

import (
	"context"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/test/testcommon/testvoteproof"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// TestVocOne runs a full test of the VocOne API. It creates a new account, and then creates a new election.
func TestVocone(t *testing.T) {
	dir := t.TempDir()

	keymng := ethereum.SignKeys{}
	err := keymng.Generate()
	qt.Assert(t, err, qt.IsNil)

	account := ethereum.SignKeys{}
	err = account.Generate()
	qt.Assert(t, err, qt.IsNil)

	vc, err := NewVocone(dir, &keymng, false, "", nil)
	qt.Assert(t, err, qt.IsNil)

	vc.App.SetBlockTimeTarget(time.Millisecond * 500)
	go vc.Start()
	port := 13000 + util.RandomInt(0, 2000)
	_, err = vc.EnableAPI("127.0.0.1", port, "/v2")
	qt.Assert(t, err, qt.IsNil)

	time.Sleep(time.Second * 2) // TODO: find a more smart way to wait until everything is ready

	err = vc.SetBulkTxCosts(0, true)
	qt.Assert(t, err, qt.IsNil)

	cli, err := apiclient.New(fmt.Sprintf("http://127.0.0.1:%d/v2", port))
	qt.Assert(t, err, qt.IsNil)
	err = cli.SetAccount(fmt.Sprintf("%x", account.PrivateKey()))
	qt.Assert(t, err, qt.IsNil)

	err = testCreateAccount(cli)
	qt.Assert(t, err, qt.IsNil)

	err = vc.CreateAccount(account.Address(), &state.Account{Account: models.Account{Balance: 10000}})
	qt.Assert(t, err, qt.IsNil)

	err = testCSPvote(cli)
	qt.Assert(t, err, qt.IsNil)
}

func testCreateAccount(cli *apiclient.HTTPclient) error {
	// Create a new account
	txhash, err := cli.AccountBootstrap(nil, nil, nil)
	if err != nil {
		return err
	}
	if _, err = cli.WaitUntilTxIsMined(context.Background(), txhash); err != nil {
		return err
	}
	_, err = cli.Account("")
	return err
}

func testCSPvote(cli *apiclient.HTTPclient) error {
	cspKey := ethereum.SignKeys{}
	if err := cspKey.Generate(); err != nil {
		return err
	}
	entityID := cli.MyAddress().Bytes()
	censusRoot := cspKey.PublicKey()
	censusOrigin := models.CensusOrigin_OFF_CHAIN_CA
	censusSize := uint64(10)
	processID, err := cli.NewElectionRaw(
		&models.Process{
			EntityId:     entityID,
			Status:       models.ProcessStatus_READY,
			CensusRoot:   censusRoot,
			CensusOrigin: censusOrigin,
			EnvelopeType: &models.EnvelopeType{},
			VoteOptions: &models.ProcessVoteOptions{
				MaxCount: 1,
				MaxValue: 1,
			},
			Mode: &models.ProcessMode{
				AutoStart:     true,
				Interruptible: true,
			},
			StartTime:     0,
			Duration:      60,
			MaxCensusSize: censusSize,
		})
	if err != nil {
		return err
	}
	voterKeys := ethereum.NewSignKeysBatch(int(censusSize))
	proofs, err := testvoteproof.GetCSPproofBatch(voterKeys, &cspKey, processID)
	if err != nil {
		return err
	}

	// Wait until the process is ready
	ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel1()
	election, err := cli.WaitUntilElectionStatus(ctx1, processID, "READY")
	if err != nil {
		return err
	}

	// Send the votes
	for i, k := range voterKeys {
		c := cli.Clone(fmt.Sprintf("%x", k.PrivateKey()))
		if _, err := c.Vote(&apiclient.VoteData{
			Choices:  []int{1},
			Election: election,
			ProofCSP: proofs[i],
		}); err != nil {
			return err
		}
	}

	// Wait until all votes are counted
	startTimeVoteCount := time.Now()
	for {
		if time.Since(startTimeVoteCount) > time.Second*10 {
			return fmt.Errorf("timeout waiting for votes to be counted")
		}
		votes, err := cli.ElectionVoteCount(processID)
		if err != nil {
			return err
		}
		if votes == uint32(censusSize) {
			break
		}
		time.Sleep(time.Second)
	}

	if _, err = cli.SetElectionStatus(processID, "ENDED"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	election, err = cli.WaitUntilElectionStatus(ctx, processID, "RESULTS")
	if err != nil {
		return err
	}
	if !election.Results[0][0].Equal(new(types.BigInt).SetUint64(0)) {
		return err
	}
	if election.Results[0][1].Equal(new(types.BigInt).SetUint64(10)) {
		return err
	}
	return nil
}
