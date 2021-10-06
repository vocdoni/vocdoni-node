package prefixeddb

import (
	"testing"

	qt "github.com/frankban/quicktest"
	kv "go.vocdoni.io/dvote/db/pebbledb"
)

func TestPrefixed(t *testing.T) {
	database, err := kv.New(kv.Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	prefix1 := []byte("one")
	prefix2 := []byte("two")

	db1 := NewPrefixedDatabase(database, prefix1)

	keys1, values1 := [][]byte{[]byte("a"), []byte("b")}, [][]byte{[]byte("1"), []byte("2")}
	keys2, values2 := [][]byte{[]byte("c"), []byte("d")}, [][]byte{[]byte("3"), []byte("4")}

	// Insert at prefix "one" with WriteTx from PrefixedDatabase
	wTx1 := db1.WriteTx()
	qt.Assert(t, wTx1.Set(keys1[0], values1[0]), qt.IsNil)
	qt.Assert(t, wTx1.Set(keys1[1], values1[1]), qt.IsNil)
	qt.Assert(t, wTx1.Commit(), qt.IsNil)

	// Insert at prefix "two" with PrefixedWriteTx from Database.WriteTx()
	wTx2 := NewPrefixedWriteTx(database.WriteTx(), prefix2)
	qt.Assert(t, wTx2.Set(keys2[0], values2[0]), qt.IsNil)
	qt.Assert(t, wTx2.Set(keys2[1], values2[1]), qt.IsNil)
	qt.Assert(t, wTx2.Commit(), qt.IsNil)

	// Check key-values in PrefixedDatabase "one"
	i := 0
	err = db1.Iterate(nil, func(key, value []byte) bool {
		qt.Assert(t, key, qt.DeepEquals, keys1[i])
		qt.Assert(t, value, qt.DeepEquals, values1[i])
		i++
		return true
	})
	qt.Assert(t, err, qt.IsNil)

	// Check key-values in PrefixedReadTx "two"
	rTx2 := NewPrefixedReadTx(database.ReadTx(), prefix2)
	v0, err := rTx2.Get(keys2[0])
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v0, qt.DeepEquals, values2[0])
	v1, err := rTx2.Get(keys2[1])
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, v1, qt.DeepEquals, values2[1])
	rTx2.Discard()

	// Check all key-values with prefixes in parent Database
	prefixes := [][]byte{prefix1, prefix1, prefix2, prefix2}
	keys := append(keys1, keys2...)
	values := append(values1, values2...)
	i = 0
	err = database.Iterate(nil, func(key, value []byte) bool {
		qt.Assert(t, key, qt.DeepEquals, append(prefixes[i], keys[i]...))
		qt.Assert(t, value, qt.DeepEquals, values[i])
		i++
		return true
	})
	qt.Assert(t, err, qt.IsNil)
}
