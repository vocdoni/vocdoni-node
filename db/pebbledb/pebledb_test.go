package pebbledb

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/internal/dbtest"
)

func TestWriteTx(t *testing.T) {
	database, err := New(db.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestWriteTx(t, database)
}

func TestIterate(t *testing.T) {
	database, err := New(db.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestIterate(t, database)
}

// NOTE: This test fails.  pebble.Batch doesn't detect conflicts.  Moreover,
// reads from a pebble.Batch return the last version from the Database, even if
// the update was made after the pebble.Batch was created.  Basically it's not
// a Transaction, but a Batch of write operations.
// func TestConcurrentWriteTx(t *testing.T) {
// 	database, err := New(db.Options{Path: t.TempDir()})
// 	qt.Assert(t, err, qt.IsNil)
//
// 	dbtest.TestConcurrentWriteTx(t, database)
// }
