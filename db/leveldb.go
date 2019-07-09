package db

// modified from https://github.com/iden3/go-iden3/blob/master/db/leveldb.go

import (
	"encoding/json"

	"github.com/syndtr/goleveldb/leveldb"
//	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/syndtr/goleveldb/leveldb/iterator"

	"gitlab.com/vocdoni/go-dvote/log"
)


type LevelDbStorage struct {
	ldb    *leveldb.DB
	prefix []byte
}

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

func (l *LevelDbStorage) WithPrefix(prefix []byte) *LevelDbStorage {
	return &LevelDbStorage{l.ldb, append(l.prefix, prefix...)}
}

func (l *LevelDbStorage) Get(key []byte) ([]byte, error) {
	v, err := l.ldb.Get(append(l.prefix, key[:]...), nil)
	if err != nil {
		return nil, err
	}
	return v, err
}

func (l *LevelDbStorage) Put(key []byte, value []byte) error {
	err := l.ldb.Put(append(l.prefix, key[:]...), value, nil)
	if err != nil {
		return err
	}
	return nil
}

func (l *LevelDbStorage) Delete(key []byte) error {
	err := l.ldb.Delete(append(l.prefix, key[:]...), nil)
	if err != nil {
		return err
	}
	return nil
}

func (l *LevelDbStorage) Close() {
	if err := l.ldb.Close(); err != nil {
		log.Panic(err)
	}
}

func (l *LevelDbStorage) Iter() iterator.Iterator {
	db := l.ldb
	i := db.NewIterator(util.BytesPrefix(l.prefix), nil)
	return i
}

func (l *LevelDbStorage) LevelDB() *leveldb.DB {
	return l.ldb
}

