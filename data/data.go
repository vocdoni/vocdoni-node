package data

import (
	"errors"
)

type Storage interface {
	Init() 
	Publish(o []byte) string
	Retrieve(id string) []byte
}

type StorageID int

const (
	IPFS StorageID = iota + 1
	BZZ
)

func StorageIDFromString(i string) StorageID {
	switch i {
	case "IPFS" :
		return IPFS
	case "BZZ":
		return BZZ
	default:
		return -1
	}
}

func Init(t StorageID) (Storage, error) {
	switch t {
	case IPFS:
		s := new(IPFSHandle)
		s.Init()
		return s, nil
	case BZZ:
		s := new(BZZHandle)
		s.Init()
		return s, nil
	default:
		return nil, errors.New("Bad storage type specification")
	}
}
