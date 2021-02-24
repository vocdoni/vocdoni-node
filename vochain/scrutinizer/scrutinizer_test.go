package scrutinizer

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
)

func TestEntityList(t *testing.T) {
	testEntityList(t, 2)
	testEntityList(t, 100)
	testEntityList(t, 155)
}

func testEntityList(t *testing.T, entityCount int) {
	log.Init("info", "stdout")
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < entityCount; i++ {
		sc.addEntity(util.RandomBytes(20), util.RandomBytes(32))
	}

	entities := make(map[string]bool)
	last := ""
	if sc.entityCount != int64(entityCount) {
		t.Fatalf("entity count is wrong, got %d expected %d", sc.entityCount, entityCount)
	}
	var list []string
	for len(entities) <= entityCount {
		lastBytes := testutil.Hex2byte(t, last)
		list = sc.EntityList(10, lastBytes)
		if len(list) < 1 {
			t.Log("list is empty")
			break
		}
		for _, e := range list {
			if entities[e] {
				t.Fatalf("found duplicated entity: %s", e)
			}
			entities[e] = true
		}
		last = list[len(list)-1]
	}
	if len(entities) < entityCount {
		t.Fatalf("expected %d entityes, got %d", entityCount, len(entities))
	}
}

func TestProcessList(t *testing.T) {
	testProcessList(t, 10)
	testProcessList(t, 20)
	testProcessList(t, 155)
}

func testProcessList(t *testing.T, procsCount int) {

	app, err := vochain.NewBaseApplication(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := app.State.AddOracle(common.HexToAddress(oracle.AddressString())); err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), app.State)
	if err != nil {
		t.Fatal(err)
	}

	// For a entity, add processes (this will be the queried entity)
	eidTest := util.RandomBytes(20)
	for i := 0; i < procsCount; i++ {
		// Add a process with status=READY and interruptible=true
		censusURI := "ipfs://123456789"
		pid := util.RandomBytes(types.ProcessIDsize)
		process := &models.Process{
			ProcessId:    pid,
			StartBlock:   0,
			EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
			Mode:         &models.ProcessMode{Interruptible: true},
			VoteOptions:  &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
			Status:       models.ProcessStatus_READY,
			EntityId:     eidTest,
			CensusRoot:   util.RandomBytes(32),
			CensusURI:    &censusURI,
			CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
			BlockCount:   1024,
			Namespace:    0,
		}
		t.Logf("adding process %x", process.ProcessId)
		if err := app.State.AddProcess(process); err != nil {
			t.Fatal(err)
		}
		sc.addEntity(eidTest, process.ProcessId)
	}

	defaultNamespace := new(uint32)
	*defaultNamespace = 0
	procs := processList(t, sc, eidTest, procsCount, 155, defaultNamespace)
	if len(procs) != procsCount {
		t.Fatalf("expected %d processes, got %d", procsCount, len(procs))
	}

	// try adding a new with different namespace
	censusURI := "ipfs://123456789"
	process := &models.Process{
		ProcessId:    util.RandomBytes(types.ProcessIDsize),
		StartBlock:   0,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Mode:         &models.ProcessMode{Interruptible: true},
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 16, MaxValue: 16},
		Status:       models.ProcessStatus_READY,
		EntityId:     eidTest,
		CensusRoot:   util.RandomBytes(32),
		CensusURI:    &censusURI,
		CensusOrigin: models.CensusOrigin_OFF_CHAIN_TREE,
		BlockCount:   1024,
		Namespace:    1,
	}
	t.Logf("adding process %x", process.ProcessId)
	if err := app.State.AddProcess(process); err != nil {
		t.Fatal(err)
	}
	sc.addEntity(eidTest, process.ProcessId)

	procs2 := processList(t, sc, eidTest, procsCount, 155, defaultNamespace)
	if len(procs2) != procsCount {
		t.Fatalf("expected %d processes, got %d", procsCount, len(procs2))
	}
	// add new process
	process.ProcessId = util.RandomBytes(types.ProcessIDsize)
	process.Namespace = 0
	t.Logf("adding process %x", process.ProcessId)
	if err := app.State.AddProcess(process); err != nil {
		t.Fatal(err)
	}
	sc.addEntity(eidTest, process.ProcessId)

	// try get processList with another max value
	procs3 := processList(t, sc, eidTest, procsCount+1, 155, defaultNamespace)
	// should count the last process added as the namespace is the ones matching with the requested
	if len(procs3) != procsCount+1 {
		t.Fatalf("expected %d processes, got %d", procsCount+1, len(procs3))
	}

	// should count all processes
	procs4 := processList(t, sc, eidTest, procsCount+2, 155, nil)
	// should count the last process added as the namespace is the ones matching with the requested
	if len(procs4) != procsCount+2 {
		t.Fatalf("expected %d processes, got %d", procsCount+2, len(procs4))
	}
}

