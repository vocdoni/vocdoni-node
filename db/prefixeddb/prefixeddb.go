package prefixeddb

import (
	"bytes"
	"slices"

	"go.vocdoni.io/dvote/db"
)

// PrefixedDatabase wraps a db.Database prefixing all keys with `prefix`.
type PrefixedDatabase struct {
	prefix []byte
	db     db.Database
}

// check that PrefixedDatabase implements the db.Database interface
var _ db.Database = (*PrefixedDatabase)(nil)

func prefixSlice(prefix, v []byte) []byte {
	// Copy prefix to ensure we don't reuse the slice's backing array.
	jointPrefix := make([]byte, 0, len(prefix)+len(v))
	jointPrefix = append(jointPrefix, prefix...)
	jointPrefix = append(jointPrefix, v...)
	// Make sure the prefix's capacity is fixed, to prevent future appends from sharing memory.
	return slices.Clip(jointPrefix)
}

// NewPrefixedDatabase creates a new PrefixedDatabase.  If the db is already a
// PrefixedDatabase, instead of wrapping again, the prefixes are appended to
// avoid unnecessary layers.
func NewPrefixedDatabase(db db.Database, prefix []byte) *PrefixedDatabase {
	if pdb, ok := db.(*PrefixedDatabase); ok {
		return &PrefixedDatabase{prefixSlice(pdb.prefix, prefix), pdb.db}
	}
	return &PrefixedDatabase{prefix, db}
}

// Close implements the db.Database.Close interface method.  Notice that this
// method also closes the wrapped db.Database.
func (d *PrefixedDatabase) Close() error {
	return d.db.Close()
}

// Compact implements the db.Database.Compact interface method.
func (d *PrefixedDatabase) Compact() error {
	return d.db.Compact()
}

// WriteTx returns a db.WriteTx
func (d *PrefixedDatabase) WriteTx() db.WriteTx {
	return NewPrefixedWriteTx(d.db.WriteTx(), d.prefix)
}

// Get implements the db.Database.Get interface method
func (d *PrefixedDatabase) Get(key []byte) ([]byte, error) {
	return d.db.Get(prefixSlice(d.prefix, key))
}

// Iterate implements the db.Database.Iterate interface method
func (d *PrefixedDatabase) Iterate(prefix []byte, callback func(key, value []byte) bool) error {
	return d.db.Iterate(prefixSlice(d.prefix, prefix), func(key, value []byte) bool {
		return callback(bytes.TrimPrefix(key, d.prefix), value)
	})
}

// PrefixedReader wraps a Reader tx prefixing all keys with `prefix`.
type PrefixedReader struct {
	prefix []byte
	rd     db.Reader
}

// check that PrefixedDatabase implements the db.Database interface
var _ db.Reader = (*PrefixedReader)(nil)

// NewPrefixedReader creates a new PrefixedReader.  If the rd is already a
// PrefixedReader, instead of wrapping again, the prefixes are appended to
// avoid unnecessary layers.
func NewPrefixedReader(rd db.Reader, prefix []byte) *PrefixedReader {
	if pdb, ok := rd.(*PrefixedReader); ok {
		return &PrefixedReader{prefixSlice(pdb.prefix, prefix), pdb.rd}
	}
	return &PrefixedReader{prefix, rd}
}

func (d *PrefixedReader) Get(key []byte) ([]byte, error) {
	return d.rd.Get(prefixSlice(d.prefix, key))
}

// Iterate implements the db.Database.Iterate interface method
func (d *PrefixedReader) Iterate(prefix []byte, callback func(key, value []byte) bool) error {
	return d.rd.Iterate(prefixSlice(d.prefix, prefix), func(key, value []byte) bool {
		return callback(bytes.TrimPrefix(key, d.prefix), value)
	})
}

// PrefixedWriteTx wraps a WriteTx prefixing all keys with `prefix`.
type PrefixedWriteTx struct {
	prefix []byte
	tx     db.WriteTx
}

// check that PrefixedWriteTx implements the db.WriteTx interface
var _ db.WriteTx = (*PrefixedWriteTx)(nil)

// NewPrefixedWriteTx creates a new db.WriteTx.  If the tx is already a
// PrefixedWriteTx, instead of wrapping again, the prefixes are appended to avoid
// unnecessary layers.
func NewPrefixedWriteTx(tx db.WriteTx, prefix []byte) *PrefixedWriteTx {
	if ptx, ok := tx.(*PrefixedWriteTx); ok {
		return &PrefixedWriteTx{prefixSlice(ptx.prefix, prefix), ptx.tx}
	}
	return &PrefixedWriteTx{prefix, tx}
}

// Get implements the db.WriteTx.Get interface method
func (t *PrefixedWriteTx) Get(key []byte) ([]byte, error) {
	return t.tx.Get(prefixSlice(t.prefix, key))
}

func (t *PrefixedWriteTx) Iterate(prefix []byte, callback func(key, value []byte) bool) error {
	return t.tx.Iterate(prefixSlice(t.prefix, prefix), func(key, value []byte) bool {
		return callback(bytes.TrimPrefix(key, t.prefix), value)
	})
}

// Discard implements the db.WriteTx.Discard interface method.  Notice that this
// method also discards the wrapped db.WriteTx.
func (t *PrefixedWriteTx) Discard() {
	t.tx.Discard()
}

// Set implements the db.WriteTx.Set interface method
func (t *PrefixedWriteTx) Set(key, value []byte) error {
	return t.tx.Set(prefixSlice(t.prefix, key), value)
}

// Delete implements the db.WriteTx.Delete interface method
func (t *PrefixedWriteTx) Delete(key []byte) error {
	return t.tx.Delete(prefixSlice(t.prefix, key))
}

// Apply implements the db.WriteTx.Apply interface method
func (t *PrefixedWriteTx) Apply(other db.WriteTx) error {
	return t.tx.Apply(other)
}

// Unwrap returns the wrapped WriteTx
func (t *PrefixedWriteTx) Unwrap() db.WriteTx {
	return t.tx
}

// Commit implements the db.WriteTx.Commit interface method.  Notice that this
// method also commits the wrapped db.WriteTx.
func (t *PrefixedWriteTx) Commit() error {
	return t.tx.Commit()
}
