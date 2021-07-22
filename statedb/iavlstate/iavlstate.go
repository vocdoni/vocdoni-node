package iavlstate

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/cosmos/iavl"
	tmdb "github.com/tendermint/tm-db"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/statedb"
)

const PrefixDBCacheSize = 1024

type IavlState struct {
	dataDir     string
	dataType    statedb.StorageType
	trees       map[string]*IavlTree
	lock        sync.RWMutex
	versionTree *iavl.MutableTree   // For each tree, saves its last committed version
	storageType statedb.StorageType // mem or disk
	db          tmdb.DB
}

// check that statedb.StateDB interface is matched by IavlState
var _ statedb.StateDB = (*IavlState)(nil)

type IavlTree struct {
	tree                *iavl.MutableTree
	itree               *iavl.ImmutableTree
	isImmutable         bool
	lastCommitedVersion uint64
}

// check that statedb.StateTree interface is matched by IavlTree
var _ statedb.StateTree = (*IavlTree)(nil)

// Init initializes a iavlstate storage.
// storageType can be disk or mem, default is disk.
func (i *IavlState) Init(storagePath string, storageType statedb.StorageType) error {
	i.dataDir = storagePath
	i.dataType = storageType
	i.trees = make(map[string]*IavlTree, 32)
	var err error

	switch storageType {
	case statedb.StorageTypeDisk, "":
		i.db, err = tmdb.NewGoLevelDB("versions", storagePath)
		if err != nil {
			return err
		}
		i.storageType = statedb.StorageTypeDisk
	case statedb.StorageTypeMemory:
		i.db = tmdb.NewMemDB()
		i.storageType = statedb.StorageTypeMemory
	default:
		return fmt.Errorf("storageType %s not supported", storageType)
	}

	if i.versionTree, err = iavl.NewMutableTree(i.db, PrefixDBCacheSize); err != nil {
		return err
	}
	return i.LoadVersion(0)
}

func (i *IavlState) Version() uint64 {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return uint64(i.versionTree.Version())
}

func (i *IavlState) LoadVersion(v int64) error { // zero means last version, -1 means previous version
	i.lock.Lock()
	defer i.lock.Unlock()
	var err error

	if v != 0 {
		// get last versiontree saved version and decrease by 1 if higher than zero
		if v == -1 {
			v, err = i.versionTree.Load()
			if err != nil {
				return fmt.Errorf("cannot load last version tree state: (%s)", err)
			}
			if v > 0 {
				v--
			}
		} else if v > 0 {
			_, err = i.versionTree.LoadVersion(v)
			if err != nil {
				return fmt.Errorf("cannot load last version tree state: (%s)", err)
			}
		} else {
			return fmt.Errorf("loading versions below -1 is not supported")
		}

		for name, t := range i.trees {
			_, vb := i.versionTree.Get([]byte(name))
			vt, _ := strconv.ParseUint(string(vb), 10, 64)
			t.tree.LoadVersionForOverwriting(int64(vt))
		}

		if _, err = i.versionTree.LoadVersionForOverwriting(v); err != nil {
			return fmt.Errorf("cannot load version number %d: (%s)", v, err)
		}
	}

	if v == 0 {
		for _, t := range i.trees {
			_, err = t.tree.Load()
			if err != nil {
				return err
			}
		}
		if _, err = i.versionTree.Load(); err != nil {
			return fmt.Errorf("cannot load version tree: (%s)", err)
		}
	}

	return i.updateImmutables()
}

func (i *IavlState) updateImmutables() (err error) {
	for _, t := range i.trees {
		v := t.tree.Version()
		if v > 0 {
			t.itree, err = t.tree.GetImmutable(v)
			if err != nil {
				return err
			}
		} else {
			t.itree = t.tree.ImmutableTree
		}
	}
	return nil
}

func (i *IavlState) AddTree(name string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	var st tmdb.DB
	var err error

	switch i.storageType {
	case statedb.StorageTypeDisk, "":
		st, err = tmdb.NewGoLevelDB(name, i.dataDir)
		if err != nil {
			return err
		}
	case statedb.StorageTypeMemory:
		st = tmdb.NewMemDB()
	default:
		return fmt.Errorf("storageType %s not supported", i.storageType)
	}

	var t *iavl.MutableTree
	if t, err = iavl.NewMutableTree(st, PrefixDBCacheSize); err != nil {
		return err
	}

	i.trees[name] = &IavlTree{
		tree:                t,
		itree:               t.ImmutableTree,
		lastCommitedVersion: uint64(t.Version()),
	}
	return nil
}

