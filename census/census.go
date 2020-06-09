// Package census provides the census management operation
package census

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	iden3db "github.com/iden3/go-iden3-core/db"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/db"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/tree"
)

// ErrNamespaceExist is the error returned when trying to add a namespace that already exist
var ErrNamespaceExist = errors.New("namespace already exists")

// ImportQueueRoutines is the number of paralel routines processing the remote census download queue
const ImportQueueRoutines = 10

// ImportRetrieveTimeout the maximum duration the import queue will wait for retreiving a remote census
const ImportRetrieveTimeout = 1 * time.Minute

type Namespaces struct {
	RootKey    string      `json:"rootKey"` // Public key allowed to created new census
	Namespaces []Namespace `json:"namespaces"`
}

type Namespace struct {
	Name string   `json:"name"`
	Keys []string `json:"keys"`
}

// Manager is the type representing the census manager component
type Manager struct {
	StorageDir string     // Root storage data dir for LocalStorage
	AuthWindow int32      // Time window (seconds) in which TimeStamp will be accepted if auth enabled
	Census     Namespaces // Available namespaces

	// TODO(mvdan): should we protect Census with the mutex too?
	TreesMu sync.RWMutex
	Trees   map[string]*tree.Tree // MkTrees map of merkle trees indexed by censusId

	RemoteStorage data.Storage    // e.g. IPFS
	LocalStorage  iden3db.Storage // e.g. Badger

	importQueue     chan censusImport
	queueSize       int32
	failedQueueLock sync.RWMutex
	failedQueue     map[string]string
	compressor
}

// Data helps satisfy an ethevents interface.
func (m *Manager) Data() data.Storage { return m.RemoteStorage }

// Init creates a new census manager
func (m *Manager) Init(storageDir, rootKey string) error {
	nsConfig := fmt.Sprintf("%s/namespaces.json", storageDir)
	m.StorageDir = storageDir
	m.Trees = make(map[string]*tree.Tree)
	m.failedQueue = make(map[string]string)

	var err error
	m.LocalStorage, err = db.NewIden3Storage(storageDir)
	if err != nil {
		return err
	}

	// add a bit of buffering, to try to keep AddToImportQueue non-blocking.
	m.importQueue = make(chan censusImport, 32)
	m.AuthWindow = 10
	m.compressor = newCompressor()

	// Start daemon for importing remote census
	log.Infof("starting %d import queue routines", ImportQueueRoutines)
	for i := 0; i < ImportQueueRoutines; i++ {
		go m.importQueueDaemon()
	}

	log.Infof("loading namespaces and keys from %s", nsConfig)
	if _, err := os.Stat(nsConfig); os.IsNotExist(err) {
		log.Info("creating new config file")
		var cns Namespaces
		if len(rootKey) < ethereum.PubKeyLength {
			// log.Warn("no root key provided or invalid, anyone will be able to create new census")
		} else {
			cns.RootKey = rootKey
		}
		m.Census = cns
		ioutil.WriteFile(nsConfig, []byte(""), 0644)
		err = m.save()
		return err
	}

	jsonBytes, err := ioutil.ReadFile(nsConfig)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonBytes, &m.Census); err != nil {
		log.Warn("could not unmarshal json config file, probably empty. Skipping")
		return nil
	}
	if len(rootKey) >= ethereum.PubKeyLength {
		log.Infof("updating root key to %s", rootKey)
		m.Census.RootKey = rootKey
	} else {
		if rootKey != "" {
			log.Infof("current root key %s", rootKey)
		}
	}
	return nil
}

// LoadTree opens the database containing the merkle tree or returns nil if already loaded
// Not thread safe
func (m *Manager) LoadTree(name string) (*tree.Tree, error) {
	if _, exist := m.Trees[name]; exist {
		return m.Trees[name], nil
	}
	tr, err := tree.NewTree(m.LocalStorage.WithPrefix([]byte(name)))
	if err != nil {
		return nil, err
	}
	log.Infof("load merkle tree %s", name)
	m.Trees[name] = tr
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
func (m *Manager) AddNamespace(name string, pubKeys []string) (*tree.Tree, error) {
	m.TreesMu.Lock()
	defer m.TreesMu.Unlock()
	if m.Exists(name) {
		return nil, ErrNamespaceExist
	}
	tr, err := tree.NewTree(m.LocalStorage.WithPrefix([]byte(name)))
	if err != nil {
		return nil, err
	}
	m.Trees[name] = tr
	m.Census.Namespaces = append(m.Census.Namespaces, Namespace{
		Name: name,
		Keys: pubKeys,
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
	m.UnloadTree(name)
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
	return ioutil.WriteFile(nsConfig, data, 0644)
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
