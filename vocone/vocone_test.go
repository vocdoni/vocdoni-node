package vocone

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

// TestVocOne runs a full test of the VocOne API. It creates a new account, and then creates a new election.
func TestVocone(t *testing.T) {
	//log.Init("info", "stdout")
	dir := t.TempDir()

	keymng := ethereum.SignKeys{}
	err := keymng.Generate()
	qt.Assert(t, err, qt.IsNil)

	account := ethereum.SignKeys{}
	err = account.Generate()
	qt.Assert(t, err, qt.IsNil)

	vc, err := NewVocone(dir, &keymng, true)
	qt.Assert(t, err, qt.IsNil)

	err = vc.SetBulkTxCosts(0, true)
	qt.Assert(t, err, qt.IsNil)

	vc.SetBlockTimeTarget(time.Millisecond * 500)
	go vc.Start()
	port := 13000 + util.RandomInt(0, 2000)
	err = vc.EnableAPI("127.0.0.1", port, "/api")
	qt.Assert(t, err, qt.IsNil)

	time.Sleep(time.Second * 2) // TODO: find a more smart way to wait until everything is ready

	u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d/api", port))
	qt.Assert(t, err, qt.IsNil)
	token := uuid.New()
	cli, err := apiclient.NewHTTPclient(u, &token)
	qt.Assert(t, err, qt.IsNil)
	err = cli.SetAccount(fmt.Sprintf("%x", account.PrivateKey()))
	qt.Assert(t, err, qt.IsNil)

	err = testCreateAccount(cli)
	qt.Assert(t, err, qt.IsNil)

	err = vc.CreateAccount(account.Address(), &vochain.Account{Account: models.Account{Balance: 10000}})
	qt.Assert(t, err, qt.IsNil)

	err = testCSPvote(cli)
	qt.Assert(t, err, qt.IsNil)
}

func testCreateAccount(cli *apiclient.HTTPclient) error {
	// Create a new account
	txhash, err := cli.AccountBootstrap(nil)
	if err != nil {
		return err
	}
	if err = cli.EnsureTxIsMined(txhash); err != nil {
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
	duration := 100
	censusSize := 10
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
			BlockCount: uint32(duration),
			StartBlock: 0,
		})
	if err != nil {
		return err
	}
	voterKeys := util.CreateEthRandomKeysBatch(censusSize)
	proofs, err := testutil.GetCSPproofBatch(voterKeys, &cspKey, processID)
	if err != nil {
		return err
	}

	// Wait until the process is ready
	info, err := cli.ChainInfo()
	if err != nil {
		return err
	}
	cli.WaitUntilHeight(*info.Height + 2)

	// Send the votes
	for i, k := range voterKeys {
		c := cli.Clone(fmt.Sprintf("%x", k.PrivateKey()))
		c.Vote(&apiclient.VoteData{
			Choices:    []int{1},
			ElectionID: processID,
			ProofCSP:   proofs[i],
		})
	}

	if _, err = cli.SetElectionStatus(processID, "ENDED"); err != nil {
		return err
	}
	if err := cli.WaitUntilElectionStatus(processID, "RESULTS"); err != nil {
		return err
	}

	election, err := cli.Election(processID)
	if err != nil {
		return err
	}
	fmt.Printf("election: %+v\n", election)
	if !election.Results[0][0].Equal(new(types.BigInt).SetUint64(0)) {
		return err
	}
	if election.Results[0][1].Equal(new(types.BigInt).SetUint64(10)) {
		return err
	}
	return nil
}
