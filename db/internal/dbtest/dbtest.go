package dbtest

import (
	"bytes"
	"strconv"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
)

func TestWriteTx(t *testing.T, database db.Database) {
	wTx := database.WriteTx()

	if _, err := wTx.Get([]byte("a")); err != db.ErrKeyNotFound {
		t.Fatal(err)
	}

	err := wTx.Set([]byte("a"), []byte("b"))
	qt.Assert(t, err, qt.IsNil)

	v, err := wTx.Get([]byte("a"))
	qt.Assert(t, err, qt.IsNil)

	if !bytes.Equal(v, []byte("b")) {
		t.Errorf("expected v (%v) to be equal to %v", v, []byte("b"))
	}
	err = wTx.Commit()
	qt.Assert(t, err, qt.IsNil)

	// Discard should not give any problem
	wTx.Discard()

	// get value from a new db after the previous commit
	wTx = database.WriteTx()
	v, err = wTx.Get([]byte("a"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v, qt.DeepEquals, []byte("b"))

	// ensure that WriteTx can be passed into a function that accepts
	// ReadTx, and that can be used
	useReadTxFromWriteTx(t, wTx)

	err = wTx.Commit()
	qt.Assert(t, err, qt.IsNil)
}

func useReadTxFromWriteTx(t *testing.T, rTx db.ReadTx) {
	v, err := rTx.Get([]byte("a"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v, qt.DeepEquals, []byte("b"))
}

func TestIterate(t *testing.T, d db.Database) {
	prefix0 := []byte("a")
	prefix0NumKeys := 20
	prefix1 := []byte("b")
	prefix1NumKeys := 30

	wTx := d.WriteTx()
	for i := 0; i < prefix0NumKeys; i++ {
		wTx.Set(append(prefix0, []byte(strconv.Itoa(i))...), []byte(strconv.Itoa(i)))
	}
	for i := 0; i < prefix1NumKeys; i++ {
		wTx.Set(append(prefix1, []byte(strconv.Itoa(i))...), []byte(strconv.Itoa(i)))
	}
	err := wTx.Commit()
	qt.Assert(t, err, qt.IsNil)

	noPrefixKeysFound := 0
	err = d.Iterate(nil, func(k, v []byte) bool {
		noPrefixKeysFound++
		return true
	})
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, noPrefixKeysFound, qt.Equals, prefix0NumKeys+prefix1NumKeys)

	prefix0KeysFound := 0
	err = d.Iterate(prefix0, func(k, v []byte) bool {
		prefix0KeysFound++
		return true
	})
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, prefix0KeysFound, qt.Equals, prefix0NumKeys)

	prefix1KeysFound := 0
	err = d.Iterate(prefix1, func(k, v []byte) bool {
		prefix1KeysFound++
		return true
	})
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, prefix1KeysFound, qt.Equals, prefix1NumKeys)
}

// TestConcurrentWriteTx validates the behaviour of badgerdb when multiple
// write transactions modify the same key.
func TestConcurrentWriteTx(t *testing.T, database db.Database) {
	var key = []byte{1}
	wTx := database.WriteTx()
	qt.Assert(t, wTx.Set(key, []byte{0}), qt.IsNil)
	qt.Assert(t, wTx.Commit(), qt.IsNil)

	var wgSync sync.WaitGroup
	wgSync.Add(2)
	inc := func(t *testing.T, m *sync.Mutex, database db.Database) error {
		var key = []byte{1}
		wTx := database.WriteTx()
		// Sync here so that both goroutines have created a WriteTx
		// before operating with it.
		wgSync.Done()
		wgSync.Wait()

		m.Lock()
		val, err := wTx.Get(key)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, wTx.Set(key, []byte{val[0] + 1}), qt.IsNil)
		m.Unlock()

		return wTx.Commit()
	}

	var wgInc sync.WaitGroup
	var m sync.Mutex
	// A
	var errA error
	wgInc.Add(1)
	go func() {
		errA = inc(t, &m, database)
		wgInc.Done()
	}()

	// B
	var errB error
	wgInc.Add(1)
	go func() {
		errB = inc(t, &m, database)
		wgInc.Done()
	}()

	wgInc.Wait()
	rTx := database.ReadTx()
	defer rTx.Discard()
	val, err := rTx.Get(key)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, val, qt.DeepEquals, []byte{1})

	if errA == nil {
		qt.Assert(t, errA, qt.IsNil)
		qt.Assert(t, errB, qt.Equals, db.ErrConflict)
	} else {
		qt.Assert(t, errA, qt.Equals, db.ErrConflict)
		qt.Assert(t, errB, qt.IsNil)
	}
}
