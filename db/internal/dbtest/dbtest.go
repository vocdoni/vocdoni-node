package dbtest

import (
	"bytes"
	"strconv"
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