func processList(t *testing.T, sc *Scrutinizer, eid []byte, procsCount int, max int64, namespace *uint32) map[string]bool {
	procs := make(map[string]bool)
	last := []byte{}
	var list [][]byte
	var err error
	for len(procs) < procsCount {
		list, err = sc.ProcessList(eid, last, max, namespace)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) < 1 {
			t.Log("list is empty")
			break
		}
		for _, p := range list {
			if procs[string(p)] {
				t.Fatalf("found duplicated entity: %x", p)
			}
			procs[string(p)] = true
		}
		last = list[len(list)-1]
	}
	return procs
}

func TestResults(t *testing.T) {
	log.Init("info", "stdout")
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}
	pid := util.RandomBytes(32)
	if err := state.AddProcess(&models.Process{
		ProcessId:             pid,
		EnvelopeType:          &models.EnvelopeType{EncryptedVotes: true},
		Status:                models.ProcessStatus_READY,
		Mode:                  &models.ProcessMode{AutoStart: true},
		EncryptionPrivateKeys: make([]string, 16),
		EncryptionPublicKeys:  make([]string, 16),
		VoteOptions:           &models.ProcessVoteOptions{MaxCount: 4, MaxValue: 1},
	}); err != nil {
		t.Fatal(err)
	}

	priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", ethereum.HashRaw(util.RandomBytes(32))))
	if err != nil {
		t.Fatalf("cannot generate encryption key: (%s)", err)
	}
	ki := uint32(1)
	if err := state.AddProcessKeys(&models.AdminTx{
		Txtype:              models.TxType_ADD_PROCESS_KEYS,
		ProcessId:           pid,
		EncryptionPublicKey: priv.Public().Bytes(),
		KeyIndex:            &ki,
	}); err != nil {
		t.Fatal(err)
	}
	if err := state.RevealProcessKeys(&models.AdminTx{
		Txtype:               models.TxType_ADD_PROCESS_KEYS,
		ProcessId:            pid,
		EncryptionPrivateKey: priv.Bytes(),
		KeyIndex:             &ki,
	}); err != nil {
		t.Fatal(err)
	}

	// Add 100 votes
	vp, err := json.Marshal(types.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomBytes(32)),
		Votes: []int{1, 1, 1, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	vp, err = priv.Encrypt(vp, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 300; i++ {
		if err := state.AddVote(&models.Vote{
			ProcessId:            pid,
			VotePackage:          vp,
			EncryptionKeyIndexes: []uint32{1},
			Nullifier:            util.RandomBytes(32),
			Weight:               new(big.Int).SetUint64(1).Bytes(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := sc.ComputeResult(pid); err != nil {
		t.Fatal(err)
	}

	// Test results
	result, err := sc.VoteResult(pid)
	if err != nil {
		t.Fatal(err)
	}
	log.Infof("results: %s", PrintResults(result))
	v0 := big.NewInt(0)
	v300 := big.NewInt(300)
	value := new(big.Int)
	for _, q := range result.GetVotes() {
		for qi, v1 := range q.Question {
			if qi > 3 {
				t.Fatalf("found more questions that expected")
			}
			value.SetBytes(v1)
			if qi != 1 && value.Cmp(v0) != 0 {
				t.Fatalf("result is not correct, %d is not 0 as expected", value.Uint64())
			}
			if qi == 1 && value.Cmp(v300) != 0 {
				t.Fatalf("result is not correct, %d is not 300 as expected", value.Uint64())
			}
		}
	}
	for _, q := range sc.GetFriendlyResults(result) {
		for qi, v1 := range q {
			if qi > 3 {
				t.Fatalf("found more questions that expected")
			}
			if qi != 1 && v1 != "0" {
				t.Fatalf("result is not correct, %s is not 0 as expected", v1)
			}
			if qi == 1 && v1 != "300" {
				t.Fatalf("result is not correct, %s is not 300 as expected", v1)
			}
		}
	}
}

func TestLiveResults(t *testing.T) {
	log.Init("info", "stdout")
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}
	pid := util.RandomBytes(32)
	if err := state.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 3, MaxValue: 1},
		Mode:         &models.ProcessMode{AutoStart: true},
	}); err != nil {
		t.Fatal(err)
	}
	sc.addLiveResultsProcess(pid)

	// Add 100 votes
	vp, err := json.Marshal(types.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: []int{1, 1, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vote{ProcessId: pid, VotePackage: vp}
	for i := 0; i < 100; i++ {
		if err := sc.addLiveResultsVote(v); err != nil {
			t.Fatal(err)
		}
	}

	if live, err := sc.isLiveResultsProcess(pid); !live || err != nil {
		t.Fatal(fmt.Errorf("isLiveResultsProcess returned false: %v", err))
	}

	// Test results
	result, err := sc.VoteResult(pid)
	if err != nil {
		t.Fatal(err)
	}

	v0 := big.NewInt(0)
	v100 := big.NewInt(100)
	value := new(big.Int)
	for _, q := range result.GetVotes() {
		for qi, v1 := range q.Question {
			if qi > 2 {
				t.Fatalf("found more questions that expected")
			}
			value.SetBytes(v1)
			if qi == 0 && value.Cmp(v0) != 0 {
				t.Fatalf("result is not correct, %d is not 0 as expected", value.Uint64())
			}
			if qi == 1 && value.Cmp(v100) != 0 {
				t.Fatalf("result is not correct, %d is not 100 as expected", value.Uint64())
			}
		}
	}
}

func TestAddVote(t *testing.T) {
	pr := emptyProcess(8, 8)
	options := &models.ProcessVoteOptions{
		MaxCount:     3,
		MaxValue:     3,
		MaxTotalCost: 6,
		CostExponent: 1,
	}
	envelopeType := &models.EnvelopeType{}

	// Should be fine
	err := addVote(pr.Votes, []int{1, 2, 3}, nil, options, envelopeType)
	qt.Assert(t, err, qt.IsNil)

	// Overflows maxTotalCost
	err = addVote(pr.Votes, []int{2, 2, 3}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Overflows maxValue
	err = addVote(pr.Votes, []int{1, 1, 4}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max value overflow.*")

	// Overflows maxCount
	err = addVote(pr.Votes, []int{1, 1, 1, 1}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max count overflow.*")

	// Quadratic voting, 10 credits to distribute among 3 options
	options = &models.ProcessVoteOptions{
		MaxCount:     3,
		MaxValue:     0,
		MaxTotalCost: 10,
		CostExponent: 2,
	}

	// Should be fine 2^2 + 2^2 + 1^2 = 9
	err = addVote(pr.Votes, []int{2, 2, 1}, nil, options, envelopeType)
	qt.Assert(t, err, qt.IsNil)

	// Should be fine 3^2 + 0 + 0 = 9
	err = addVote(pr.Votes, []int{3, 0, 0}, nil, options, envelopeType)
	qt.Assert(t, err, qt.IsNil)

	// Should fail since 2^2 + 2^2 + 2^2 = 12
	err = addVote(pr.Votes, []int{2, 2, 2}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Should fail since 4^2 = 16
	err = addVote(pr.Votes, []int{4, 0, 0}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "max total cost overflow.*")

	// Check unique values work
	envelopeType = &models.EnvelopeType{UniqueValues: true}
	err = addVote(pr.Votes, []int{2, 1, 1}, nil, options, envelopeType)
	qt.Assert(t, err, qt.ErrorMatches, "values are not unique")

}

var vote = func(v []int, sc *Scrutinizer, pid []byte) error {
	vp, err := json.Marshal(types.VotePackage{
		Nonce: fmt.Sprintf("%x", util.RandomHex(32)),
		Votes: v,
	})
	if err != nil {
		return err
	}
	return sc.addLiveResultsVote(
		&models.Vote{
			ProcessId:   pid,
			VotePackage: vp,
		})
}

func TestBallotProtocolRateProduct(t *testing.T) {
	// Rate a product from 0 to 4
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := state.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		Mode:         &models.ProcessMode{AutoStart: true},
		VoteOptions:  &models.ProcessVoteOptions{MaxCount: 2, MaxValue: 4},
	}); err != nil {
		t.Fatal(err)
	}

	sc.addLiveResultsProcess(pid)

	// Rate a product, exepected result: [ [1,0,1,0,2], [0,0,2,0,2] ]
	qt.Assert(t, vote([]int{4, 2}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{4, 2}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{2, 4}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{0, 4}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{0, 5}, sc, pid), qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{0, 0, 0}, sc, pid), qt.ErrorMatches, ".*overflow.*")

	result, err := sc.VoteResult(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := sc.GetFriendlyResults(result)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "0", "1", "0", "2"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "0", "2", "0", "2"})
}

func TestBallotProtocolMultiChoice(t *testing.T) {
	// Choose your 3 favorite colours out of 5

	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}

	// Rate 2 products from 0 to 4
	pid := util.RandomBytes(32)
	if err := state.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
		Status:       models.ProcessStatus_READY,
		Mode:         &models.ProcessMode{AutoStart: true},
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount:     5,
			MaxValue:     1,
			MaxTotalCost: 3,
			CostExponent: 1,
		},
	}); err != nil {
		t.Fatal(err)
	}

	sc.addLiveResultsProcess(pid)

	// Multichoice (choose 3 ouf of 5):
	// - Vote Envelope: `[1,1,1,0,0]` `[0,1,1,1,0]` `[1,1,0,0,0]`
	// - Results: `[ [1, 2], [0, 3], [1, 2], [2, 1], [3, 0] ]`
	qt.Assert(t, vote([]int{1, 1, 1, 0, 0}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{0, 1, 1, 1, 0}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{1, 1, 0, 0, 0}, sc, pid), qt.IsNil)
	qt.Assert(t, vote([]int{2, 1, 0, 0, 0}, sc, pid), qt.ErrorMatches, ".*overflow.*")
	qt.Assert(t, vote([]int{1, 1, 1, 1, 0}, sc, pid), qt.ErrorMatches, ".*overflow.*")

	result, err := sc.VoteResult(pid)
	qt.Assert(t, err, qt.IsNil)
	votes := sc.GetFriendlyResults(result)
	qt.Assert(t, votes[0], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[1], qt.DeepEquals, []string{"0", "3"})
	qt.Assert(t, votes[2], qt.DeepEquals, []string{"1", "2"})
	qt.Assert(t, votes[3], qt.DeepEquals, []string{"2", "1"})
	qt.Assert(t, votes[4], qt.DeepEquals, []string{"3", "0"})

}
