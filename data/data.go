// Package data provides an abstraction layer for distributed data storage providers (currently only IPFS)
package data

import (
	"context"
	"errors"
	"os"

	"gitlab.com/vocdoni/go-dvote/types"
)

type Storage interface {
	Init(d *types.DataStore) error
	Publish(ctx context.Context, o []byte) (string, error)
	Retrieve(ctx context.Context, id string) ([]byte, error)
	Pin(ctx context.Context, path string) error
	Unpin(ctx context.Context, path string) error
	ListPins(ctx context.Context) (map[string]string, error)
	URIprefix() string
	Stats(ctx context.Context) (string, error)
}

type StorageConfig interface {
	Type() StorageID
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

func InitDefault(t StorageID) (Storage, error) {
	switch t {
	case IPFS:
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.New("cannot get $HOME")
		}
		s := new(IPFSHandle)
		defaultDataStore := new(types.DataStore)
		defaultDataStore.Datadir = home + "/.ipfs/"
		err = s.Init(defaultDataStore)
		return s, err
	default:
		return nil, errors.New("bad storage type specification")
	}
}

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
