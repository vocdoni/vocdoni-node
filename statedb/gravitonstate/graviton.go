package gravitonstate

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"sync/atomic"

	"github.com/deroproject/graviton"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/statedb"
)

const (
	GravitonHashSizeBytes = graviton.HASHSIZE_BYTES
	GravitonMaxKeySize    = graviton.MAX_KEYSIZE
	GravitonMaxValueSize  = graviton.MAX_VALUE_SIZE
)

type GravitonState struct {
	store             *graviton.Store
	hash              []byte
	trees             map[string]*GravitonTree
	imTrees           map[string]*GravitonTree
	treeLock          sync.RWMutex
	lastCommitVersion uint64
	vTree             *VersionTree
}

type GravitonTree struct {
	tree    *graviton.Tree
	version uint64
	size    uint64
}

type VersionTree struct {
	Name  string
	tree  *graviton.Tree
	store *graviton.Store
}

func (v *VersionTree) Init(g *graviton.Store) error {
	s, err := g.LoadSnapshot(0)
	if err != nil {
		return err
	}
	if v.Name == "" {
		v.Name = "versions"
	}
	v.tree, err = s.GetTree(v.Name)
	v.store = g
	return err
}

func (v *VersionTree) Commit() error {
	return v.tree.Commit()
}

func (v *VersionTree) Version() uint64 {
	return v.tree.GetVersion()
}

func (v *VersionTree) LoadVersion(version int64) error {
	s, err := v.store.LoadSnapshot(0)
	if err != nil {
		return err
	}
	if version == -1 {
		version = int64(v.tree.GetParentVersion())
	}
	v.tree, err = s.GetTreeWithVersion(v.Name, uint64(version))
	if err != nil {
		v.tree, err = s.GetTree(v.Name)
	}
	return err
}

func (v *VersionTree) Add(name string, version uint64) error {
	return v.tree.Put([]byte(name), []byte(strconv.FormatUint(version, 10)))
}

func (v *VersionTree) Get(name string) (uint64, error) {
	vb, err := v.tree.Get([]byte(name))
	if err != nil {
		return 0, nil
	}
	return strconv.ParseUint(string(vb), 10, 64)
}

func (v *VersionTree) String() (s string) {
	c := v.tree.Cursor()
	for k, _, err := c.First(); err == nil; k, _, err = c.Next() {
		vt, _ := v.Get(string(k))
		s = fmt.Sprintf("%s %s:%d ", s, k, vt)
	}
	return s
}

func (g *GravitonState) Init(storagePath, storageType string) (err error) {
	if storageType == "disk" || storageType == "" {
		if g.store, err = graviton.NewDiskStore(storagePath); err != nil {
			return err
		}
	} else if storageType == "mem" {
		if g.store, err = graviton.NewMemStore(); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("storageType %s not supported", storageType)
	}
	g.trees = make(map[string]*GravitonTree, 32)
	g.imTrees = make(map[string]*GravitonTree, 32)
	g.vTree = &VersionTree{}
	if err = g.vTree.Init(g.store); err != nil {
		return err
	}
	return nil
}

func (g *GravitonState) Version() uint64 {
	return g.lastCommitVersion
}

// LoadVersion loads a current version.
// Zero means last version, -1 means previous version.
// Values under -1 are not supported.
// Versions are obtained from a version Tree which stores the version of all existing trees.
func (g *GravitonState) LoadVersion(v int64) error {
	var err error
	g.treeLock.Lock()
	defer g.treeLock.Unlock()

	if err = g.vTree.LoadVersion(v); err != nil {
		return err
	}

	sn, err := g.store.LoadSnapshot(0)
	if err != nil {
		return err
	}

	// Update each tree to its version
	for k := range g.trees {
		vt := uint64(0)
		if v != 0 {
			vt, err = g.vTree.Get(k)
			if err != nil {
				return err
			}
		} else {
			vt, err = sn.GetTreeHighestVersion(k)
			if err != nil {
				vt = 0
			}
		}
		t2, err := sn.GetTreeWithVersion(k, vt)
		if err != nil {
			return err
		}
		g.trees[k].tree = t2
		g.trees[k].version = g.vTree.Version()
	}

	g.lastCommitVersion = g.vTree.Version()
	return g.updateImmutable()
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
	return g.updateImmutable()
}

func (g *GravitonState) Tree(name string) statedb.StateTree {
	g.treeLock.RLock()
	defer g.treeLock.RUnlock()
	return g.trees[name]
}

func (g *GravitonState) TreeWithRoot(root []byte) statedb.StateTree {
	g.treeLock.RLock()
	defer g.treeLock.RUnlock()
	sn, err := g.store.LoadSnapshot(0)
	if err != nil {
		return nil
	}
	gt, err := sn.GetTreeWithRootHash(root)
	if err != nil {
		log.Warn(err)
		return nil
	}
	return &GravitonTree{tree: gt, version: g.Version()}
}

