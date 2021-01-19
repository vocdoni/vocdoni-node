package vochain

import (
	"fmt"
	"testing"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
	models "go.vocdoni.io/proto/build/go/models"
)

func TestState(t *testing.T) {
	log.Init("info", "stdout")
	s, err := NewState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	var pids [][]byte
	for i := 0; i < 100; i++ {
		pids = append(pids, util.RandomBytes(32))
		mkuri := "ipfs://foobar"
		p := &models.Process{EntityId: util.RandomBytes(32), CensusURI: &mkuri, ProcessId: pids[i]}
		s.AddProcess(p)

		for j := 0; j < 10; j++ {
			v := &models.Vote{
				ProcessId:   pids[i],
				Nullifier:   util.RandomBytes(32),
				VotePackage: []byte(fmt.Sprintf("%d%d", i, j)),
			}
			if err := s.AddVote(v); err != nil {
				t.Error(err)
			}
		}
	}
	s.Save()

	p, err := s.Process(pids[10], false)
	if err != nil {
		t.Error(err)
	}
	if len(p.EntityId) != 32 {
		t.Errorf("entityID is not correct")
	}

	_, err = s.Process(util.RandomBytes(32), false)
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
