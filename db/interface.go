package db

import (
	"fmt"
	"io"
)

// TypePebble defines the type of db that uses PebbleDB
const (
	TypePebble  = "pebble"
	TypeLevelDB = "leveldb"
	TypeMongo   = "mongodb"
)

// ErrKeyNotFound is used to indicate that a key does not exist in the db.
var ErrKeyNotFound = fmt.Errorf("key not found")

// ErrTxnTooBig is used to indicate that a WriteTx is too big and can't include
// more writes.
var ErrTxnTooBig = fmt.Errorf("txn too big")

// ErrConflict is returned when a transaction conflicts with another transaction. This can
// happen if the read rows had been updated concurrently by another transaction.
var ErrConflict = fmt.Errorf("txn conflict")

// Options defines generic parameters for creating a new Database.
type Options struct {
	Path string
}

// Database wraps all database operations. All methods are safe for concurrent
// use.
type Database interface {
	io.Closer

	Reader

	// WriteTx creates a new write transaction.
	WriteTx() WriteTx

	// Compact compacts the underlying storage.
	Compact() error
}

// Reader contains the read-only database operations.
type Reader interface {
	// Get retrieves the value for the given key. If the key does not
	// exist, returns the error ErrKeyNotFound
	Get(key []byte) ([]byte, error)

	// Iterate calls callback with all key-value pairs in the database whose key
	// starts with prefix. The calls are ordered lexicographically by key.
	//
	// The iteration is stopped early when the callback function returns false.
	//
	// It is not safe to use the key or value slices after the callback returns.
	// To use the values for longer, make a copy.
	Iterate(prefix []byte, callback func(key, value []byte) bool) error
}

type WriteTx interface {
	Reader

	// Set adds a key-value pair. If the key already exists, its value is
	// updated.
	Set(key []byte, value []byte) error
	// Delete deletes a key and its value.
	Delete(key []byte) error
	// Apply applies the value-passed WriteTx into the given WriteTx,
	// copying the key-values from the original WriteTx into the one from
	// which the method is called.
	// TODO Review once generics are ready: WriteTx is an interface, so
	// Apply internally needs type assertions, revisit this once generics
	// are ready.
	Apply(WriteTx) error
	// Commit commits the transaction into the db.
	// Calling Commit more than once, or after Discard, is an error.
	Commit() error
	// Discard releases the transaction's resources as they don't need to be committed.
	// This method can be safely called after any previous Commit or Discard call,
	// for the sake of allowing deferred Discard calls.
	Discard()
}

// UnwrapWriteTx unwraps (if possible) the WriteTx using Unwrap method
func UnwrapWriteTx(tx WriteTx) WriteTx {
	for {
		wtx, ok := tx.(interface{ Unwrap() WriteTx })
		if !ok {
			return tx
		}
		tx = wtx.Unwrap()
	}
}
