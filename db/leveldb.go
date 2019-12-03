package db

// modified from https://github.com/iden3/go-iden3/blob/master/db/leveldb.go

import (
	"encoding/json"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"gitlab.com/vocdoni/go-dvote/log"
)

// LevelDbStorage represents an abstraction of a levelDB with prefix
type LevelDbStorage struct {
	ldb    *leveldb.DB
	prefix []byte
}

// NewLevelDbStorage returns a level db (new or existing) giving a path
func NewLevelDbStorage(path string, errorIfMissing bool) (*LevelDbStorage, error) {
	o := &opt.Options{
		ErrorIfMissing: errorIfMissing,
	}
	ldb, err := leveldb.OpenFile(path, o)
	if err != nil {
		return nil, err
	}
	return &LevelDbStorage{ldb, []byte{}}, nil
}

type storageInfo struct {
	KeyCount int
}

// Count returns the number of elements of the database
func (l *LevelDbStorage) Count() int {
	keycount := 0
	db := l.ldb
	iter := db.NewIterator(util.BytesPrefix(l.prefix), nil)
	for iter.Next() {
		keycount++
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		log.Panic(err)
	}
	return keycount
}

// Info returns some basic info regarding the database
func (l *LevelDbStorage) Info() string {
	keycount := 0
	db := l.ldb
	iter := db.NewIterator(util.BytesPrefix(l.prefix), nil)
	for iter.Next() {
		keycount++
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		return err.Error()
	}
	json, _ := json.MarshalIndent(
		storageInfo{keycount},
		"", "  ",
	)
	return string(json)
}

// WithPrefix returns a levelDB with an appended new prefix
func (l *LevelDbStorage) WithPrefix(prefix []byte) *LevelDbStorage {
	return &LevelDbStorage{l.ldb, append(l.prefix, prefix...)}
}

// Get returns the value of a given key
func (l *LevelDbStorage) Get(key []byte) ([]byte, error) {
	v, err := l.ldb.Get(append(l.prefix, key[:]...), nil)
	if err != nil {
		return nil, err
	}
	return v, err
}

// Put updates the value of a given key
func (l *LevelDbStorage) Put(key []byte, value []byte) error {
	err := l.ldb.Put(append(l.prefix, key[:]...), value, nil)
	if err != nil {
		return err
	}
	return nil
}

// Delete deletes a db entry
func (l *LevelDbStorage) Delete(key []byte) error {
	err := l.ldb.Delete(append(l.prefix, key[:]...), nil)
	if err != nil {
		return err
	}
	return nil
}

// Close closes a database instance
func (l *LevelDbStorage) Close() {
	if err := l.ldb.Close(); err != nil {
		log.Panic(err)
	}
}

// Iter returns an iterator over the database
func (l *LevelDbStorage) Iter() iterator.Iterator {
	db := l.ldb
	i := db.NewIterator(util.BytesPrefix(l.prefix), nil)
	return i
}

// LevelDB returns a pointer to the DB
func (l *LevelDbStorage) LevelDB() *leveldb.DB {
	return l.ldb
}
