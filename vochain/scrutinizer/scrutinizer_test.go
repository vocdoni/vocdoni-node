package scrutinizer

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

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
	log.Init("info", "stdout")
	state, err := vochain.NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sc, err := NewScrutinizer(t.TempDir(), state)
	if err != nil {
		t.Fatal(err)
	}

	// Add 10 entities and process for storing random content
	for i := 0; i < 10; i++ {
		sc.addEntity(util.RandomBytes(20), util.RandomBytes(32))
	}

	// For a entity, add 25 processes (this will be the queried entity)
	eidTest := util.RandomBytes(20)
	for i := 0; i < procsCount; i++ {
		sc.addEntity(eidTest, util.RandomBytes(32))
	}

	procs := make(map[string]bool)
	last := []byte{}
	var list [][]byte
	for len(procs) < procsCount {
		list, err = sc.ProcessList(eidTest, last, 10)
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
	if len(procs) != procsCount {
		t.Fatalf("expected %d processes, got %d", procsCount, len(procs))
	}
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
	state.AddProcess(&models.Process{
		ProcessId:             pid,
		EnvelopeType:          &models.EnvelopeType{EncryptedVotes: true},
		Status:                models.ProcessStatus_READY,
		EncryptionPrivateKeys: make([]string, 16),
		EncryptionPublicKeys:  make([]string, 16),
	})

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
	log.Infof("Results: %v", result)
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
			if qi != 1 && v1 != 0 {
				t.Fatalf("result is not correct, %d is not 0 as expected", v1)
			}
			if qi == 1 && v1 != 300 {
				t.Fatalf("result is not correct, %d is not 300 as expected", v1)
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
	state.AddProcess(&models.Process{
		ProcessId:    pid,
		EnvelopeType: &models.EnvelopeType{EncryptedVotes: false},
	})
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
		t.Fatal(fmt.Errorf("isLiveResultsProcess returned false while true is expected or some other error: %v", err))
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
			if qi == 1 && value.Cmp(v100) != 100 {
				t.Fatalf("result is not correct, %d is not 100 as expected", value.Uint64())
			}
		}
	}
}
