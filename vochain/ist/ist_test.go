package ist

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

func TestISTCschedule(t *testing.T) {
	rng := testutil.NewRandom(0)
	s, err := state.NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	// initialize the ISTC
	istc := NewISTC(s)

	// create a new election
	pid := rng.RandomBytes(32)
	censusURI := "ipfs://foobar"
	p := &models.Process{
		EntityId:  rng.RandomBytes(32),
		CensusURI: &censusURI,
		ProcessId: pid,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount: 2,
			MaxValue: 1,
		},
		EnvelopeType: &models.EnvelopeType{},
		Mode: &models.ProcessMode{
			PreRegister:   false,
			AutoStart:     true,
			Interruptible: true,
		},
		Status:     models.ProcessStatus_READY,
		StartBlock: 1,
		BlockCount: 1, // endblock = 2
	}
	err = s.AddProcess(p)
	qt.Assert(t, err, qt.IsNil)

	// schedule the election results computation at block 2 (endblock)
	istc.Schedule(2, pid, Action{ID: ActionComputeResults, ElectionID: pid})

	// commit block 0
	testAdvanceBlock(t, s, istc, false)

	// cast 10 votes
	for j := 0; j < 10; j++ {
		vp, err := state.NewVotePackage([]int{1, 0}).Encode()
		qt.Assert(t, err, qt.IsNil)
		v := &state.Vote{
			ProcessID:   pid,
			Nullifier:   rng.RandomBytes(32),
			VotePackage: vp,
		}
		if err := s.AddVote(v); err != nil {
			t.Error(err)
		}
	} // results should be equal to: [ [0,10], [10,0] ]

	// commit block 1, finalize election
	testAdvanceBlock(t, s, istc, false)
	err = s.SetProcessStatus(pid, models.ProcessStatus_ENDED, true)
	qt.Assert(t, err, qt.IsNil)

	// start block 2, on this block results should be scheduled for computation
	testAdvanceBlock(t, s, istc, false)

	// start block 3
	testAdvanceBlock(t, s, istc, false)

	// start block 4, results should be commited
	testAdvanceBlock(t, s, istc, false)

	// Wait for results
	for {
		p, err := s.Process(pid, true)
		qt.Assert(t, err, qt.IsNil)
		if p.Results.GetVotes() != nil {
			break
		}
	}

	// check results
	p, err = s.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	r := results.ProtoToResults(p.Results)
	qt.Assert(t, r.String(), qt.Equals, "[0,10][10,0]")
}

func TestISTCsyncing(t *testing.T) {
	rng := testutil.NewRandom(0)
	s, err := state.NewState(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	// initialize the ISTC
	istc := NewISTC(s)

	// create a new election
	pid := rng.RandomBytes(32)
	censusURI := "ipfs://foobar"
	p := &models.Process{
		EntityId:  rng.RandomBytes(32),
		CensusURI: &censusURI,
		ProcessId: pid,
		VoteOptions: &models.ProcessVoteOptions{
			MaxCount: 2,
			MaxValue: 1,
		},
		EnvelopeType: &models.EnvelopeType{},
		Mode: &models.ProcessMode{
			PreRegister:   false,
			AutoStart:     true,
			Interruptible: true,
		},
		Status:     models.ProcessStatus_READY,
		StartBlock: 1,
		BlockCount: 1, // endblock = 2
	}
	err = s.AddProcess(p)
	qt.Assert(t, err, qt.IsNil)

	// schedule the election results computation at block 2 (endblock)
	istc.Schedule(2, pid, Action{ID: ActionCommitResults, ElectionID: pid})

	// commit block 0
	testAdvanceBlock(t, s, istc, true)

	// cast 10 votes
	for j := 0; j < 10; j++ {
		vp, err := state.NewVotePackage([]int{1, 0}).Encode()
		qt.Assert(t, err, qt.IsNil)
		v := &state.Vote{
			ProcessID:   pid,
			Nullifier:   rng.RandomBytes(32),
			VotePackage: vp,
		}
		if err := s.AddVote(v); err != nil {
			t.Error(err)
		}
	} // results should be equal to: [ [0,10], [10,0] ]

	// commit block 1
	testAdvanceBlock(t, s, istc, true)

	// commit block 2 with synchronization flag to true
	// the commit action should compute the results
	testAdvanceBlock(t, s, istc, true)

	// check results
	p, err = s.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	r := results.ProtoToResults(p.Results)
	qt.Assert(t, r.String(), qt.Equals, "[0,10][10,0]")
}

func testAdvanceBlock(t *testing.T, s *state.State, istc *Controller, syncFlag bool) {
	height := s.CurrentHeight()
	err := istc.Commit(height, syncFlag)
	qt.Assert(t, err, qt.IsNil)
	_, err = s.Save()
	qt.Assert(t, err, qt.IsNil)
	s.SetHeight(height + 1)
	log.Infof("commited block %d", height)
}
