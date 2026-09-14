package ist

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// makeValidator returns a validator whose signing address, cometbft validator
// address, and pubkey are all derived from the given seed byte, so tests can
// address them by a single-letter mnemonic. The pubkey is a 33-byte value
// that matches what CometBFT expects (compressed-secp256k1 shape).
func makeValidator(seed byte, power uint64, joinHeight uint64) *models.Validator {
	addr := make([]byte, 20)
	vaddr := make([]byte, 20)
	pubkey := make([]byte, 33)
	for i := range addr {
		addr[i] = seed
		vaddr[i] = seed ^ 0x80 // distinct from signing address
	}
	// First byte 0x02 is the compressed-point prefix; remainder from seed.
	pubkey[0] = 0x02
	for i := 1; i < len(pubkey); i++ {
		pubkey[i] = seed ^ byte(i)
	}
	return &models.Validator{
		Address:          addr,
		ValidatorAddress: vaddr,
		PubKey:           pubkey,
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

// TestScoringPreservesJoinHeightAndLifetimeVotes verifies that scoring never
// overwrites `models.Validator.Height` (join height) or `models.Validator.Votes`
// (lifetime signature count). Both are surfaced by GET /chain/validators as
// `joinHeight` and `votes`; a prior revision of updateValidatorScore reset them
// to the window boundary and to zero every period, which would have made the
// two API fields useless for any downstream computing tenure/age or lifetime
// participation. The window boundary now lives in TreeExtra under `vldSW/`.
func TestScoringPreservesJoinHeightAndLifetimeVotes(t *testing.T) {
	h := newISTHarness(t)

	// One steadily-voting validator joining at height 0.
	seeds := []byte{'A', 'B', 'C', 'D'}
	const joinHeight = uint64(0)
	for _, seed := range seeds {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, 500, joinHeight)), qt.IsNil)
	}
	h.commit()

	votingAddrs := [][]byte{}
	for _, seed := range seeds {
		votingAddrs = append(votingAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}
	proposer := votingAddrs[0]

	// Run enough periods to close several score windows.
	const periods = 5
	for range periods {
		h.advancePeriod(votingAddrs, proposer)
	}

	list, err := h.s.Validators(true)
	qt.Assert(t, err, qt.IsNil)
	for _, v := range list {
		qt.Assert(t, v.Height, qt.Equals, joinHeight,
			qt.Commentf("Validator.Height (joinHeight in public API) must stay at the join height, not be reset per score window; got %d", v.Height))
		qt.Assert(t, v.Votes, qt.Equals, uint64(periods*updatePowerPeriod),
			qt.Commentf("Validator.Votes must be the lifetime signature count, not reset per window; got %d after %d blocks of voting", v.Votes, periods*updatePowerPeriod))
	}
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

// TestSingleSilentPeriodDropsPowerByDecayRate pins `powerDecayRate = 0.10`:
// one silent period must move an active validator from `maxPower` (1000) to
// exactly 900. If `powerDecayRate` regresses to its pre-recalibration 0.05,
// the drop would be to 950 and this assertion catches it.
func TestSingleSilentPeriodDropsPowerByDecayRate(t *testing.T) {
	h := newISTHarness(t)

	// Four validators so removal is legal (len > 3), even though we don't
	// exercise removal here. A, B, C vote; D is silent.
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, 1000, 0)), qt.IsNil)
	}
	h.commit()

	votingAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C'} {
		votingAddrs = append(votingAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}
	h.advancePeriod(votingAddrs, votingAddrs[0])

	// D silent for one period: 1000 * (1 - 0.10) = 900. A literal, not
	// `maxPower * (1 - powerDecayRate)`, so that reverting `powerDecayRate`
	// to a different value fails this test.
	qt.Assert(t, h.power('D'), qt.Equals, uint64(900),
		qt.Commentf("one silent period must drop power from 1000 to exactly 900 (10%% decay)"))
}

// TestSinglePositivePeriodIncrementsByTen pins `powerIncrement = 10`: one
// positive-score period from 990 must land on exactly 1000. If
// `powerIncrement` regresses to its pre-recalibration 1, the ramp would land
// at 991 and this assertion catches it. Also pins `maxPower = 1000` via the
// `min(..., maxPower)` clamp: with `maxPower = 100`, the clamp would drop
// the result to 100.
func TestSinglePositivePeriodIncrementsByTen(t *testing.T) {
	h := newISTHarness(t)

	// A single validator voting every block earns score = 100 in the first
	// period; that clears both branches of the ramp-up guard (`newScore >
	// v.Score` on the first period since Score starts at 0). Three others so
	// the harness has a legal set and set-size logic never trips.
	qt.Assert(t, h.s.AddValidator(makeValidator('A', 990, 0)), qt.IsNil)
	for _, seed := range []byte{'B', 'C', 'D'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, 1000, 0)), qt.IsNil)
	}
	h.commit()

	votingAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		votingAddrs = append(votingAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}
	h.advancePeriod(votingAddrs, votingAddrs[0])

	qt.Assert(t, h.power('A'), qt.Equals, uint64(1000),
		qt.Commentf("one positive period from 990 must reach exactly 1000 (increment=10, cap=1000)"))
}

