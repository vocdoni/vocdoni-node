package pebbledb

import (
	"errors"
	"os"

	"github.com/cockroachdb/pebble"
	"go.vocdoni.io/dvote/db"
)

// ReadTx implements the interface db.ReadTx
type ReadTx struct {
	batch *pebble.Batch
}

// check that ReadTx implements the db.ReadTx interface
var _ db.ReadTx = (*ReadTx)(nil)

// WriteTx implements the interface db.WriteTx
type WriteTx struct {
	batch *pebble.Batch
}

// check that WriteTx implements the db.ReadTx & db.WriteTx interfaces
var _ db.WriteTx = (*WriteTx)(nil)

// Get implements the db.ReadTx.Get interface method
func (tx ReadTx) Get(k []byte) ([]byte, error) {
	v, closer, err := tx.batch.Get(k)
	if errors.Is(err, pebble.ErrNotFound) {
		return nil, db.ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	// Note that the returned value slice is only valid until Close is called.
	// Make a copy so we can return it.
	// TODO(mvdan): write a dbtest test to ensure this property on all DBs.
	v2 := make([]byte, len(v))
	copy(v2, v)

	if err := closer.Close(); err != nil {
		return nil, err
	}
	return v2, nil
}

// Discard implements the db.ReadTx.Discard interface method
func (tx ReadTx) Discard() {
	// Close returns an error, but here in the Discard context is ommited
	tx.batch.Close()
}

// Get implements the db.WriteTx.Get interface method
func (tx WriteTx) Get(k []byte) ([]byte, error) {
	return ReadTx(tx).Get(k)
}

// Set implements the db.WriteTx.Set interface method
func (tx WriteTx) Set(k, v []byte) error {
	return tx.batch.Set(k, v, nil)
}

// Delete implements the db.WriteTx.Delete interface method
func (tx WriteTx) Delete(k []byte) error {
	return tx.batch.Delete(k, nil)
}

// Apply implements the db.WriteTx.Apply interface method
func (tx WriteTx) Apply(other db.WriteTx) (err error) {
	otherPebble := other.(WriteTx)
	return tx.batch.Apply(otherPebble.batch, nil)
}

// Commit implements the db.WriteTx.Commit interface method
func (tx WriteTx) Commit() error {
	return tx.batch.Commit(nil)
}

// Discard implements the db.WriteTx.Discard interface method
func (tx WriteTx) Discard() {
	ReadTx(tx).Discard()
}

// PebbleDB implements db.Database interface
type PebbleDB struct {
	db *pebble.DB
}

// check that PebbleDB implements the db.Database interface
var _ db.Database = (*PebbleDB)(nil)

// New returns a PebbleDB using the given Options, which implements the
// db.Database interface
func New(opts db.Options) (*PebbleDB, error) {
	if err := os.MkdirAll(opts.Path, os.ModePerm); err != nil {
		return nil, err
	}
	o := &pebble.Options{}
	db, err := pebble.Open(opts.Path, o)
	if err != nil {
		return nil, err
	}

	return &PebbleDB{
		db: db,
	}, nil
}

// ReadTx returns a db.ReadTx
func (db *PebbleDB) ReadTx() db.ReadTx {
	return ReadTx{
		batch: db.db.NewIndexedBatch(),
	}
}

// WriteTx returns a db.WriteTx
func (db *PebbleDB) WriteTx() db.WriteTx {
	return WriteTx{
		batch: db.db.NewIndexedBatch(),
	}
}

// Close closes the PebbleDB
func (db *PebbleDB) Close() error {
	return db.db.Close()
}

func keyUpperBound(b []byte) []byte {
	// https://github.com/cockroachdb/pebble/blob/b2eb88a7182687c81d911c425309ef0e1f545452/iterator_example_test.go#L44
	end := make([]byte, len(b))
	copy(end, b)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil // no upper-bound
}

// Iterate implements the db.Database.Iterate interface method
func (db *PebbleDB) Iterate(prefix []byte, callback func(k, v []byte) bool) (err error) {
	iterOptions := &pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	}
	iter := db.db.NewIter(iterOptions)
	defer func() {
		errC := iter.Close()
		if err != nil {
			return
		}
		err = errC
	}()

	for iter.First(); iter.Valid(); iter.Next() {
		localKey := iter.Key()[len(prefix):]
		if cont := callback(localKey, iter.Value()); !cont {
			break
		}
	}
	return iter.Error()
}
