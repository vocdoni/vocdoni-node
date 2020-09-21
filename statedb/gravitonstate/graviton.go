package gravitonstate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/deroproject/graviton"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/statedb"
)

const versionTree = "version"

type GravitonState struct {
	store             *graviton.Store
	hash              []byte
	trees             map[string]*GravitonTree
	imTrees           map[string]*GravitonTree
	treeLock          sync.RWMutex
	versionTree       *graviton.Tree
	lastCommitVersion uint64
}

type GravitonTree struct {
	tree    *graviton.Tree
	version uint64
}

func (g *GravitonState) Init(storagePath, sorageType string) (err error) {
	if g.store, err = graviton.NewDiskStore(storagePath); err != nil {
		return err
	}
	g.trees = make(map[string]*GravitonTree, 32)
	g.imTrees = make(map[string]*GravitonTree, 32)
	if err := g.LoadVersion(0); err != nil {
		return err
	}
	if err := g.LoadVersion(0); err != nil {
		return err
	}
	return nil
}

func (g *GravitonState) Version() uint64 {
	return g.lastCommitVersion
}

func (g *GravitonState) LoadVersion(v int64) error { // zero means last version
	var err error
	g.treeLock.Lock()
	defer g.treeLock.Unlock()

	version := uint64(v)

	if v == -1 {
		vb, _ := g.versionTree.Get([]byte(versionTree))
		if vb == nil {
			version = 0
		} else {
			version, err = strconv.ParseUint(string(vb), 10, 64)
			if err != nil {
				return err
			}
		}
	}

	sn, err := g.store.LoadSnapshot(version)
	if err != nil {
		return err
	}

	sn2, err := g.store.LoadSnapshot(version) // Do we need to create a separated snapshot for keep it immutable?
	if err != nil {
		return err
	}

	if g.versionTree, err = sn.GetTree(versionTree); err != nil {
		return err
	}

	for k := range g.trees {
		g.trees[k].tree, err = sn.GetTree(k)
		if err != nil {
			return err
		}
		g.imTrees[k].tree, err = sn2.GetTree(k)
		if err != nil {
			return err
		}
	}
	g.lastCommitVersion = sn.GetVersion()
	return nil
}

func (g *GravitonState) AddTree(name string) error {
	sn, err := g.store.LoadSnapshot(0)
	if err != nil {
		return err
	}
	t, err := sn.GetTree(name)
	if err != nil {
		return err
	}
	g.trees[name] = &GravitonTree{tree: t}

	sn2, err := g.store.LoadSnapshot(0)
	if err != nil {
		return err
	}
	t2, err := sn2.GetTree(name)
	if err != nil {
		return err
	}
	g.imTrees[name] = &GravitonTree{tree: t2}

	return nil
}

func (g *GravitonState) Tree(name string) statedb.StateTree {
	g.treeLock.RLock()
	defer g.treeLock.RUnlock()
	return g.trees[name]
}

func (g *GravitonState) updateImmutable() error {
	sn, err := g.store.LoadSnapshot(g.lastCommitVersion)
	if err != nil {
		return err
	}
	for k := range g.imTrees {
		t, err := sn.GetTree(k)
		if err != nil {
			return err
		}
		g.imTrees[k] = &GravitonTree{tree: t}
	}
	return nil
}

func (g *GravitonState) ImmutableTree(name string) statedb.StateTree {
	g.treeLock.RLock()
	defer g.treeLock.RUnlock()
	return g.imTrees[name]
} // a tree version that won't change

func (g *GravitonState) Commit() ([]byte, error) { // Returns New Hash
	var err error
	g.treeLock.Lock()
	defer g.treeLock.Unlock()

	// Save the last commited version
	g.versionTree.Put([]byte(versionTree), []byte(strconv.FormatUint(g.lastCommitVersion, 10)))
	if err = g.versionTree.Commit(); err != nil {
		return nil, err
	}

	// First update immutable tree to last stored version
	if err = g.updateImmutable(); err != nil {
		return nil, err
	}
	for _, t := range g.trees {
		if err = t.tree.Commit(); err != nil {
			return nil, err
		}
	}
	sn, err := g.store.LoadSnapshot(0) // Not sure if it's needed to load the snapshot to get the version, I cannot find another way
	if err != nil {
		return nil, err
	}
	g.lastCommitVersion = sn.GetVersion()
	g.hash = g.getHash()
	return g.hash, nil
}

func (g *GravitonState) getHash() []byte {
	var hash string
	var gh [32]byte
	var err error
	keys := make([]string, 0, len(g.trees))
	for k := range g.trees {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, t := range keys {
		gh, err = g.trees[t].tree.Hash()
		if err != nil {
			return nil
		}
		hash = fmt.Sprintf("%s%s", hash, gh[:])
	}
	return ethereum.HashRaw([]byte(hash))
}

func (g *GravitonState) Rollback() error {
	for _, t := range g.trees {
		if err := t.tree.Discard(); err != nil {
			return err
		}
	}
	return nil
}

func (g *GravitonState) Hash() []byte {
	return g.getHash()
}

func (t *GravitonTree) Get(key []byte) []byte {
	b, _ := t.tree.Get(key)
	return b
}

func (t *GravitonTree) Add(key, value []byte) error {
	return t.tree.Put(key, value)
}

func (t *GravitonTree) Version() uint64 {
	return t.version
}

func (t *GravitonTree) Iterate(prefix string, until string, callback func(key, value []byte) bool) {
	c := t.tree.Cursor()
	for k, v, err := c.First(); err == nil; k, v, err = c.Next() { // TBD: This is horrible from the performance point of view...
		if strings.HasPrefix(string(k), prefix) {
			if callback(k, v) {
				break
			}
		}
	}
}

func (t *GravitonTree) Hash() []byte {
	var h [32]byte
	var err error
	h, err = t.tree.Hash()
	if err != nil {
		return nil
	}
	var b []byte
	copy(b, h[:])
	return b
}

func (t *GravitonTree) Count() (count uint64) {
	c := t.tree.Cursor()
	for _, _, err := c.First(); err == nil; _, _, err = c.Next() { // TBD: This is horrible from the performance point of view...
		count++
	}
	return
}
