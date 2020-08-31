package db

import (
	"os"

	"github.com/dgraph-io/badger/v2"
)

// BadgerDB implements chainsafe's database interface. The implementation is
// inspired by the original by chainsafe, but also different.
type BadgerDB struct {
	path string
	db   *badger.DB
}

func NewBadgerDB(path string) (*BadgerDB, error) {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions(path).
		WithLogger(nil).
		// Do we want compression in the future?
		// WithCompression(options.Snappy).
		WithSyncWrites(false)

	// The default is 64<<20, which means Badger pre-allocates quite a lot
	// of memory. Lower it a bit.
	opts.MaxTableSize /= 4

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &BadgerDB{
		path: path,
		db:   db,
	}, nil
}

func (db *BadgerDB) Path() string { return db.path }

type batchWriter struct {
	wb badger.WriteBatch
}

func (db *BadgerDB) NewBatch() Batch {
	return &batchWriter{
		wb: *db.db.NewWriteBatch(), // avoid double indirection
	}
}

func (db *BadgerDB) Put(key []byte, value []byte) error {
	return db.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (db *BadgerDB) Has(key []byte) (bool, error) {
	exists := true
	err := db.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			exists = false
			err = nil
		}
		return err
	})
	return exists, err
}

func (db *BadgerDB) Get(key []byte) ([]byte, error) {
	var value []byte
	err := db.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	return value, err
}

func (db *BadgerDB) Del(key []byte) error {
	return db.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete(key)
		if err == badger.ErrKeyNotFound {
			err = nil
		}
		return err
	})
}

func (db *BadgerDB) Close() error {
	return db.db.Close()
}

type BadgerIterator struct {
	txn      *badger.Txn
	Iter     *badger.Iterator
	first    bool // so that the first Next does a Rewind
	released bool
}

func (db *BadgerDB) NewIterator() Iterator {
	txn := db.db.NewTransaction(false)
	iter := txn.NewIterator(badger.DefaultIteratorOptions)
	return &BadgerIterator{
		txn:   txn,
		Iter:  iter,
		first: true,
	}
}

func (i *BadgerIterator) Release() {
	i.Iter.Close()
	i.txn.Discard()
	i.released = true
}

func (i *BadgerIterator) Released() bool {
	return i.released
}

func (i *BadgerIterator) Next() bool {
	if i.first {
		// For the first element, we only rewind.
		// Don't call iter.Next, as that would skip the first element
		// entirely.
		i.Iter.Rewind()
		i.first = false
	} else {
		i.Iter.Next()
	}
	return i.Iter.Valid()
}

func (i *BadgerIterator) Seek(key []byte) {
	i.Iter.Seek(key)
}

func (i *BadgerIterator) Key() []byte {
	return i.Iter.Item().Key()
}

func (i *BadgerIterator) Value() []byte {
	val, err := i.Iter.Item().ValueCopy(nil)
	if err != nil {
		panic(err)
	}
	return val
}

func (b *batchWriter) Put(key, value []byte) error {
	return b.wb.Set(key, value)
}

func (b *batchWriter) Write() error {
	return b.wb.Flush()
}

func (b *batchWriter) ValueSize() int {
	panic("unimplemented")
}

func (b *batchWriter) Del(key []byte) error {
	return b.wb.Delete(key)
}

func (b *batchWriter) Reset() {
	panic("unimplemented")
}
