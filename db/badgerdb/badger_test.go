package badgerdb

import (
	"encoding/binary"
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

func TestConcurrentWriteTx(t *testing.T) {
	database, err := New(db.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestConcurrentWriteTx(t, database)
}

func TestWriteTxApply(t *testing.T) {
	database, err := New(db.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestWriteTxApply(t, database)
}

func TestBatch(t *testing.T) {
	database, err := New(db.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	const n = 1_000_000
	wTx := database.WriteTx()
	var key [4]byte
	var value [4]byte
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(key[:], uint32(i))
		binary.LittleEndian.PutUint32(value[:], uint32(i))
		if err = wTx.Set(key[:], value[:]); err != nil {
			break
		}
	}
	// A WriteTx with too many writes fails because the Txn becomes too big
	qt.Assert(t, err, qt.Equals, db.ErrTxnTooBig)
	wTx.Discard()

	batch := db.NewBatch(database)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(key[:], uint32(i))
		binary.LittleEndian.PutUint32(value[:], uint32(i))
		if err = batch.Set(key[:], value[:]); err != nil {
			break
		}
	}
	// A Batch will commit automatically during writes if the Txn becomes
	// too big, so we expect all the writes to work and no error here.
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, batch.Commit(), qt.IsNil)
}
