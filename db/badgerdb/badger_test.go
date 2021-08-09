package badgerdb

import (
	"bytes"
	"strconv"
	"testing"

	"go.vocdoni.io/dvote/db"
)

func TestWriteTx(t *testing.T) {
	database, err := New(Options{Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	wTx := database.WriteTx()

	if _, err = wTx.Get([]byte("a")); err != db.ErrKeyNotFound {
		t.Fatal(err)
	}

	if err = wTx.Set([]byte("a"), []byte("b")); err != nil {
		t.Fatal(err)
	}

	v, err := wTx.Get([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(v, []byte("b")) {
		t.Errorf("expected v (%v) to be equal to %v", v, []byte("b"))
	}
	if err = wTx.Commit(); err != nil {
		t.Fatal(err)
	}
	// Discard should not give any problem, as Commit inside does a
	// Discard, but in badger docs they recommend to use Discard anyway
	// after a Commit:
	// https://pkg.go.dev/github.com/dgraph-io/badger/v3#Txn.Discard
	wTx.Discard()

	// get value from a new db after the previous commit
	wTx = database.WriteTx()
	v, err = wTx.Get([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(v, []byte("b")) {
		t.Errorf("expected v (%v) to be equal to %v", v, []byte("b"))
	}

	// ensure that WriteTx can be passed into a function that accepts
	// ReadTx, and that can be used
	useReadTxFromWriteTx(t, wTx)
}

func useReadTxFromWriteTx(t *testing.T, rTx db.ReadTx) {
	v, err := rTx.Get([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(v, []byte("b")) {
		t.Errorf("expected v (%v) to be equal to %v", v, []byte("b"))
	}
}

func TestIterate(t *testing.T) {
	d, err := New(Options{Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

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
	if err := wTx.Commit(); err != nil {
		t.Fatal(err)
	}

	noPrefixKeysFound := 0
	if err := d.Iterate(nil, func(k, v []byte) bool {
		noPrefixKeysFound++
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if noPrefixKeysFound != (prefix0NumKeys + prefix1NumKeys) {
		t.Errorf("expected %d keys, found %d", prefix0NumKeys+prefix1NumKeys, noPrefixKeysFound)
	}

	prefix0KeysFound := 0
	if err := d.Iterate(prefix0, func(k, v []byte) bool {
		prefix0KeysFound++
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if prefix0KeysFound != prefix0NumKeys {
		t.Errorf("expected %d keys, found %d", prefix0NumKeys, prefix0KeysFound)
	}

	prefix1KeysFound := 0
	if err := d.Iterate(prefix1, func(k, v []byte) bool {
		prefix1KeysFound++
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if prefix1KeysFound != prefix1NumKeys {
		t.Errorf("expected %d keys, found %d", prefix1NumKeys, prefix1KeysFound)
	}
}
