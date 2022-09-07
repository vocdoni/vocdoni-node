// Package census provides the census management operation
package census

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
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

// CurrentCensusVersion history:
// - 0: First Version, Graviton based with a separate database for each census.
// - 1: Arbo + PebbleDB/BadgerDB based, single database with all censuses
// separated by Database prefixes (using Namespace.Name).
const CurrentCensusVersion = 1

// Namespace is composed by a list of keys which are capable to execute private operations
// on the namespace.
type Namespace struct {
	Type models.Census_Type `json:"type"`
	Name string             `json:"name"`
	Keys []string           `json:"keys"`
	// Version of the census in this Namespace.  If the version doesn't
	// correspond to the CurrentCensusVersion, this census is outdated and
	// will not be used.
	Version int `json:"version"`
}

// Manager is the type representing the census manager component
type Manager struct {
	StorageDir string      // Root storage data dir for LocalStorage
	db         db.Database // Database where all census trees are stored under different prefixes
	AuthWindow int32       // Time window (seconds) in which TimeStamp will be accepted if auth enabled
	Census     Namespaces  // Available namespaces

	// TODO(mvdan): should we protect Census with the mutex too?
	TreesMu sync.RWMutex
	Trees   map[string]*censustree.Tree // MkTrees map of merkle trees indexed by censusId

	RemoteStorage data.Storage // e.g. IPFS

	importQueue     chan censusImport
	queueSize       int32
	failedQueueLock sync.RWMutex
	failedQueue     map[string]string
	compressor
	cancel         context.CancelFunc
	wgQueueDaemons sync.WaitGroup
}

// Data helps satisfy an ethevents interface.
func (m *Manager) Data() data.Storage { return m.RemoteStorage }

// DBBatched wraps db.Database turning all requested db.WriteTx
// into db.Batch so that commits happen automatically when the Tx becomes too
// big.
type DBBatched struct {
	db.Database
}

// WriteTx returns a db.Batch that will commit automatically with the Tx
// becomes too big.
func (d *DBBatched) WriteTx() db.WriteTx {
	return db.NewBatch(d.Database)
}

