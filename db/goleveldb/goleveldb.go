package goleveldb

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"go.vocdoni.io/dvote/db"
)

type LevelDB struct {
	db *leveldb.DB
}

// Ensure that LevelDB implements the db.Database interface
var _ db.Database = (*LevelDB)(nil)

// New returns a LevelDB which implements the db.Database interface
func New(opts db.Options) (*LevelDB, error) {
	// Open the LevelDB database
	db, err := leveldb.OpenFile(opts.Path, &opt.Options{})
	if err != nil {
		return nil, fmt.Errorf("could not open leveldb: %w", err)
	}
	return &LevelDB{
		db: db,
	}, nil
}

func (d *LevelDB) Close() error {
	return d.db.Close()
}

func (d *LevelDB) WriteTx() db.WriteTx {
	return &WriteTx{
		db:    d.db,
		batch: new(leveldb.Batch),
	}
}

func (d *LevelDB) Get(key []byte) ([]byte, error) {
	val, err := d.db.Get(key, nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return nil, db.ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (d *LevelDB) Iterate(prefix []byte, callback func(key, value []byte) bool) error {
	iter := d.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()
	for iter.Next() {
		if !callback(iter.Key(), iter.Value()) {
			break
		}
	}
	return iter.Error()
}

func (d *LevelDB) Set(key, value []byte) error {
	return d.db.Put(key, value, nil)
}

func (d *LevelDB) Delete(key []byte) error {
	return d.db.Delete(key, nil)
}

func (d *LevelDB) Commit(batch *leveldb.Batch) error {
	return d.db.Write(batch, nil)
}

// Compact implements the db.Database.Compact interface method.
func (d *LevelDB) Compact() error {
	return d.db.CompactRange(util.Range{})
}

// WriteTx implements the interface db.WriteTx for goleveldb
type WriteTx struct {
	batch      *leveldb.Batch
	db         *leveldb.DB
	inMemBatch sync.Map
}

// check that WriteTx implements the db.WriteTx interface
var _ db.WriteTx = (*WriteTx)(nil)

func (tx *WriteTx) Get(k []byte) ([]byte, error) {
	val, ok := tx.inMemBatch.Load(string(k))
	if !ok {
		val, err := tx.db.Get(k, nil)
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, db.ErrKeyNotFound
		}
		return val, err
	}
	return val.([]byte), nil
}

func (tx *WriteTx) Iterate(prefix []byte, callback func(k, v []byte) bool) error {
	inMemory := make(map[string]bool)
	tx.inMemBatch.Range(func(k, v any) bool {
		keyBytes := []byte(k.(string))
		if bytes.HasPrefix(keyBytes, prefix) {
			inMemory[string(keyBytes)] = true
			return callback(keyBytes, v.([]byte))
		}
		return true
	})
	iter := tx.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()
	for iter.Next() {
		if inMemory[string(iter.Key())] {
			continue
		}
		if !callback(iter.Key(), iter.Value()) {
			break
		}
	}
	return iter.Error()
}

func (tx *WriteTx) Set(k, v []byte) error {
	tx.batch.Put(k, v)
	tx.inMemBatch.Store(string(k), v)
	return nil
}

func (tx *WriteTx) Delete(k []byte) error {
	tx.batch.Delete(k)
	tx.inMemBatch.Delete(string(k))
	return nil
}

func (tx *WriteTx) Apply(otherTx db.WriteTx) error {
	return otherTx.Iterate(nil, func(k, v []byte) bool {
		tx.inMemBatch.Store(string(k), v)
		tx.batch.Put(k, v)
		return true
	})
}

func (tx *WriteTx) Commit() error {
	return tx.db.Write(tx.batch, nil)
}

func (tx *WriteTx) Discard() {
	tx.batch.Reset()
	tx.inMemBatch = sync.Map{}
}
