package badgerdb

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/dgraph-io/badger/v3"
	"go.vocdoni.io/dvote/db"
)

var inTest = len(os.Args) > 0 && strings.HasSuffix(strings.TrimSuffix(os.Args[0], ".exe"), ".test")

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
var _ db.WriteTx = (*WriteTx)(nil)

// Get implements the db.ReadTx.Get interface method
func (tx ReadTx) Get(k []byte) ([]byte, error) {
	item, err := tx.tx.Get(k)
	if errors.Is(err, badger.ErrKeyNotFound) {
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
	if err := tx.tx.Set(k, v); errors.Is(err, badger.ErrTxnTooBig) {
		return db.ErrTxnTooBig
	} else {
		return err
	}
}

// Delete implements the db.WriteTx.Delete interface method
func (tx WriteTx) Delete(k []byte) error {
	if err := tx.tx.Delete(k); errors.Is(err, badger.ErrTxnTooBig) {
		return db.ErrTxnTooBig
	} else {
		return err
	}
}

// Commit implements the db.WriteTx.Commit interface method
func (tx WriteTx) Commit() error {
	// Note that badger's Txn.Commit will not call discard if the transaction
	// has zero pending writes.
	// Seems like a potential bug or leak? In any case, always discard.
	defer tx.tx.Discard()

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

// New returns a BadgerDB using the given Options, which implements the
// db.Database interface
func New(opts db.Options) (*BadgerDB, error) {
	if err := os.MkdirAll(opts.Path, os.ModePerm); err != nil {
		return nil, err
	}
	badgerOpts := badger.DefaultOptions(opts.Path).
		WithLogger(nil).
		// Do we want compression in the future?
		// WithCompression(options.Snappy).
		WithSyncWrites(false).
		WithCompression(0).
		WithBlockCacheSize(0).
		WithNumCompactors(20).
		WithNumMemtables(1).
		WithBlockSize(16)

	badgerOpts.MemTableSize = MemTableSize
	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, err
	}
	runtime.SetFinalizer(db, func(db *badger.DB) {
		if !db.IsClosed() {
			if inTest {
				panic("badgerdb: database was not closed")
			} else {
				println("warning: badgerdb: database was not closed")
				db.Close()
			}
		}
	})

	return &BadgerDB{
		db: db,
	}, nil
}

func isDiscarded(tx *badger.Txn) bool {
	// If the tx isn't discarded, this key should throw an "invalid key" error.
	// This helps the Get be a cheap no-op either way.
	// Unfortunately, badger has no IsDiscarded method.
	err := tx.Set([]byte("!badger!banned"), nil)
	switch err {
	case badger.ErrDiscardedTxn:
		return true
	case badger.ErrInvalidKey:
		return false
	default:
		panic(fmt.Sprintf("unexpected isDiscarded Set error: %v", err))
	}
}

// ReadTx returns a db.ReadTx
func (db *BadgerDB) ReadTx() db.ReadTx {
	tx := db.db.NewTransaction(false)
	// Unfortunately, we can't easily use isDiscarded on read-only transactions,
	// as tx.Set quickly errors there as an invalid operation.
	// Using tx.Get would get us the same information, but might not be a no-op.
	return WriteTx{tx: tx}
}

// WriteTx returns a db.WriteTx
func (db *BadgerDB) WriteTx() db.WriteTx {
	tx := db.db.NewTransaction(true)
	var stack []byte
	if inTest {
		stack = debug.Stack()
	}
	runtime.SetFinalizer(tx, func(tx *badger.Txn) {
		if !isDiscarded(tx) {
			if inTest {
				panic("badgerdb: write tx was not committed or discarded; creator stack:\n\n" +
					string(stack) + "\nfinalize stack (to ignore):\n")
			} else {
				println("warning: badgerdb: write tx was not committed or discarded")
				tx.Discard()
			}
		}
	})
	return WriteTx{tx: tx}
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
