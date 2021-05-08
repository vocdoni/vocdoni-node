// Package data provides an abstraction layer for distributed data storage providers (currently only IPFS)
package data

import (
	"context"
	"errors"

	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/types"
)

type Storage interface {
	Init(d *types.DataStore) error
	Publish(ctx context.Context, o []byte) (string, error)
	Retrieve(ctx context.Context, id string, maxSize int64) ([]byte, error)
	Pin(ctx context.Context, path string) error
	Unpin(ctx context.Context, path string) error
	ListPins(ctx context.Context) (map[string]string, error)
	URIprefix() string
	Stats(ctx context.Context) (string, error)
	CollectMetrics(ctx context.Context, ma *metrics.Agent) error

	// TODO(mvdan): Temporary until we rethink Init/Start/etc.
	Stop() error
}

type StorageID int

const (
	IPFS StorageID = iota + 1
	BZZ
)

func StorageIDFromString(i string) StorageID {
	switch i {
	case "IPFS":
		return IPFS
	case "BZZ":
		return BZZ
	default:
		return -1
	}
}

func IPFSNewConfig(path string) *types.DataStore {
	datastore := new(types.DataStore)
	datastore.Datadir = path
	return datastore
}

// TODO(mvdan): This is really a Start, not an Init. Rethink this.

func Init(t StorageID, d *types.DataStore) (Storage, error) {
	switch t {
	case IPFS:
		s := new(IPFSHandle)
		err := s.Init(d)
		return s, err
	default:
		return nil, errors.New("bad storage type or DataStore specification")
	}
}
