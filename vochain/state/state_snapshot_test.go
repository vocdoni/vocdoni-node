package state

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/proto/build/go/models"
)

func TestSnapshot(t *testing.T) {
	st := newStateForTest(t)

	err := st.AddValidator(&models.Validator{
		Address: []byte("validator1"),
		Power:   10,
	})
	qt.Assert(t, err, qt.IsNil)
	err = st.AddValidator(&models.Validator{
		Address: []byte("validator2"),
		Power:   20,
	})
	qt.Assert(t, err, qt.IsNil)

	nostate := st.NoState(true)
	err = nostate.Set([]byte("nostate1"), []byte("value1"))
	qt.Assert(t, err, qt.IsNil)
	err = nostate.Set([]byte("nostate2"), []byte("value2"))
	qt.Assert(t, err, qt.IsNil)

	_, err = st.PrepareCommit()
	qt.Assert(t, err, qt.IsNil)
	hash, err := st.Save()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, hash, qt.Not(qt.IsNil))
}

func newStateForTest(t *testing.T) *State {
	state, err := New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	return state
}
