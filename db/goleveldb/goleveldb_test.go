package goleveldb

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/internal/dbtest"
	"go.vocdoni.io/dvote/db/prefixeddb"
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

func TestWriteTxApply(t *testing.T) {
	database, err := New(db.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestWriteTxApply(t, database)
}

func TestWriteTxApplyPrefixed(t *testing.T) {
	database, err := New(db.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	prefix := []byte("one")
	dbWithPrefix := prefixeddb.NewPrefixedDatabase(database, prefix)

	dbtest.TestWriteTxApplyPrefixed(t, database, dbWithPrefix)
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