func (g *GravitonState) updateImmutable() error {
	sn, err := g.store.LoadSnapshot(0)
	if err != nil {
		return err
	}
	for k := range g.trees {
		t, err := sn.GetTreeWithVersion(k, g.trees[k].tree.GetVersion())
		if err != nil {
			return err
		}
		g.imTrees[k] = &GravitonTree{tree: t, version: g.lastCommitVersion}
	}
	return nil
}

// ImmutableTree is a tree snapshot that won't change, useful for making queries on a state changing environment
func (g *GravitonState) ImmutableTree(name string) statedb.StateTree {
	g.treeLock.RLock()
	defer g.treeLock.RUnlock()
	return g.imTrees[name]
}

// Commit saves the current state of the trees and updates versions.
// Returns New Hash
func (g *GravitonState) Commit() ([]byte, error) {
	var err error
	g.treeLock.Lock()
	defer g.treeLock.Unlock()

	// Commit current trees and save versions to the version Tree
	for name, t := range g.trees {
		if err = t.tree.Commit(); err != nil {
			return nil, err
		}
		if err = g.vTree.Add(name, t.tree.GetVersion()); err != nil {
			return nil, err
		}
	}
	// Las commmit version is the versions-tree Version number
	if err = g.vTree.Commit(); err != nil {
		return nil, err
	}
	g.lastCommitVersion = g.vTree.Version()

	// Update immutable tree to last commited version, this tree should not change until next Commit
	if err = g.updateImmutable(); err != nil {
		return nil, err
	}

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

// Rollback discards the non-commited changes of the tree
func (g *GravitonState) Rollback() error {
	for _, t := range g.trees {
		if err := t.tree.Discard(); err != nil {
			return err
		}
	}
	return nil
}

// Hash returns the merkle root hash of all trees hash(hashTree1+hashTree2+...)
func (g *GravitonState) Hash() []byte {
	return g.getHash()
}

func (g *GravitonState) Close() error {
	g.store.Close()
	return nil
}

func (t *GravitonTree) Get(key []byte) []byte {
	b, _ := t.tree.Get(key)
	return b
}

func (t *GravitonTree) Add(key, value []byte) error {
	// if already exist, just return
	if v, err := t.tree.Get(key); err == nil && string(v) == string(value) {
		return nil
	}
	// if it does not exist, add, increase size counter and return
	err := t.tree.Put(key, value)
	if err == nil {
		atomic.AddUint64(&t.size, 1)
	}
	return err
}

func (t *GravitonTree) Version() uint64 {
	return t.version
}

func (t *GravitonTree) Iterate(prefix []byte, callback func(key, value []byte) bool) {
	c := t.tree.Cursor()
	for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
		// This is horrible from the performance point of view...
		// TBD: Find better ways to to this iteration over the whole tree
		if bytes.HasPrefix(k, prefix) {
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
	b := make([]byte, 32)
	copy(b, h[:])
	return b
}

func (t *GravitonTree) count() (count uint64) {
	c := t.tree.Cursor()
	for _, _, err := c.First(); err == nil; _, _, err = c.Next() {
		// TBD: This is horrible from the performance point of view...
		count++
	}
	return
}

func (t *GravitonTree) Count() uint64 {
	c := atomic.LoadUint64(&t.size)
	if c == 0 {
		c = t.count()
		atomic.StoreUint64(&t.size, c)
	}
	return c
}

func (t *GravitonTree) Proof(key []byte) ([]byte, error) {
	proof, err := t.tree.GenerateProof(key)
	if err != nil {
		return nil, err
	}
	proofBytes := proof.Marshal()
	if !t.Verify(key, proofBytes, nil) {
		return nil, nil
	}
	return proofBytes, nil
}

func (t *GravitonTree) Verify(key, proof, root []byte) bool {
	var p graviton.Proof
	var err error
	var r [32]byte
	// Unmarshal() will generate a panic if the proof size is incorrect. See https://gitlab.com/vocdoni/go-dvote/-/issues/333
	// While this is not fixed upstream, we need to recover the panic.
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("recovered graviton verify panic: %v", r)
		}
	}()
	if err = p.Unmarshal(proof); err != nil {
		log.Error(err)
		return false
	}
	if root == nil {
		r, err = t.tree.Hash()
		if err != nil {
			return false
		}
	} else {
		copy(r[:], root[:32])
	}
	return p.VerifyMembership(r, key)
}

func Verify(key, proof, root []byte) (bool, error) {
	var p graviton.Proof
	var r [32]byte
	if err := p.Unmarshal(proof); err != nil {
		log.Error(err)
		return false, err
	}
	if len(root) != 32 {
		return false, fmt.Errorf("root hash size is not correct")
	}
	copy(r[:], root[:32])
	return p.VerifyMembership(r, key), nil
}