// Start creates a new census manager.
// A constructor function for the interface censustree.Tree must be provided.
func (m *Manager) Start(dbType, storageDir, rootAuthPubKey string) (err error) {
	nsConfig := filepath.Join(storageDir, "namespaces.json")
	m.StorageDir = storageDir
	m.Trees = make(map[string]*censustree.Tree)
	m.failedQueue = make(map[string]string)
	// add a bit of buffering, to try to keep AddToImportQueue non-blocking.
	m.importQueue = make(chan censusImport, importQueueBuffer)
	m.AuthWindow = 10
	m.compressor = newCompressor()

	dbDir := filepath.Join(m.StorageDir, fmt.Sprintf("v%v", CurrentCensusVersion))
	database, err := metadb.New(dbType, dbDir)
	if err != nil {
		return err
	}

	m.db = &DBBatched{database}
	defer func() {
		if err != nil {
			database.Close()
		}
	}()

	// Start daemon for importing remote census
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	log.Infof("starting %d import queue routines", ImportQueueRoutines)
	for i := 0; i < ImportQueueRoutines; i++ {
		m.wgQueueDaemons.Add(1)
		go m.importQueueDaemon(ctx)
	}
	m.wgQueueDaemons.Add(1)
	go m.importFailedQueueDaemon(ctx)

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
		if err := os.WriteFile(nsConfig, []byte(""), 0o600); err != nil {
			return err
		}
		return m.save()
	}

	jsonBytes, err := os.ReadFile(nsConfig)
	if err != nil {
		return err
	}
	var namespaces Namespaces
	if err := json.Unmarshal(jsonBytes, &namespaces); err != nil {
		log.Warn("could not unmarshal json config file, probably empty. Skipping")
		return nil
	}

	// Copy censuses of the expected version into m.Census and remove old ones
	m.Census.RootKey = namespaces.RootKey
	cleanOldCensues := false
	timestamp := time.Now()
	for _, v := range namespaces.Namespaces {
		switch v.Version {
		case 0:
			// Version 0 are graviton censuses.  These censuses are
			// no longer supported and belong to legacy processes.
			cleanOldCensues = true
			outdatedDir := filepath.Join(storageDir,
				fmt.Sprintf("outdated.%v",
					timestamp.Format(time.RFC3339)), "v0")
			if err := os.MkdirAll(outdatedDir, 0o700); err != nil {
				return err
			}
			log.Infof("Moving outdated census %v to %v", v.Name, outdatedDir)
			dir := strings.Split(v.Name, string(os.PathSeparator))[0]
			src := filepath.Join(storageDir, dir)
			dst := filepath.Join(outdatedDir, dir)
			if err := os.Rename(src, dst); err != nil {
				log.Warnf("Unable to move outdated census from %v to %v: %v",
					src, dst, err)
			}
		case CurrentCensusVersion:
			m.Census.Namespaces = append(m.Census.Namespaces, v)
		default:
			return fmt.Errorf("unexpected census version in %v: %v",
				nsConfig, v.Version)
		}
	}
	if cleanOldCensues {
		// Make a backup of the old namespaces.json
		nsConfigBackup := filepath.Join(storageDir,
			fmt.Sprintf("namespaces.%v.json", timestamp.Format(time.RFC3339)))
		if err := os.WriteFile(nsConfigBackup, jsonBytes, 0o600); err != nil {
			return err
		}
		if err := m.save(); err != nil {
			return err
		}
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

// Stop terminates the Manager goroutines and closes the database.  The
// Manager can't be used after Stop is called.
func (m *Manager) Stop() error {
	log.Infof("stopping import queue routines")
	m.cancel()
	m.wgQueueDaemons.Wait()
	return m.db.Close()
}

// LoadTree opens the database containing the merkle tree or returns nil if already loaded
// Not thread safe
func (m *Manager) LoadTree(name string, censusType models.Census_Type) (*censustree.Tree, error) {
	if _, exist := m.Trees[name]; exist {
		return m.Trees[name], nil
	}

	censusTree, err := censustree.New(censustree.Options{Name: name, ParentDB: m.db,
		MaxLevels: 256, CensusType: censusType})
	if err != nil {
		return nil, err
	}

	log.Infof("load merkle tree %s", name)
	m.Trees[name] = censusTree
	m.Trees[name].Publish()
	return censusTree, nil
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
func (m *Manager) AddNamespace(name string, censusType models.Census_Type, authPubKeys []string) (*censustree.Tree, error) {
	m.TreesMu.Lock()
	defer m.TreesMu.Unlock()
	if m.Exists(name) {
		return nil, ErrNamespaceExist
	}
	censusTree, err := censustree.New(censustree.Options{Name: name, ParentDB: m.db,
		MaxLevels: 256, CensusType: censusType})
	if err != nil {
		return nil, err
	}
	m.Trees[name] = censusTree
	m.Census.Namespaces = append(m.Census.Namespaces, Namespace{
		Type:    censusType,
		Name:    name,
		Keys:    authPubKeys,
		Version: CurrentCensusVersion,
	})
	return censusTree, m.save()
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
	if err := os.MkdirAll(m.StorageDir, 0o750); err != nil {
		return err
	}
	nsConfig := filepath.Join(m.StorageDir, "namespaces.json")
	data, err := json.Marshal(m.Census)
	if err != nil {
		return fmt.Errorf("error marshaling census file: %w", err)
	}
	log.Warnf("NSCONFIG: %s", nsConfig)
	if err := os.WriteFile(nsConfig, data, 0o600); err != nil {
		log.Warnf("ERROR: %v", err)
	}

	return nil
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

// keyCensusKeyIndex returns the Manager.db key where key index is stored.
func keyCensusKeyIndex(censusID string, key []byte) []byte {
	return []byte(path.Join("key", censusID, string(key)))
}

func keyCensusLen(censusID string) []byte {
	return []byte(path.Join(censusID, "len"))
}

// fillKeyToIndex fills a mapping in m.db under the path "`key`/censusID" of
// value -> key.  This function should be called when importing a Poseidon
// Census tree as we expect it to use a sequential index for the key, and a
// public key for the value.  Having this mapping will allow us to resolve the
// index given a public key.
func (m *Manager) fillKeyToIndex(censusID string, t *censustree.Tree) error {
	type IndexKey struct {
		IndexLE []byte
		Key     []byte
	}
	indexKeys := make([]IndexKey, 0)
	t.IterateLeaves(func(indexLE, key []byte) bool {
		indexKeys = append(indexKeys, IndexKey{IndexLE: indexLE, Key: key})
		return false
	})
	tx := m.db.WriteTx()
	defer tx.Discard()
	for _, indexKey := range indexKeys {
		if err := tx.Set(keyCensusKeyIndex(censusID, indexKey.Key),
			indexKey.IndexLE); err != nil {
			return fmt.Errorf("error storing census key index by key: %w", err)
		}
	}
	censusLenLE := [8]byte{}
	binary.LittleEndian.PutUint64(censusLenLE[:], uint64(len(indexKeys)))
	if err := tx.Set(keyCensusLen(censusID), censusLenLE[:]); err != nil {
		return err
	}
	return tx.Commit()
}

// KeyToIndex resolves the index of the key for the Poseidon census identified
// by `censusID`.  The returned index is 64 bit little endian encoded and can
// be used to query the tree.
func (m *Manager) KeyToIndex(censusID string, key []byte) ([]byte, error) {
	// Sanity check
	m.TreesMu.Lock()
	tr, ok := m.Trees[censusID]
	m.TreesMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("census tree not loaded")
	}
	if tr.Type() != models.Census_ARBO_POSEIDON {
		return nil, fmt.Errorf("expected Tree type to be Census_ARBO_POSEIDON, found %v",
			tr.Type())
	}

	tx := m.db.ReadTx()
	defer tx.Discard()
	return tx.Get(keyCensusKeyIndex(censusID, key))
}
