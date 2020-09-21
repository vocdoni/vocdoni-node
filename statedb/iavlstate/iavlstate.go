package iavlstate

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/tendermint/iavl"
	tmdb "github.com/tendermint/tm-db"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/statedb"
)

const PrefixDBCacheSize = 1024
const versionTree = "version"

type IavlState struct {
	dataDir     string
	dataType    string
	trees       map[string]*IavlTree
	lock        sync.RWMutex
	versionTree *iavl.MutableTree // For each tree, saves its last commited version
}

type IavlTree struct {
	tree                *iavl.MutableTree
	itree               *iavl.ImmutableTree
	isImmutable         bool
	lastCommitedVersion uint64
}

func (i *IavlState) Init(storagePath, storageType string) error {
	i.dataDir = storagePath
	i.dataType = storageType
	i.trees = make(map[string]*IavlTree, 32)

	// Create/Open version tree
	st, err := tmdb.NewGoLevelDB(versionTree, storagePath)
	if err != nil {
		return err
	}
	if i.versionTree, err = iavl.NewMutableTree(st, PrefixDBCacheSize); err != nil {
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
		// get las versiontree saved version and decrease by 1 if higher than zero
		if v == -1 {
			v, err = i.versionTree.Load()
			if err != nil {
				return fmt.Errorf("cannot load last version tree state: (%s)", err)
			}
			if v > 0 {
				v--
			}
		} else if v < -1 {
			return fmt.Errorf("loading versions below -1 is not supported")
		}

		for name, t := range i.trees {
			if _, err = i.versionTree.LoadVersionForOverwriting(v); err != nil {
				return fmt.Errorf("cannot load version number %d: (%s)", v, err)
			}
			_, vb := i.versionTree.Get([]byte(name))
			vt, err := strconv.ParseUint(string(vb), 10, 64)
			if err == nil {
				t.tree.LoadVersionForOverwriting(int64(vt))
			} else {
				v = 0
			}
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
			return fmt.Errorf("cannot load versions tree: (%s)", err)
		}
	}

	return i.updateImmutables()
}

func (i *IavlState) updateImmutables() error {
	for _, t := range i.trees {
		t.itree = t.tree.ImmutableTree
	}
	return nil
}

func (i *IavlState) AddTree(name string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	st, err := tmdb.NewGoLevelDB(name, i.dataDir)
	if err != nil {
		return err
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

func (i *IavlState) ImmutableTree(name string) statedb.StateTree { // a tree version that won't change
	i.lock.RLock()
	defer i.lock.RUnlock()
	return &IavlTree{itree: i.trees[name].itree, isImmutable: true}
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

func (t *IavlTree) Get(key []byte) (r []byte) {
	if t.isImmutable {
		_, r = t.itree.Get(key)
	} else {
		_, r = t.tree.Get(key)
	}
	return r
}

func (t *IavlTree) Add(key, value []byte) error {
	if t.isImmutable {
		return fmt.Errorf("cannot add values to a immutable tree")
	}
	if value == nil {
		return fmt.Errorf("value cannot be nil")
	}
	t.tree.Set(key, value)
	return nil
}

func (t *IavlTree) Iterate(prefix string, until string, callback func(key, value []byte) bool) {
	if t.isImmutable {
		t.itree.IterateRange([]byte(prefix), []byte(until), true, callback)
	} else {
		t.tree.IterateRange([]byte(prefix), []byte(until), true, callback)
	}
}

func (t *IavlTree) Hash() []byte {
	if t.isImmutable {
		return t.itree.Hash()
	} else {
		return t.tree.Hash()
	}
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
