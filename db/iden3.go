package db

import (
	iden3db "github.com/iden3/go-iden3-core/db"
)

// Iden3Storage is an overlay on top of BadgerDB to satisfy iden3's current
// interface.
type Iden3Storage struct {
	db Database
}

var _ iden3db.Storage = Iden3Storage{}

func NewIden3Storage(path string) (Iden3Storage, error) {
	db, err := NewBadgerDB(path)
	return Iden3Storage{db: db}, err
}

func (s Iden3Storage) NewTx() (iden3db.Tx, error) {
	return Iden3Tx{db: s.db, bc: s.db.NewBatch()}, nil
}

func (s Iden3Storage) WithPrefix(prefix []byte) iden3db.Storage {
	return Iden3Storage{db: NewTable(s.db, string(prefix))}
}

func (s Iden3Storage) Get(key []byte) ([]byte, error) {
	return s.db.Get(key)
}

func (s Iden3Storage) List(limit int) ([]iden3db.KV, error) {
	panic("unimplemented") // not needed for merkletree?
}

func (s Iden3Storage) Close() {
	if err := s.db.Close(); err != nil {
		panic(err) // since we can't return it
	}
}

func (s Iden3Storage) Info() string {
	return ""
}

func (s Iden3Storage) Iterate(fn func([]byte, []byte) (bool, error)) error {
	panic("unimplemented") // not needed for merkletree?
}

type Iden3Tx struct {
	db Database // just for the Get method
	bc Batch
}

var _ iden3db.Tx = Iden3Tx{}

func (t Iden3Tx) Get(key []byte) ([]byte, error) {
	return t.db.Get(key)
}

func (t Iden3Tx) Put(key, value []byte) {
	if err := t.bc.Put(key, value); err != nil {
		panic(err) // since we can't return it
	}
}

func (t Iden3Tx) Add(tx iden3db.Tx) {
	panic("unimplemented") // not needed for merkletree?
}

func (t Iden3Tx) Commit() error {
	return t.bc.Write()
}

func (t Iden3Tx) Close() {
	// done via Commit
}
