package prefixeddb

import (
	"bytes"

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
	return jointPrefix[:len(jointPrefix):len(jointPrefix)]
}

// NewPrefixedDatabase creates a new PrefixedDatabase.  If the db is already a
// PrefixedDatabase, instead of wrapping again, the prefixes are appended to
// avoid unecessay layers.
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

// ReadTx returns a db.ReadTx
func (d *PrefixedDatabase) ReadTx() db.ReadTx {
	return NewPrefixedReadTx(d.db.ReadTx(), d.prefix)
}

// WriteTx returns a db.WriteTx
func (d *PrefixedDatabase) WriteTx() db.WriteTx {
	return NewPrefixedWriteTx(d.db.WriteTx(), d.prefix)
}

// Iterate implements the db.Database.Iterate interface method
func (d *PrefixedDatabase) Iterate(prefix []byte, callback func(key, value []byte) bool) error {
	return d.db.Iterate(prefixSlice(d.prefix, prefix), func(key, value []byte) bool {
		return callback(bytes.TrimPrefix(key, d.prefix), value)
	})
}

// PrefixedReadTx wraps a db.ReadTx prefixing all keys with `prefix`.
type PrefixedReadTx struct {
	prefix []byte
	tx     db.ReadTx
}

// check that PrefixedReadTx implements the db.ReadTx interface
var _ db.ReadTx = (*PrefixedReadTx)(nil)

// NewPrefixedDatabase creates a new db.ReadTx.  If the tx is already a
// PrefixedReadTx, instead of wrapping again, the prefixes are appended to avoid
// unecessay layers.
func NewPrefixedReadTx(tx db.ReadTx, prefix []byte) *PrefixedReadTx {
	if ptx, ok := tx.(*PrefixedReadTx); ok {
		return &PrefixedReadTx{prefixSlice(ptx.prefix, prefix), ptx.tx}
	}
	return &PrefixedReadTx{prefix, tx}
}

// Get implements the db.ReadTx.Get interface method
func (t *PrefixedReadTx) Get(key []byte) ([]byte, error) {
	return t.tx.Get(prefixSlice(t.prefix, key))
}

// Discard implements the db.ReadTx.Discard interface method.  Notice that this
// method also discards the wrapped db.ReadTx.
func (t *PrefixedReadTx) Discard() {
	t.tx.Discard()
}

// PrefixedWriteTx wraps a WriteTx prefixing all keys with `prefix`.
type PrefixedWriteTx struct {
	prefix []byte
	tx     db.WriteTx
}

// check that PrefixedWriteTx implements the db.ReadTx & db.WriteTx interfaces
var _ db.ReadTx = (*PrefixedWriteTx)(nil)
var _ db.WriteTx = (*PrefixedWriteTx)(nil)

// NewPrefixedWriteTx creates a new db.WriteTx.  If the tx is already a
// PrefixedWriteTx, instead of wrapping again, the prefixes are appended to avoid
// unecessay layers.
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

// Discard implements the db.ReadTx.Discard interface method.  Notice that this
// method also discards the wrapped db.WriteTx.
func (t *PrefixedWriteTx) Discard() {
	t.tx.Discard()
}

// Set implements the db.WriteTx.Set interface method
func (t *PrefixedWriteTx) Set(key []byte, value []byte) error {
	return t.tx.Set(prefixSlice(t.prefix, key), value)
}

// Delete implements the db.WriteTx.Delete interface method
func (t *PrefixedWriteTx) Delete(key []byte) error {
	return t.tx.Delete(prefixSlice(t.prefix, key))
}

// Commit implements the db.WriteTx.Commit interface method.  Notice that this
// method also commits the wrapped db.WriteTx.
func (t *PrefixedWriteTx) Commit() error {
	return t.tx.Commit()
}
