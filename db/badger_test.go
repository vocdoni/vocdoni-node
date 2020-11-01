package db

import (
	"strconv"
	"testing"
)

func TestIter(t *testing.T) {
	d := NewTestDB(t)
	numKeys := 20
	for i := 0; i < numKeys; i++ {
		d.Put([]byte("key"+strconv.Itoa(i)), []byte(strconv.Itoa(i)))
	}
	iter := d.NewIterator().(*BadgerIterator)
	keysFound := 0
	for iter.Next() {
		keysFound++
	}
	if keysFound != numKeys {
		t.Errorf("expected %d keys, found %d", numKeys, keysFound)
	}
}

func TestIterSeek(t *testing.T) {
	d := NewTestDB(t)
	numKeys := 20
	for i := 0; i < numKeys; i++ {
		d.Put([]byte("key"+strconv.Itoa(i)), []byte(strconv.Itoa(i)))
	}
	iter := d.NewIterator().(*BadgerIterator)
	keysFound := 0
	iter.Seek([]byte("key" + strconv.Itoa(0)))
	for iter.Next() {
		keysFound++
	}
	if keysFound != numKeys {
		t.Errorf("expected %d keys, found %d", numKeys, keysFound)
	}
}

func TestIterEmpty(t *testing.T) {
	d := NewTestDB(t)
	iter := d.NewIterator().(*BadgerIterator)
	for i := 0; i < 10; i++ {
		d.Put([]byte("key"+strconv.Itoa(i)), []byte(strconv.Itoa(i)))
	}
	iter.Seek([]byte("yek" + strconv.Itoa(0)))
	keysFound := 0
	for iter.Next(); iter.Iter.ValidForPrefix([]byte("yek")); iter.Next() {
		keysFound++
	}
	if keysFound != 0 {
		t.Errorf("expected 0 keys, found %d", keysFound)
	}
}

func NewTestDB(tb testing.TB) *BadgerDB {
	db, err := NewBadgerDB(tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	return db
}
