package ist

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// makeValidator returns a validator whose signing address and cometbft
// validator address are both filled with the given seed byte, so tests can
// address them by a single-letter mnemonic.
func makeValidator(seed byte, power uint64, joinHeight uint64) *models.Validator {
	addr := make([]byte, 20)
	vaddr := make([]byte, 20)
	for i := range addr {
		addr[i] = seed
		vaddr[i] = seed ^ 0x80 // distinct from signing address
	}
	return &models.Validator{
		Address:          addr,
		ValidatorAddress: vaddr,
		Power:            power,
		Height:           joinHeight,
	}
}

// istHarness advances the state one period at a time, feeding
// updateValidatorScore with the given voter/proposer validator addresses.
type istHarness struct {
	t      *testing.T
	s      *state.State
	istc   *Controller
	height uint32
}

func newISTHarness(t *testing.T) *istHarness {
	t.Helper()
	s, err := state.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	t.Cleanup(func() { s.Close() })
	qt.Assert(t, s.SetTimestamp(0), qt.IsNil)
	return &istHarness{t: t, s: s, istc: NewISTC(s)}
}

func (h *istHarness) commit() {
	h.t.Helper()
	_, err := h.s.PrepareCommit()
	qt.Assert(h.t, err, qt.IsNil)
	_, err = h.s.Save()
	qt.Assert(h.t, err, qt.IsNil)
}

// advancePeriod advances the state by updatePowerPeriod blocks, matching
// production where updateValidatorScore is called (and its state mutations
// committed) once per block. The attending set votes on every block; the
// score branch fires only on the period-boundary block.
func (h *istHarness) advancePeriod(attending [][]byte, proposer []byte) {
	h.t.Helper()
	for range updatePowerPeriod {
		h.height++
		h.s.SetHeight(h.height)
		qt.Assert(h.t, h.istc.updateValidatorScore(attending, proposer), qt.IsNil)
		h.commit()
	}
}

func (h *istHarness) power(seed byte) uint64 {
	h.t.Helper()
	list, err := h.s.Validators(true)
	qt.Assert(h.t, err, qt.IsNil)
	for _, v := range list {
		if v.Address[0] == seed {
			return v.Power
		}
	}
	return 0 // removed or absent
}

func (h *istHarness) exists(seed byte) bool {
	h.t.Helper()
	list, err := h.s.Validators(true)
	qt.Assert(h.t, err, qt.IsNil)
	for _, v := range list {
		if v.Address[0] == seed {
			return true
		}
	}
	return false
}

func (h *istHarness) marker(seed byte) (uint32, bool) {
	h.t.Helper()
	addr := make([]byte, 20)
	for i := range addr {
		addr[i] = seed
	}
	since, marked, err := h.s.ValidatorInactiveSince(addr, true)
	qt.Assert(h.t, err, qt.IsNil)
	return since, marked
}

// TestPowerDecaysToFloor verifies that a non-voting validator's power
// decays exponentially and floors at minPower rather than crossing to zero.
func TestPowerDecaysToFloor(t *testing.T) {
	h := newISTHarness(t)

	// Four validators are enough to make removal legal (len > 3).
	// A, B, C vote; D is silent from the start.
	seeds := []byte{'A', 'B', 'C', 'D'}
	initialPower := uint64(1000)
	for _, seed := range seeds {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, initialPower, 0)), qt.IsNil)
	}
	h.commit()

	votingAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C'} {
		votingAddrs = append(votingAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}
	proposer := votingAddrs[0]

	// Enough periods for decay to reach the floor: 1000*0.9^n < 1 at n≈66.
	for range 90 {
		h.advancePeriod(votingAddrs, proposer)
	}

	qt.Assert(t, h.power('D'), qt.Equals, uint64(minPower))
	// The three active validators keep their power at the cap.
	for _, seed := range []byte{'A', 'B', 'C'} {
		qt.Assert(t, h.power(seed), qt.Equals, uint64(maxPower),
			qt.Commentf("validator %c should be pinned at maxPower", seed))
	}
}

// TestInactiveSinceMarkerLifecycle verifies the marker is set on first floor
// entry and cleared once the validator recovers above the floor.
func TestInactiveSinceMarkerLifecycle(t *testing.T) {
	h := newISTHarness(t)

	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, 100, 0)), qt.IsNil)
	}
	h.commit()

	activeVAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C'} {
		activeVAddrs = append(activeVAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}

	// D silent long enough to hit the floor.
	for range 60 {
		h.advancePeriod(activeVAddrs, activeVAddrs[0])
	}
	qt.Assert(t, h.power('D'), qt.Equals, uint64(minPower))
	sinceEntry, marked := h.marker('D')
	qt.Assert(t, marked, qt.IsTrue)
	qt.Assert(t, sinceEntry > 0 && sinceEntry <= h.height, qt.IsTrue,
		qt.Commentf("marker %d should sit inside the harness period range (0, %d]", sinceEntry, h.height))

	// Bring D back: now include D in the attending set, at a high enough score
	// to trigger ramp-up. positiveScoreThreshold is measured relative to
	// (Votes / (currentHeight - joinHeight) * 100); one period of exclusive D
	// participation lifts the ratio enough to trigger power++.
	// Simulate D voting alone: bump D.Votes directly by attending several
	// periods with only D in the set, then observe the marker clears.
	dVAddr := makeValidator('D', 0, 0).ValidatorAddress
	fullSet := append([][]byte{dVAddr}, activeVAddrs...)
	for range 20 {
		h.advancePeriod(fullSet, activeVAddrs[0])
	}

	qt.Assert(t, h.power('D') > minPower, qt.IsTrue,
		qt.Commentf("D should have recovered above the floor after voting again; got power=%d", h.power('D')))
	_, marked = h.marker('D')
	qt.Assert(t, marked, qt.IsFalse,
		qt.Commentf("marker should be cleared once D climbed back above the floor"))
}

