package state

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
)

func TestValidatorInactiveSinceRoundtrip(t *testing.T) {
	s, err := New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	defer s.Close()

	addr := []byte("\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13")

	// Unset returns not-marked without error.
	_, marked, err := s.ValidatorInactiveSince(addr, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, marked, qt.IsFalse)

	// Set, then read back.
	qt.Assert(t, s.SetValidatorInactiveSince(addr, 12345), qt.IsNil)
	got, marked, err := s.ValidatorInactiveSince(addr, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, marked, qt.IsTrue)
	qt.Assert(t, got, qt.Equals, uint32(12345))

	// Committed view reflects the value once the state is committed.
	testSaveState(t, s)
	got, marked, err = s.ValidatorInactiveSince(addr, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, marked, qt.IsTrue)
	qt.Assert(t, got, qt.Equals, uint32(12345))

	// Different address is independent.
	other := []byte("\x20\x21\x22\x23\x24\x25\x26\x27\x28\x29\x2a\x2b\x2c\x2d\x2e\x2f\x30\x31\x32\x33")
	_, marked, err = s.ValidatorInactiveSince(other, true)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, marked, qt.IsFalse)

	// Overwrite with a fresh height.
	qt.Assert(t, s.SetValidatorInactiveSince(addr, 99999), qt.IsNil)
	got, marked, err = s.ValidatorInactiveSince(addr, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, marked, qt.IsTrue)
	qt.Assert(t, got, qt.Equals, uint32(99999))

	// Clear returns to not-marked.
	qt.Assert(t, s.ClearValidatorInactiveSince(addr), qt.IsNil)
	_, marked, err = s.ValidatorInactiveSince(addr, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, marked, qt.IsFalse)

	// Clearing again is a no-op.
	qt.Assert(t, s.ClearValidatorInactiveSince(addr), qt.IsNil)
	_, marked, err = s.ValidatorInactiveSince(addr, false)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, marked, qt.IsFalse)
}