func (i *IavlState) Tree(name string) statedb.StateTree {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return &IavlTree{tree: i.trees[name].tree, isImmutable: false}
}

func (g *IavlState) TreeWithRoot(root []byte) statedb.StateTree {
	// TO-DO
	return nil
}

func (i *IavlState) ImmutableTree(name string) statedb.StateTree { // a tree version that won't change
	i.lock.RLock()
	defer i.lock.RUnlock()
	return &IavlTree{itree: i.trees[name].itree, isImmutable: true}
}

func (i *IavlState) KeyDiff(root1, root2 []byte) ([][]byte, error) {
	// TO-DO
	return nil, nil
}

func (i *IavlState) Commit() ([]byte, error) { // Returns New Hash
	i.lock.Lock()
	defer i.lock.Unlock()
	for name, t := range i.trees {
		i.versionTree.Set([]byte(name), []byte(strconv.FormatUint(t.lastCommitedVersion, 10)))
		if _, v, err := t.tree.SaveVersion(); err != nil {
			return nil, err
		} else {
			t.lastCommitedVersion = uint64(v)
		}
	}
	if _, _, err := i.versionTree.SaveVersion(); err != nil {
		return nil, fmt.Errorf("cannot save version state tree: (%s)", err)
	}
	if err := i.updateImmutables(); err != nil {
		return nil, err
	}
	return i.getHash(), nil
}

func (i *IavlState) Rollback() error {
	i.lock.Lock()
	defer i.lock.Unlock()
	for _, t := range i.trees {
		t.tree.Rollback()
	}
	return nil
}

func (i *IavlState) getHash() []byte {
	var hash string
	keys := make([]string, 0, len(i.trees))
	for k := range i.trees {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, t := range keys {
		hash = fmt.Sprintf("%s%s", hash, i.trees[t].tree.Hash())
	}
	return ethereum.HashRaw([]byte(hash))
}

func (i *IavlState) Hash() []byte {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.getHash()
}

func (t *IavlState) Close() error {
	return t.db.Close()
}

func (t *IavlTree) Get(key []byte) (r []byte, err error) {
	if t.isImmutable {
		_, r = t.itree.Get(key)
	} else {
		_, r = t.tree.Get(key)
	}
	return r, nil
}

func (t *IavlTree) Add(key, value []byte) error {
	if t.isImmutable {
		return fmt.Errorf("cannot add values to an immutable tree")
	}
	if value == nil {
		return fmt.Errorf("value cannot be nil")
	}
	t.tree.Set(key, value)
	return nil
}

func (t *IavlTree) Iterate(prefix []byte, callback func(key, value []byte) bool) {
	until := make([]byte, len(prefix))
	copy(until, prefix)
	// Set until to the next prefix: 0xABCDEF => 0xABCDFF
	for i := len(until) - 1; i >= 0; i-- {
		if until[i] != byte(0xFF) {
			until[i] += byte(0x01)
			break
		}
		if i == -1 {
			until = nil
			break
		}
	}

	if t.isImmutable {
		t.itree.IterateRange(prefix, until, true, callback)
	} else {
		t.tree.IterateRange(prefix, until, true, callback)
	}
}

func (t *IavlTree) Hash() []byte {
	if t.isImmutable {
		return t.itree.Hash()
	} else {
		return t.tree.Hash()
	}
}

func (t *IavlTree) Commit() error {
	_, _, err := t.tree.SaveVersion()
	return err
}

func (t *IavlTree) Count() uint64 {
	if t.isImmutable {
		return uint64(t.itree.Size())
	} else {
		return uint64(t.tree.Size())
	}
}

func (t *IavlTree) Version() uint64 {
	if t.isImmutable {
		return uint64(t.itree.Version())
	} else {
		return uint64(t.tree.Version())
	}
}

func (t *IavlTree) Proof(key []byte) ([]byte, error) {
	var p *iavl.RangeProof
	var err error
	if t.isImmutable {
		_, p, err = t.itree.GetWithProof(key)
	} else {
		_, p, err = t.tree.GetWithProof(key)
	}

	return []byte(p.String()), err
}

func (t *IavlTree) Verify(key, value, proof, root []byte) bool {
	// TO-DO
	return false
}
