// Package census provides the census management operation
package census

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.vocdoni.io/dvote/censustree"
	censustreefactory "go.vocdoni.io/dvote/censustree/factory"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
)

// ErrNamespaceExist is the error returned when trying to add a namespace
// that already exist
var ErrNamespaceExist = errors.New("namespace already exists")

const (
	// ImportQueueRoutines is the number of parallel routines processing the
	// remote census download queue
	ImportQueueRoutines = 10

	// ImportRetrieveTimeout the maximum duration the import queue will wait
	// for retreiving a remote census
	ImportRetrieveTimeout = 1 * time.Minute

	importQueueBuffer = 32
)

// Namespaces contains a list of existing census namespaces and the root public key
type Namespaces struct {
	RootKey    string      `json:"rootKey"` // Public key allowed to created new census
	Namespaces []Namespace `json:"namespaces"`
}

// Namespace is composed by a list of keys which are capable to execute private operations
// on the namespace.
type Namespace struct {
	Type models.Census_Type `json:"type"`
	Name string             `json:"name"`
	Keys []string           `json:"keys"`
}

// Manager is the type representing the census manager component
type Manager struct {
	StorageDir string     // Root storage data dir for LocalStorage
	AuthWindow int32      // Time window (seconds) in which TimeStamp will be accepted if auth enabled
	Census     Namespaces // Available namespaces

	// TODO(mvdan): should we protect Census with the mutex too?
	TreesMu sync.RWMutex
	Trees   map[string]censustree.Tree // MkTrees map of merkle trees indexed by censusId

	RemoteStorage data.Storage // e.g. IPFS

	importQueue     chan censusImport
	queueSize       int32
	failedQueueLock sync.RWMutex
	failedQueue     map[string]string
	compressor
}

// Data helps satisfy an ethevents interface.
func (m *Manager) Data() data.Storage { return m.RemoteStorage }

// Init creates a new census manager.
// A constructor function for the interface censustree.Tree must be provided.
func (m *Manager) Init(storageDir, rootAuthPubKey string) error {
	nsConfig := fmt.Sprintf("%s/namespaces.json", storageDir)
	m.StorageDir = storageDir
	m.Trees = make(map[string]censustree.Tree)
	m.failedQueue = make(map[string]string)
	// add a bit of buffering, to try to keep AddToImportQueue non-blocking.
	m.importQueue = make(chan censusImport, importQueueBuffer)
	m.AuthWindow = 10
	m.compressor = newCompressor()

	// Start daemon for importing remote census
	log.Infof("starting %d import queue routines", ImportQueueRoutines)
	for i := 0; i < ImportQueueRoutines; i++ {
		go m.importQueueDaemon()
	}
	go m.importFailedQueueDaemon()

	log.Infof("loading namespaces and keys from %s", nsConfig)
	if _, err := os.Stat(nsConfig); os.IsNotExist(err) {
		log.Info("creating new config file")
		var cns Namespaces
		if len(rootAuthPubKey) < ethereum.PubKeyLengthBytes*2 {
			// log.Warn("no root key provided or invalid, anyone will be able to create new census")
		} else {
			cns.RootKey = rootAuthPubKey
		}
		m.Census = cns
		if err := os.WriteFile(filepath.Clean(nsConfig), []byte(""), 0o600); err != nil {
			return err
		}
		return m.save()
	}

	jsonBytes, err := os.ReadFile(filepath.Clean(nsConfig))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonBytes, &m.Census); err != nil {
		log.Warn("could not unmarshal json config file, probably empty. Skipping")
		return nil
	}
	if len(rootAuthPubKey) >= ethereum.PubKeyLengthBytes*2 {
		log.Infof("updating root key to %s", rootAuthPubKey)
		m.Census.RootKey = rootAuthPubKey
	} else if rootAuthPubKey != "" {
		log.Infof("current root key %s", rootAuthPubKey)
	}
	for _, v := range m.Census.Namespaces {
		if _, err := m.LoadTree(v.Name, v.Type); err != nil {
			log.Warnf("census %s cannot be loaded: (%v)", v.Name, err)
		}
	}
	return nil
}

