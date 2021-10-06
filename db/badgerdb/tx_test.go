package badgerdb

import (
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v3"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
)

var key = []byte{1}

func inc(t *testing.T, m *sync.Mutex, db *BadgerDB) error {
	wTx := db.WriteTx()
	time.Sleep(100 * time.Millisecond)

	m.Lock()
	val, err := wTx.Get(key)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, wTx.Set(key, []byte{val[0] + 1}), qt.IsNil)
	m.Unlock()

	return wTx.Commit()
}

// TestConcurrentWriteTx validates the behaviour of badgerdb when multiple
// write transactions modify the same key.
func TestConcurrentWriteTx(t *testing.T) {
	db, err := New(db.Options{Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	wTx := db.WriteTx()
	qt.Assert(t, wTx.Set(key, []byte{0}), qt.IsNil)
	qt.Assert(t, wTx.Commit(), qt.IsNil)

	var wg sync.WaitGroup
	var m sync.Mutex
	// A
	var errA error
	wg.Add(1)
	go func() {
		errA = inc(t, &m, db)
		wg.Done()
	}()

	// B
	var errB error
	wg.Add(1)
	go func() {
		errB = inc(t, &m, db)
		wg.Done()
	}()

	wg.Wait()
	rTx := db.ReadTx()
	defer rTx.Discard()
	val, err := rTx.Get(key)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, val, qt.DeepEquals, []byte{1})

	if errA == nil {
		qt.Assert(t, errA, qt.IsNil)
		qt.Assert(t, errB, qt.Equals, badger.ErrConflict)
	} else {
		qt.Assert(t, errA, qt.Equals, badger.ErrConflict)
		qt.Assert(t, errB, qt.IsNil)
	}
}