// TestFloorReachedInExpectedPeriodCount pins the whole decay curve — the
// combination of `powerDecayRate`, `minPower`, and `maxPower` — by asserting
// that a validator starting at `maxPower` (1000) reaches `minPower` (1) in
// exactly `expectedPeriods` periods and no earlier. Any regression on those
// three constants shifts the number.
func TestFloorReachedInExpectedPeriodCount(t *testing.T) {
	h := newISTHarness(t)

	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, 1000, 0)), qt.IsNil)
	}
	h.commit()

	votingAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C'} {
		votingAddrs = append(votingAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}

	// With powerDecayRate=0.10, maxPower=1000, minPower=1, the uint64
	// truncation at each step of `uint64(power * 0.9)` compounds and reaches
	// 1 at period 49 (never earlier: assert the "not on period 48" boundary).
	const expectedPeriods = 49
	for i := range expectedPeriods - 1 {
		h.advancePeriod(votingAddrs, votingAddrs[0])
		qt.Assert(t, h.power('D') > minPower, qt.IsTrue,
			qt.Commentf("D must still be above the floor at period %d (want floor at exactly period %d)", i+1, expectedPeriods))
	}
	h.advancePeriod(votingAddrs, votingAddrs[0])
	qt.Assert(t, h.power('D'), qt.Equals, uint64(minPower),
		qt.Commentf("D must reach the floor at exactly period %d", expectedPeriods))
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

// TestNoEvictionWhenInactiveSinceAheadOfHeight guards against uint32
// underflow in the grace-period check: after a cometbft rollback or a chain
// restart whose InitialHeight is below a previously recorded inactive-since
// marker, `height - since` wraps to ~4e9 and every marked validator would be
// evicted with no grace at all. The `height >= since` guard in
// updateValidatorScore is what prevents that.
func TestNoEvictionWhenInactiveSinceAheadOfHeight(t *testing.T) {
	original := inactiveGraceBlocks
	inactiveGraceBlocks = 50
	t.Cleanup(func() { inactiveGraceBlocks = original })

	h := newISTHarness(t)

	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, minPower, 0)), qt.IsNil)
	}
	// Mark all four validators as inactive at a high height, then commit.
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		addr := make([]byte, 20)
		for i := range addr {
			addr[i] = seed
		}
		qt.Assert(t, h.s.SetValidatorInactiveSince(addr, 10_000), qt.IsNil)
	}
	h.commit()

	// Simulate a rollback: chain height is now well below the marker.
	// Pick a value that's a multiple of updatePowerPeriod so the score branch fires.
	h.height = 100
	h.s.SetHeight(h.height)

	qt.Assert(t, h.istc.updateValidatorScore(nil, nil), qt.IsNil)
	h.commit()

	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		qt.Assert(t, h.exists(seed), qt.IsTrue,
			qt.Commentf("validator %c must NOT be evicted when its inactive-since marker is ahead of current height (rollback scenario)", seed))
	}
	qt.Assert(t, len(h.istc.DrainRemovedPubKeys()), qt.Equals, 0,
		qt.Commentf("removedPubKeys must be empty when no eviction happened"))
}

// TestScoreGapDoesNotWrapWhenWindowStartAheadOfHeight guards the sibling
// underflow inside score computation: `gap := height - windowStart` on a
// rollback would wrap to ~4e9, driving `newScore` to zero on every validator
// and forcing an unwarranted decay every period.
func TestScoreGapDoesNotWrapWhenWindowStartAheadOfHeight(t *testing.T) {
	h := newISTHarness(t)

	// Four validators at mid-range power, no inactive-since marker set — the
	// power path should be exercised without touching eviction.
	const startPower = uint64(500)
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, startPower, 0)), qt.IsNil)
	}
	// Seed a score-window entry at a height much higher than what we'll
	// simulate below — mimics the post-rollback state where vldSW/ still
	// holds a boundary from the pre-rollback tip.
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		addr := make([]byte, 20)
		for i := range addr {
			addr[i] = seed
		}
		qt.Assert(t, h.s.SetValidatorScoreWindow(addr, 10_000, 0), qt.IsNil)
	}
	h.commit()

	// Rollback: chain height jumps back to a value below the recorded window.
	// Keep it on a period boundary so the score branch runs.
	h.height = 100
	h.s.SetHeight(h.height)

	votingAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		votingAddrs = append(votingAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}
	// One period boundary run; every validator votes so windowVotes > 0.
	qt.Assert(t, h.istc.updateValidatorScore(votingAddrs, votingAddrs[0]), qt.IsNil)
	h.commit()

	// Without the guard, `gap` would be ~4e9, newScore would be 0, and the
	// decay branch (`newScore == 0`) would fire on every validator, dropping
	// power from 500 to 450. With the guard, gap falls back to
	// updatePowerPeriod (10), newScore = 1/10 * 100 = 10, which is above the
	// starting v.Score of 0 and hits the increment branch (power → 510).
	// Assert power did not decay — i.e. no wraparound-triggered erroneous
	// decay slipped through.
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		p := h.power(seed)
		qt.Assert(t, p >= startPower, qt.IsTrue,
			qt.Commentf("validator %c power must not have decayed after a windowStart-ahead-of-height run; want >= %d, got %d", seed, startPower, p))
	}
}