// LoadTree opens the database containing the merkle tree or returns nil if already loaded
// Not thread safe
func (m *Manager) LoadTree(name string, treeType models.Census_Type) (censustree.Tree, error) {
	if _, exist := m.Trees[name]; exist {
		return m.Trees[name], nil
	}

	// ensure backwards compatibility to CensusTrees created before the
	// Protobuf CensusTypes implementation. When TreeType is set to 0, uses
	// the 'default' tree type
	if treeType == models.Census_UNKNOWN {
		treeType = censusDefaultType
	}

	tr, err := censustreefactory.NewCensusTree(treeType, name, m.StorageDir)
	if err != nil {
		return nil, err
	}
	log.Infof("load merkle tree %s", name)
	m.Trees[name] = tr
	m.Trees[name].Publish()
	return tr, nil
}

// UnloadTree closes the database containing the merkle tree
// Not thread safe
func (m *Manager) UnloadTree(name string) {
	log.Debugf("unload merkle tree %s", name)
	delete(m.Trees, name)
}

// Exists returns true if a given census exists on disk
// While Exists() means there is a tree database with such name,
//  Load() reads the tree from disk and create the required memory structure in order to use it
// Not thread safe, Mutex must be controlled on the calling function
func (m *Manager) Exists(name string) bool {
	for _, ns := range m.Census.Namespaces {
		if name == ns.Name {
			return true
		}
	}
	return false
}

// AddNamespace adds a new merkletree identified by a censusId (name), and
// returns the new tree.
func (m *Manager) AddNamespace(name string, treeType models.Census_Type, authPubKeys []string) (censustree.Tree, error) {
	m.TreesMu.Lock()
	defer m.TreesMu.Unlock()
	if m.Exists(name) {
		return nil, ErrNamespaceExist
	}

	/*
		Hotfix to census publish/import bug
			The census publish method had a bug that didn't add the census treeType to the censusDump published to ipfs.
			This caused the type to be `0`, which is invalid. Gateways importing the census, then, would error.
			The bug was fixed here: https://github.com/vocdoni/vocdoni-node/pull/259
			Unfortunately this only fixes census published in the future. For existing census that already don't
			 have the treeType set properly, gateways still cannot import them.
			This clause fixes the bug for already published census by assuming that an invalid treeType of `0`
			 should really be a Graviton tree (all current census). Gateways running this hotfix should be
			 re-started and re-synced, and they should then be able to correctly import the census.
			This hotfix is temporary and should not be included in future versions of the code once the
			 Vochain is reset and the broken census are no longer used. Until then, this fix is needed
			 whenever a gateway is syncing.
	*/
	if treeType == 0 {
		treeType = censusDefaultType
	}

	tr, err := censustreefactory.NewCensusTree(treeType, name, m.StorageDir)
	if err != nil {
		return nil, err
	}
	m.Trees[name] = tr
	m.Census.Namespaces = append(m.Census.Namespaces, Namespace{
		Type: treeType,
		Name: name,
		Keys: authPubKeys,
	})
	return tr, m.save()
}

// DelNamespace removes a merkletree namespace
func (m *Manager) DelNamespace(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("no valid namespace provided")
	}
	m.TreesMu.Lock()
	defer m.TreesMu.Unlock()
	if !m.Exists(name) {
		return nil
	}
	// TODO(mvdan): re-implement with the prefixed database
	// if err := os.RemoveAll(m.StorageDir + "/" + name); err != nil {
	// 	return fmt.Errorf("cannot remove census: (%s)", err)
	// }

	for i, ns := range m.Census.Namespaces {
		if ns.Name == name {
			m.Census.Namespaces = m.Census.Namespaces[:i+
				copy(m.Census.Namespaces[i:], m.Census.Namespaces[i+1:])]
			break
		}
	}
	return m.save()
}

func (m *Manager) save() error {
	log.Debug("saving namespaces")
	nsConfig := fmt.Sprintf("%s/namespaces.json", m.StorageDir)
	data, err := json.Marshal(m.Census)
	if err != nil {
		return err
	}
	return os.WriteFile(nsConfig, data, 0o600)
}

// Count returns the number of local created, external imported and loaded/active census
func (m *Manager) Count() (local, imported, loaded int) {
	for _, n := range m.Census.Namespaces {
		if strings.Contains(n.Name, "/") {
			local++
		} else {
			imported++
		}
	}
	loaded = len(m.Trees)
	return
}
