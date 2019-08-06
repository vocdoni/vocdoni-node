package types

import (
	dbm "github.com/tendermint/tm-db"
)

// Store is a generic KV-DB inteface
type Store struct {
	db dbm.DB
}

// NewStore creates a new Store given a KV-DB interface
func NewStore(db dbm.DB) *Store {
	return &Store{
		db: db,
	}
}
