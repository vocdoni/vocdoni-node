package vochain

import (
	"fmt"
	"testing"

	amino "github.com/tendermint/go-amino"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

func TestState(t *testing.T) {
	log.Init("info", "stdout")
	c := amino.NewCodec()
	s, err := NewState(tempDir(t, "state"), c)
	if err != nil {
		t.Fatal(err)
	}

	var pids [][]byte
	for i := 0; i < 100; i++ {
		pids = append(pids, util.Hex2byte(t, randomHex(32)))
		p := types.Process{EntityID: util.Hex2byte(t, randomHex(32))}
		s.AddProcess(p, pids[i], "ipfs://foobar")

		for j := 0; j < 10; j++ {
			//t.Logf("adding vote %d for process %d", j, i)
			v := types.Vote{
				ProcessID:   pids[i],
				Nullifier:   util.Hex2byte(t, randomHex(32)),
				VotePackage: fmt.Sprintf("%d%d", i, j),
			}
			if err := s.AddVote(&v); err != nil {
				t.Error(err)
			}
		}
	}
	s.Save()

	p, err := s.Process(pids[10], false)
	if err != nil {
		t.Error(err)
	}
	if len(p.EntityID) != 32 {
		t.Errorf("entityID is not correct")
	}

	_, err = s.Process(util.Hex2byte(t, util.RandomHex(32)), false)
	if err == nil {
		t.Errorf("process must not exist")
	}

	votes := s.CountVotes(pids[40], false)
	if votes != 10 {
		t.Errorf("missing votes for process %x (got %d expected %d)", pids[40], votes, 10)
	}
	nullifiers := s.EnvelopeList(pids[50], 0, 20, false)
	if len(nullifiers) != 10 {
		t.Errorf("missing vote nullifiers (got %d expected %d)", len(nullifiers), 10)
	}
	nullifiers = s.EnvelopeList(pids[50], 0, 5, false)
	if len(nullifiers) != 5 {
		t.Errorf("missing vote nullifiers (got %d expected %d)", len(nullifiers), 5)
	}

}
