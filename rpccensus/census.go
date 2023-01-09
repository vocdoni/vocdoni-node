// Package rpccensus provides the census management operation
package rpccensus

import (
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data"
)

// Manager is the type representing the census manager component
type Manager struct {
	cdb           *censusdb.CensusDB
	RemoteStorage data.Storage // e.g. IPFS
}

// NewCensusManager creates a new instance of the RPC census manager
func NewCensusManager(cdb *censusdb.CensusDB, storage data.Storage) *Manager {
	return &Manager{
		cdb:           cdb,
		RemoteStorage: storage,
	}
}