// TestExpulsionAfterGracePeriod verifies a floor-pinned validator is removed
// from the set only once currentHeight - marker >= inactiveGraceBlocks, and
// only when doing so leaves at least three validators.
func TestExpulsionAfterGracePeriod(t *testing.T) {
	// Shorten the grace period so the test runs in seconds.
	original := inactiveGraceBlocks
	inactiveGraceBlocks = 100
	t.Cleanup(func() { inactiveGraceBlocks = original })

	h := newISTHarness(t)

	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, 100, 0)), qt.IsNil)
	}
	h.commit()

	activeVAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C'} {
		activeVAddrs = append(activeVAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}

	// Drive D down to the floor.
	for h.power('D') > minPower {
		h.advancePeriod(activeVAddrs, activeVAddrs[0])
		qt.Assert(t, h.height < 1000, qt.IsTrue, qt.Commentf("safety cap: D should have floored well before height 1000"))
	}
	sinceEntry, marked := h.marker('D')
	qt.Assert(t, marked, qt.IsTrue)
	qt.Assert(t, h.exists('D'), qt.IsTrue, qt.Commentf("D should still be in the set immediately after flooring"))

	// Advance up to just before the grace period elapses: D remains in the set.
	for h.height-sinceEntry < inactiveGraceBlocks {
		h.advancePeriod(activeVAddrs, activeVAddrs[0])
	}
	// The last advance made height - since == inactiveGraceBlocks (unless we
	// overshot by one period, in which case D was already removed on that
	// same period). Verify D is gone on this or the next period.
	if h.exists('D') {
		h.advancePeriod(activeVAddrs, activeVAddrs[0])
	}
	qt.Assert(t, h.exists('D'), qt.IsFalse,
		qt.Commentf("D should have been removed once the grace period elapsed"))
	_, marked = h.marker('D')
	qt.Assert(t, marked, qt.IsFalse,
		qt.Commentf("marker should be cleared alongside removal"))
}

// TestLongLivedSilentValidatorDecaysFast is a regression test for a bug in
// main: a validator that voted reliably for a long time and then went silent
// kept its power near the cap for weeks because the score formula was a
// lifetime average `Votes / (currentHeight - joinHeight) * 100`. Once
// `Votes ≈ currentHeight - joinHeight`, the ratio moved by one integer step
// per tens of thousands of blocks, and the "score stable at >= threshold"
// ramp-up condition undid every decay step in between. Lucas' validator was
// observed at ~80 power after weeks disconnected.
//
// The fix resets `Votes` and `Height` at every score computation, so the
// score is always the participation rate over the just-closed
// `updatePowerPeriod`-block window. A validator that stops voting sees its
// score go to 0 the very next period, and its power decays every period
// after that, regardless of how long it had been running.
func TestLongLivedSilentValidatorDecaysFast(t *testing.T) {
	// Grace-period expulsion is not what this test is about; push it out of
	// the way so we can observe the power trajectory alone.
	original := inactiveGraceBlocks
	inactiveGraceBlocks = 1 << 30
	t.Cleanup(func() { inactiveGraceBlocks = original })

	h := newISTHarness(t)

	// Simulate the "long-lived" precondition without actually running a
	// million blocks: seed each validator as if it had voted every block from
	// height 0 to `startHeight` (Votes == startHeight, Score == 100). This is
	// the state a healthy validator settles into over time.
	const startHeight = uint32(1_000_000)
	h.height = startHeight
	h.s.SetHeight(startHeight)
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		v := makeValidator(seed, maxPower, 0)
		v.Votes = uint64(startHeight)
		v.Score = 100
		qt.Assert(t, h.s.AddValidator(v), qt.IsNil)
	}
	h.commit()

	activeVAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C'} {
		activeVAddrs = append(activeVAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}

	// D is silent from here on; A, B, C keep voting. With decayRate = 10%,
	// power reaches the floor in ~66 periods; the assertion window is 100
	// periods = 1000 blocks ≈ 3.3 minutes at 12s/block, well below "weeks".
	const decayBudget = 100
	for range decayBudget {
		h.advancePeriod(activeVAddrs, activeVAddrs[0])
	}
	qt.Assert(t, h.power('D'), qt.Equals, uint64(minPower),
		qt.Commentf("D should have reached the floor within %d periods (~%d blocks) after going silent",
			decayBudget, decayBudget*updatePowerPeriod))

	// A, B, C stayed pinned at the cap over the same window.
	for _, seed := range []byte{'A', 'B', 'C'} {
		qt.Assert(t, h.power(seed), qt.Equals, uint64(maxPower),
			qt.Commentf("validator %c should have stayed at maxPower", seed))
	}
}

// TestBottomOfBarrelKeepsThree verifies that when only three validators
// remain, none of them is expelled even past the grace period.
func TestBottomOfBarrelKeepsThree(t *testing.T) {
	original := inactiveGraceBlocks
	inactiveGraceBlocks = 30
	t.Cleanup(func() { inactiveGraceBlocks = original })

	h := newISTHarness(t)

	// Only three validators, all silent forever: none can be removed.
	for _, seed := range []byte{'A', 'B', 'C'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, 10, 0)), qt.IsNil)
	}
	h.commit()

	// Empty vote set: all three decay together.
	for range 200 {
		h.advancePeriod(nil, nil)
	}

	list, err := h.s.Validators(true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(list), qt.Equals, 3,
		qt.Commentf("no removals allowed while only three validators remain"))
	for _, v := range list {
		qt.Assert(t, v.Power, qt.Equals, uint64(minPower))
	}
}
