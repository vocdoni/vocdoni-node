// Package data provides an abstraction layer for distributed data storage providers (currently only IPFS)
package data

import (
	"errors"
	"os"

	"gitlab.com/vocdoni/go-dvote/types"
)

type Storage interface {
	Init(d *types.DataStore) error
	Publish(o []byte) (string, error)
	Retrieve(id string) ([]byte, error)
	Pin(path string) error
	Unpin(path string) error
	ListPins() (map[string]string, error)
	URIprefix() string
	Stats() (string, error)
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
			return nil, errors.New("Cannot get $HOME")
		}
		s := new(IPFSHandle)
		defaultDataStore := new(types.DataStore)
		defaultDataStore.Datadir = home + "/.ipfs/"
		err = s.Init(defaultDataStore)
		return s, err
	case BZZ:
		s := new(BZZHandle)
		defaultDataStore := new(types.DataStore)
		defaultDataStore.Datadir = "this_is_still_ignored"
		err := s.Init(defaultDataStore)
		return s, err
	default:
		return nil, errors.New("Bad storage type specification")
	}
}

func Init(t StorageID, d *types.DataStore) (Storage, error) {
	switch t {
	case IPFS:
		s := new(IPFSHandle)
		err := s.Init(d)

		return s, err
	case BZZ:
		s := new(BZZHandle)
		err := s.Init(d)
		return s, err
	default:
		return nil, errors.New("Bad storage type or DataStore specification")
	}
}
