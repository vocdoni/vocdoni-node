package genesis

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestCSPSaltedProofV2Active(t *testing.T) {
	// a chain scheduled at a concrete timestamp, to exercise the boundary
	const scheduled = "vocdoni/TEST/FORK"
	cspSaltedProofV2Time[scheduled] = 1000
	t.Cleanup(func() { delete(cspSaltedProofV2Time, scheduled) })

	for _, tt := range []struct {
		name      string
		chainID   string
		startTime uint32
		want      bool
	}{
		// unknown chains (vocone, custom testsuites, the "test" chainID used by
		// vochain.TestBaseApplication) must default to inactive
		{"unknown chainID", "some/custom/chain", 999999, false},
		{"empty chainID", "", 999999, false},
		{"test chainID used by unit tests", "test", 999999, false},

		// unscheduled chains stay inactive at every start time
		{"forkNever, low startTime", "vocdoni/LTS/1.2", 0, false},
		{"forkNever, high startTime", "vocdoni/LTS/1.2", 4294967294, false},

		// a chain activated from genesis is active for every election
		{"activated from zero", "vocdoni/TEST/1", 0, true},
		{"activated from zero, later election", "vocdoni/TEST/1", 999999, true},

		// the boundary is inclusive: an election starting exactly at the fork
		// timestamp is governed by the new rule
		{"one second before the fork", scheduled, 999, false},
		{"exactly at the fork", scheduled, 1000, true},
		{"one second after the fork", scheduled, 1001, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CSPSaltedProofV2Active(tt.chainID, tt.startTime)
			qt.Assert(t, got, qt.Equals, tt.want)
		})
	}
}

// TestCSPSaltedProofV2NotScheduledOnProductionChains guards against activating a
// production chain by accident. Removing a chain from this list is a deliberate,
// coordinated rollout step, never a drive-by edit.
func TestCSPSaltedProofV2NotScheduledOnProductionChains(t *testing.T) {
	for _, chainID := range []string{"vocdoni/DEV/36", "vocdoni/STAGE/12", "vocdoni/LTS/1.2"} {
		qt.Assert(t, cspSaltedProofV2Time[chainID], qt.Equals, forkNever,
			qt.Commentf("chain %s is scheduled; if that is intentional, update this test "+
				"and follow the runbook in forks.go", chainID))
	}
}
