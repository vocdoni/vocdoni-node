package db

import (
	"fmt"
	"io"
)

// StorageType defines the key value storage used as a backend database for the state
type StorageType int

const (
	// StoragePebble defines pebbledb key value
	StoragePebble StorageType = iota
	// StorageBadgerv2 defines badgerdb v2 key value
	StorageBadgerv2
	// StorageBadgerv3 defines badgerdb v3 key value
	StorageBadgerv3
)

// ErrKeysNotFound is used to indicate that a key does not exist in the db.
var ErrKeyNotFound = fmt.Errorf("key not found")

// ErrTxnTooBig is used to indicate that a WriteTx is too big and can't include
// more writes.
var ErrTxnTooBig = fmt.Errorf("txn too big")

// Database wraps all database operations. All methods are safe for concurrent
// use.
type Database interface {
	io.Closer

	// ReadTx returns a ReadTx
	ReadTx() ReadTx
	// WriteTx returns a WriteTx
	WriteTx() WriteTx
	// Iterate iterates over the key-value pairs executing the given
	// callback function for each pair. If a prefix is given, it will only
	// iterate over the key-values with the prefix. The callback returns a
	// boolean which is used to indicate to the iteration if to stop or to
	// continue iterating. While the callback returns `true`, the iteration
	// will keep continuing, and if the callback returned boolean is
	// `false`, the iteration will stop.
	Iterate(prefix []byte, callback func(key, value []byte) bool) error
}

type ReadTx interface {
	// Get retreives the value for the given key. If the key does not
	// exist, returns the error ErrKeyNotFound
	Get(key []byte) ([]byte, error)
	// Discard discards the transaction. This method can be called always,
	// even if previously the Tx has been Commited (for the WriteTx case).
	// So it's a good practice to `defer tx.Discard()` just after creating
	// the tx.
	Discard()
}

type WriteTx interface {
	ReadTx

	// Set adds a key-value pair. If the key already exists, its value is
	// updated.
	Set(key []byte, value []byte) error
	// Delete deletes a key and its value.
	Delete(key []byte) error
	// Commit commits the transaction into the db
	Commit() error
}
