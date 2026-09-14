package vochain

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/proto/build/go/models"
)

// makeVal builds a Validator with a deterministic 33-byte pubkey seeded from
// `seed`, matching the shape CometBFT expects (compressed-secp256k1 first byte
// 0x02). All three copies of the pubkey/address bytes are distinct so
// duplicate-detection in validatorUpdate cannot false-negative by aliasing.
func makeVal(seed byte, power uint64) *models.Validator {
	pubkey := make([]byte, 33)
	pubkey[0] = 0x02
	for i := 1; i < len(pubkey); i++ {
		pubkey[i] = seed ^ byte(i)
	}
	return &models.Validator{PubKey: pubkey, Power: power}
}

// pubkeyOf mirrors makeVal so tests can build the "removed pubkey" argument
// separately from the surviving-validators map.
func pubkeyOf(seed byte) []byte {
	return makeVal(seed, 0).PubKey
}

// TestValidatorUpdate is the regression test for the CometBFT zombie-validator
// bug: an evicted validator only leaves the CometBFT ValidatorSet when its
// pubkey appears in ValidatorUpdates with Power=0. This test pins the four
// invariants validatorUpdate must guarantee.
func TestValidatorUpdate(t *testing.T) {
	cases := []struct {
		name       string
		validators map[string]*models.Validator
		removed    [][]byte
		// expectPower is keyed by pubkey (as a string, since []byte is not a
		// valid map key) and gives the expected Power value in the emitted
		// ValidatorUpdates. Assertion is that the emitted set matches this
		// map exactly (same size, same entries).
		expectPower map[string]int64
	}{
		{
			name: "surviving-only: each validator gets its own power, no removals emitted",
			validators: map[string]*models.Validator{
				"A": makeVal('A', 100),
				"B": makeVal('B', 500),
				"C": makeVal('C', 1000),
			},
			removed: nil,
			expectPower: map[string]int64{
				string(pubkeyOf('A')): 100,
				string(pubkeyOf('B')): 500,
				string(pubkeyOf('C')): 1000,
			},
		},
		{
			name: "removed-only: every entry is Power=0",
			validators: map[string]*models.Validator{
				"A": makeVal('A', 100),
			},
			removed: [][]byte{pubkeyOf('D'), pubkeyOf('E')},
			expectPower: map[string]int64{
				string(pubkeyOf('A')): 100,
				string(pubkeyOf('D')): 0,
				string(pubkeyOf('E')): 0,
			},
		},
		{
			name: "mixed: surviving keep their power, removed appear once each at Power=0",
			validators: map[string]*models.Validator{
				"A": makeVal('A', 100),
				"B": makeVal('B', 500),
				"C": makeVal('C', 1000),
			},
			removed: [][]byte{pubkeyOf('D'), pubkeyOf('E')},
			expectPower: map[string]int64{
				string(pubkeyOf('A')): 100,
				string(pubkeyOf('B')): 500,
				string(pubkeyOf('C')): 1000,
				string(pubkeyOf('D')): 0,
				string(pubkeyOf('E')): 0,
			},
		},
		{
			name:        "empty inputs produce empty updates",
			validators:  map[string]*models.Validator{},
			removed:     nil,
			expectPower: map[string]int64{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updates := validatorUpdate(tc.validators, tc.removed)

			qt.Assert(t, len(updates), qt.Equals, len(tc.expectPower),
				qt.Commentf("expected %d updates, got %d", len(tc.expectPower), len(updates)))

			seen := map[string]bool{}
			for _, u := range updates {
				pk := string(u.PubKeyBytes)
				want, ok := tc.expectPower[pk]
				qt.Assert(t, ok, qt.IsTrue,
					qt.Commentf("unexpected pubkey %x in updates", u.PubKeyBytes))
				qt.Assert(t, u.Power, qt.Equals, want,
					qt.Commentf("pubkey %x: expected Power=%d, got Power=%d", u.PubKeyBytes, want, u.Power))
				qt.Assert(t, seen[pk], qt.IsFalse,
					qt.Commentf("pubkey %x appears twice in updates (invariant: no duplicates across surviving+removed groups)", u.PubKeyBytes))
				seen[pk] = true
			}
		})
	}
}

// TestValidatorUpdate_RemovedPubKeysNilIsIdentical pins the pre-fix baseline
// (removedPubKeys == nil emits no Power=0 entries), so accidentally injecting
// a spurious Power=0 update is caught.
func TestValidatorUpdate_RemovedPubKeysNilIsIdentical(t *testing.T) {
	validators := map[string]*models.Validator{
		"A": makeVal('A', 100),
		"B": makeVal('B', 500),
	}
	updates := validatorUpdate(validators, nil)
	qt.Assert(t, len(updates), qt.Equals, len(validators),
		qt.Commentf("with removedPubKeys=nil, output length must equal the number of surviving validators"))
	for _, u := range updates {
		qt.Assert(t, u.Power > 0, qt.IsTrue,
			qt.Commentf("no Power=0 entry should be emitted when removedPubKeys is nil; got %x @ Power=%d", u.PubKeyBytes, u.Power))
	}
}
