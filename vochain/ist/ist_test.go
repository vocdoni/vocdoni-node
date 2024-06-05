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
	s, err := state.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()
	err = s.SetTimestamp(0)
	qt.Assert(t, err, qt.IsNil)

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
		Status:    models.ProcessStatus_READY,
		StartTime: 0,
		Duration:  2, // 2s
	}
	err = s.AddProcess(p)
	qt.Assert(t, err, qt.IsNil)

	// schedule the election results computation at block 2 (endblock)
	err = istc.Schedule(Action{ID: pid, TypeID: ActionCommitResults, ElectionID: pid, TimeStamp: 2})
	qt.Assert(t, err, qt.IsNil)

	// commit block 0
	testAdvanceBlock(t, s, istc)

	vp, err := state.NewVotePackage([]int{1, 0}).Encode()
	qt.Assert(t, err, qt.IsNil)

	// cast 10 votes
	for j := 0; j < 10; j++ {
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
	testAdvanceBlock(t, s, istc)
	err = s.SetProcessStatus(pid, models.ProcessStatus_ENDED, true)
	qt.Assert(t, err, qt.IsNil)

	// start block 2, on this block results should be scheduled for commit
	testAdvanceBlock(t, s, istc)

	// check results
	p, err = s.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	r := results.ProtoToResults(p.Results)
	qt.Assert(t, r.String(), qt.Equals, "[0,10][10,0]")
}

func TestISTCsyncing(t *testing.T) {
	rng := testutil.NewRandom(0)
	s, err := state.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	err = s.SetTimestamp(0)
	qt.Assert(t, err, qt.IsNil)

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
		Status:    models.ProcessStatus_READY,
		StartTime: 0,
		Duration:  2, // endblock = 2
	}
	err = s.AddProcess(p)
	qt.Assert(t, err, qt.IsNil)

	// schedule the election results computation at block 2 (endblock)
	err = istc.Schedule(Action{TypeID: ActionCommitResults, ElectionID: pid, Height: 2, ID: pid})
	qt.Assert(t, err, qt.IsNil)

	// commit block 0
	testAdvanceBlock(t, s, istc)

	vp, err := state.NewVotePackage([]int{1, 0}).Encode()
	qt.Assert(t, err, qt.IsNil)

	// cast 10 votes
	for j := 0; j < 10; j++ {
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
	testAdvanceBlock(t, s, istc)

	// commit block 2 with synchronization flag to true
	// the commit action should compute the results
	testAdvanceBlock(t, s, istc)

	// check results
	p, err = s.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	r := results.ProtoToResults(p.Results)
	qt.Assert(t, r.String(), qt.Equals, "[0,10][10,0]")
}

func TestISTCMigrateLegacy(t *testing.T) {
	rng := testutil.NewRandom(0)
	s, err := state.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()
	err = s.SetTimestamp(0)
	qt.Assert(t, err, qt.IsNil)

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
		Status:    models.ProcessStatus_READY,
		StartTime: 0,
		Duration:  2, // 2s
	}
	err = s.AddProcess(p)
	qt.Assert(t, err, qt.IsNil)

	// schedule (with legacy format) the election results computation at block 2 (endblock)
	act := legacyAction{ID: ActionCommitResults, ElectionID: pid}
	actions := legacyActions{}
	actions[string(pid)] = act
	err = s.NoState(true).Set(legacyDBIndex(2), actions.encode())
	qt.Assert(t, err, qt.IsNil)

	acts, err := istc.findPendingActions(2, 3)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, acts[0].Attempts, qt.Equals, act.Attempts)
	qt.Assert(t, acts[0].ElectionID, qt.DeepEquals, act.ElectionID)
	qt.Assert(t, acts[0].TypeID, qt.Equals, act.ID)
	qt.Assert(t, acts[0].ValidatorProposer, qt.DeepEquals, act.ValidatorProposer)
	qt.Assert(t, acts[0].ValidatorVotes, qt.DeepEquals, act.ValidatorVotes)

	// commit block 0
	testAdvanceBlock(t, s, istc)

	vp, err := state.NewVotePackage([]int{1, 0}).Encode()
	qt.Assert(t, err, qt.IsNil)

	// cast 10 votes
	for j := 0; j < 10; j++ {
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
	testAdvanceBlock(t, s, istc)
	err = s.SetProcessStatus(pid, models.ProcessStatus_ENDED, true)
	qt.Assert(t, err, qt.IsNil)

	// start block 2, on this block results should be scheduled for commit
	testAdvanceBlock(t, s, istc)

	// check results
	p, err = s.Process(pid, true)
	qt.Assert(t, err, qt.IsNil)
	r := results.ProtoToResults(p.Results)
	qt.Assert(t, r.String(), qt.Equals, "[0,10][10,0]")
}

// testAdvanceBlock advances height and timestamp +1
func testAdvanceBlock(t *testing.T, s *state.State, istc *Controller) {
	height := s.CurrentHeight()
	timestamp, err := s.Timestamp(false)
	qt.Assert(t, err, qt.IsNil)
	err = istc.Commit(height, timestamp)
	qt.Assert(t, err, qt.IsNil)
	_, err = s.PrepareCommit()
	qt.Assert(t, err, qt.IsNil)
	_, err = s.Save()
	qt.Assert(t, err, qt.IsNil)
	s.SetHeight(height + 1)
	qt.Assert(t, s.SetTimestamp(timestamp+1), qt.IsNil)
	log.Infow("committed block", "height", height, "timestamp", timestamp)
}
