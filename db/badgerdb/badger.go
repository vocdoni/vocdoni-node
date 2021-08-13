package badgerdb

import (
	"os"

	"github.com/dgraph-io/badger/v3"
	"go.vocdoni.io/dvote/db"
)

// MemTableSize defines the BadgerDB maximum size in bytes for memtable table.
// The default is 64<<20 (64MB), this does not pre-allocate enough memory for
// big Txs, that's why we use 128<<20.
const MemTableSize = 128 << 20

// ReadTx implements the interface db.ReadTx
type ReadTx struct {
	tx *badger.Txn
}

// check that ReadTx implements the db.ReadTx interface
var _ db.ReadTx = (*ReadTx)(nil)

// WriteTx implements the interface db.WriteTx
type WriteTx struct {
	tx *badger.Txn
}

// check that WriteTx implements the db.ReadTx & db.WriteTx interfaces
var _ db.ReadTx = (*WriteTx)(nil)
var _ db.WriteTx = (*WriteTx)(nil)

// Get implements the db.ReadTx.Get interface method
func (tx ReadTx) Get(k []byte) ([]byte, error) {
	item, err := tx.tx.Get(k)
	if err == badger.ErrKeyNotFound {
		return nil, db.ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
}

// Discard implements the db.ReadTx.Discard interface method
func (tx ReadTx) Discard() {
	tx.tx.Discard()
}

// Get implements the db.WriteTx.Get interface method
func (tx WriteTx) Get(k []byte) ([]byte, error) {
	return ReadTx(tx).Get(k)
}

// Set implements the db.WriteTx.Set interface method
func (tx WriteTx) Set(k, v []byte) error {
	return tx.tx.Set(k, v)
}

// Delete implements the db.WriteTx.Delete interface method
func (tx WriteTx) Delete(k []byte) error {
	return tx.tx.Delete(k)
}

// Commit implements the db.WriteTx.Commit interface method
func (tx WriteTx) Commit() error {
	return tx.tx.Commit()
}

// Discard implements the db.WriteTx.Discard interface method
func (tx WriteTx) Discard() {
	ReadTx(tx).Discard()
}

// BadgerDB implements db.Database interface
type BadgerDB struct {
	db *badger.DB
}

// check that BadgerDB implements the db.Database interface
var _ db.Database = (*BadgerDB)(nil)

// Options defines params for creating a new BadgerDB
type Options struct {
	Path string
}

// New returns a BadgerDB using the given Options, which implements the
// db.Database interface
func New(opts Options) (*BadgerDB, error) {
	if err := os.MkdirAll(opts.Path, os.ModePerm); err != nil {
		return nil, err
	}
	badgerOpts := badger.DefaultOptions(opts.Path).
		WithLogger(nil).
		// Do we want compression in the future?
		// WithCompression(options.Snappy).
		WithSyncWrites(false)

	badgerOpts.MemTableSize = MemTableSize

	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, err
	}

	return &BadgerDB{
		db: db,
	}, nil
}

// ReadTx returns a db.ReadTx
func (db *BadgerDB) ReadTx() db.ReadTx {
	return ReadTx{
		tx: db.db.NewTransaction(false),
	}
}

// WriteTx returns a db.WriteTx
func (db *BadgerDB) WriteTx() db.WriteTx {
	return WriteTx{
		tx: db.db.NewTransaction(true),
	}
}

// Close closes the BadgerDB
func (db *BadgerDB) Close() error {
	return db.db.Close()
}

// Iterate implements the db.Database.Iterate interface method
func (db *BadgerDB) Iterate(prefix []byte, callback func(k, v []byte) bool) error {
	return db.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		if prefix != nil {
			opts.Prefix = prefix
		}
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			stopIter := false
			err := item.Value(func(v []byte) error {
				if cont := callback(item.Key(), v); !cont {
					stopIter = true
				}
				return nil
			})
			if err != nil {
				return err
			}
			if stopIter {
				break
			}
		}
		return nil
	})
}