// TestEvictedValidatorEmitsPowerZero is the regression test for the CometBFT
// zombie-validator bug fixed in this branch. Per the ABCI spec, a validator is
// only removed from CometBFT's internal ValidatorSet when it appears in
// ValidatorUpdates with Power=0. A validator that is simply absent from the
// map is silently kept at its last-known power ("not mentioned" ≠ "removed").
//
// Before this fix, RemoveValidator dropped the validator from the state map
// before validatorUpdate ran, so no Power=0 entry was ever emitted for the
// removed validator. With PR#1454's minPower=1, the validator's last-known
// power was 1, not 0, so CometBFT kept it in its ValidatorSet indefinitely.
//
// The fix: updateValidatorScore appends the evicted validator's pubkey to
// Controller.removedPubKeys; FinalizeBlock drains it via DrainRemovedPubKeys
// and passes the list to validatorUpdate, which emits a Power=0 entry for each.
//
// This test exercises the full production path:
//
//	updateValidatorScore → RemoveValidator → Controller.removedPubKeys
//	→ DrainRemovedPubKeys → (consumed by validatorUpdate in FinalizeBlock)
//
// We verify both invariants that must hold:
//
//	(a) the validator is removed from state (not reachable via Validators),
//	(b) DrainRemovedPubKeys returns the evicted validator's pubkey, which
//	    validatorUpdate will use to emit the required Power=0 entry.
func TestEvictedValidatorEmitsPowerZero(t *testing.T) {
	// Shorten the grace period so the test runs in milliseconds.
	original := inactiveGraceBlocks
	inactiveGraceBlocks = 50
	t.Cleanup(func() { inactiveGraceBlocks = original })

	h := newISTHarness(t)

	// Four validators: A, B, C vote; D is silent from the start.
	// Four is the minimum for removal to be legal (len > 3 check).
	for _, seed := range []byte{'A', 'B', 'C', 'D'} {
		qt.Assert(t, h.s.AddValidator(makeValidator(seed, 100, 0)), qt.IsNil)
	}
	h.commit()

	activeVAddrs := [][]byte{}
	for _, seed := range []byte{'A', 'B', 'C'} {
		activeVAddrs = append(activeVAddrs, makeValidator(seed, 0, 0).ValidatorAddress)
	}

	// Advance period by period until D is evicted. The loop stops on the same
	// period that removes D, so removedPubKeys is still populated from that
	// updateValidatorScore run and has not yet been cleared by a subsequent
	// call. We impose a generous safety cap to avoid infinite loops.
	const safetyCapPeriods = 10000
	for i := range safetyCapPeriods {
		if !h.exists('D') {
			break
		}
		h.advancePeriod(activeVAddrs, activeVAddrs[0])
		if i == safetyCapPeriods-1 {
			t.Fatalf("safety cap reached: D was not evicted after %d periods", safetyCapPeriods)
		}
	}

	// (a) D must be gone from state.
	qt.Assert(t, h.exists('D'), qt.IsFalse,
		qt.Commentf("D should have been removed from state after the grace period"))

	// (b) DrainRemovedPubKeys must contain D's pubkey. This is the exact data
	// that FinalizeBlock passes to validatorUpdate to emit the Power=0 entry
	// that tells CometBFT to remove D from its ValidatorSet.
	//
	// On the pre-fix code, removedPubKeys was never populated, so this would
	// return nil (len 0) and the assertion below would fail — confirming the
	// bug.  With the fix, updateValidatorScore appends the pubkey on eviction.
	dPubKey := makeValidator('D', 0, 0).PubKey
	removed := h.istc.DrainRemovedPubKeys()
	qt.Assert(t, len(removed), qt.Equals, 1,
		qt.Commentf("exactly one validator was removed; DrainRemovedPubKeys should return one entry"))
	qt.Assert(t, removed[0], qt.DeepEquals, dPubKey,
		qt.Commentf("the removed pubkey must be D's so validatorUpdate emits Power=0 for D"))

	// Draining is destructive: a second call returns nothing.
	qt.Assert(t, h.istc.DrainRemovedPubKeys(), qt.IsNil,
		qt.Commentf("DrainRemovedPubKeys must clear the buffer on first call"))
}
