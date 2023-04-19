package db

import (
	"errors"
)

// Batch wraps a WriteTx to automatically commit when it becomes too big
// (either the Tx write operation returns ErrTxnTooBig or the write count
// reaches the specified maxSize),
// causing a new internal WriteTx to be created.
type Batch struct {
	tx      WriteTx
	db      Database
	maxSize uint
	count   uint
}

// check that Batch implements the ReadTx & WriteTx interfaces
var _ ReadTx = (*Batch)(nil)
var _ WriteTx = (*Batch)(nil)

// DefaultBatchMaxSize is the default value for maxSize used in NewBatch.
// Assuming keys and values of 64 bytes, which should be common in our use
// case, this value allows for 128 MiB of key-values which should be a nice
// tradeoff between performance and memory consumption.
const DefaultBatchMaxSize uint = 1024 * 1024

// NewBatch creates a new Batch from a Database with a default maxSize.
func NewBatch(db Database) *Batch {
	return &Batch{
		tx:      db.WriteTx(),
		db:      db,
		maxSize: DefaultBatchMaxSize,
	}
}

// NewBatchMaxSize creates a new Batch from a Database with the specified maxSize.
func NewBatchMaxSize(db Database, maxSize uint) *Batch {
	return &Batch{
		tx:      db.WriteTx(),
		db:      db,
		maxSize: maxSize,
	}
}

// Get implements the WriteTx.Get interface method.
func (t *Batch) Get(key []byte) ([]byte, error) {
	return t.tx.Get(key)
}

// Discard implements the ReadTx.Discard interface method.  Notice that this
// method will only discard the writes of the last interal tx: if the tx
// previously has become too big a commit will have been applied that can't be
// discarded.
func (t *Batch) Discard() {
	t.tx.Discard()
}

// Apply implements the WriteTx.Apply interface method
func (t *Batch) Apply(other WriteTx) (err error) {
	return t.tx.Apply(other)
}

// Unwrap returns the wrapped WriteTx
func (t *Batch) Unwrap() WriteTx {
	return t.tx
}

// Set implements the WriteTx.Set interface method.  If during this
// operation, the internal tx becomes too big, all the pending writes will be
// committed and a new WriteTx will be created to continue with this and
// future writes.
func (t *Batch) Set(key []byte, value []byte) (err error) {
	defer func() {
		if err == nil {
			t.count++
		}
	}()
	if err := t.tx.Set(key, value); errors.Is(err, ErrTxnTooBig) || t.count+1 == t.maxSize {
		if err := t.tx.Commit(); err != nil {
			return err
		}
		t.count = 0
		t.tx = t.db.WriteTx()
		return t.tx.Set(key, value)
	} else {
		return err
	}
}

// Delete implements the WriteTx.Delete interface method.  If during this
// operation, the internal tx becomes too big, all the pending writes will be
// committed and a new WriteTx will be created to continue with this and
// future writes.
func (t *Batch) Delete(key []byte) (err error) {
	defer func() {
		if err == nil {
			t.count++
		}
	}()
	if err := t.tx.Delete(key); errors.Is(err, ErrTxnTooBig) || t.count+1 == t.maxSize {
		if err := t.tx.Commit(); err != nil {
			return err
		}
		t.count = 0
		t.tx = t.db.WriteTx()
		return t.tx.Delete(key)
	} else {
		return err
	}
}

// Commit implements the WriteTx.Commit interface method.  Notice that this
// method also commits the wrapped WriteTx.  Notice that this
// method will only commit the writes of the last interal tx: if the tx
// previously has become too big a commit will have already been applied to the
// previous tx transparently during a Set or Delete.
func (t *Batch) Commit() error {
	return t.tx.Commit()
}
