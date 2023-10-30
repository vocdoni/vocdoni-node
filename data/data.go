// Package data provides an abstraction layer for distributed data storage providers (currently only IPFS)
package data

import (
	"context"
	"fmt"
	"io"

	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/types"
)

// Storage is the interface that wraps the basic methods for a distributed data storage provider.
type Storage interface {
	Init(d *types.DataStore) error
	Publish(ctx context.Context, data []byte) (string, error)
	PublishReader(ctx context.Context, data io.Reader) (string, error)
	Retrieve(ctx context.Context, id string, maxSize int64) ([]byte, error)
	RetrieveDir(ctx context.Context, id string, maxSize int64) (map[string][]byte, error)
	Pin(ctx context.Context, path string) error
	Unpin(ctx context.Context, path string) error
	ListPins(ctx context.Context) (map[string]string, error)
	URIprefix() string
	Stats() map[string]any
	Stop() error
}

// StorageID is the type for the different storage providers.
// Currently only IPFS is supported.
type StorageID int

const (
	// IPFS is the InterPlanetary File System.
	IPFS StorageID = iota + 1
)

// StorageIDFromString returns the Storage identifier from a string.
func StorageIDFromString(i string) StorageID {
	switch i {
	case "IPFS":
		return IPFS
	default:
		return -1
	}
}

// IPFSNewConfig returns a new DataStore configuration for IPFS.
func IPFSNewConfig(path string) *types.DataStore {
	datastore := new(types.DataStore)
	datastore.Datadir = path
	return datastore
}

// Init returns a new Storage instance of type `t`.
func Init(t StorageID, d *types.DataStore) (Storage, error) {
	switch t {
	case IPFS:
		s := new(ipfs.Handler)
		err := s.Init(d)
		return s, err
	default:
		return nil, fmt.Errorf("bad storage type or DataStore specification")
	}
}
