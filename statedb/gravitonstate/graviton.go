package gravitonstate

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/deroproject/graviton"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
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
	lastCommitVersion uint64
	vTree             *VersionTree
}

// check that statedb.StateDB interface is matched by GravitonState
var _ statedb.StateDB = (*GravitonState)(nil)

// safeTree is a wrapper around graviton.Tree which uses a RWMutex to ensure its
// methods are safe for concurrent use. The methods chosen to use the lock were
// found via the race detector.
//
// Note that Cursor is special, because we should not let go of the read lock
// until we are done using the cursor entirely.
type safeTree struct {
	gtreeMu sync.RWMutex
	gtree   *graviton.Tree
}

func (t *safeTree) Put(key, value []byte) error {
	t.gtreeMu.Lock()
	defer t.gtreeMu.Unlock()
	return t.gtree.Put(key, value)
}

func (t *safeTree) Commit(tags ...string) error {
	t.gtreeMu.Lock()
	defer t.gtreeMu.Unlock()
	return t.gtree.Commit(tags...)
}

func (t *safeTree) Cursor() (_ graviton.Cursor, deferFn func()) {
	t.gtreeMu.RLock()
	return t.gtree.Cursor(), t.gtreeMu.RUnlock
}

func (t *safeTree) Delete(key []byte) error {
	t.gtreeMu.Lock()
	defer t.gtreeMu.Unlock()
	return t.gtree.Delete(key)
}

func (t *safeTree) Discard() error {
	return t.gtree.Discard()
}

func (t *safeTree) GenerateProof(key []byte) (*graviton.Proof, error) {
	return t.gtree.GenerateProof(key)
}

func (t *safeTree) Get(key []byte) ([]byte, error) {
	return t.gtree.Get(key)
}

func (t *safeTree) GetParentVersion() uint64 {
	return t.gtree.GetParentVersion()
}

func (t *safeTree) GetVersion() uint64 {
	return t.gtree.GetVersion()
}

func (t *safeTree) Hash() (h [graviton.HASHSIZE]byte, err error) {
	return t.gtree.Hash()
}

func (t *safeTree) IsDirty() bool {
	return t.gtree.IsDirty()
}

type GravitonTree struct {
	tree    safeTree
	version uint64
	size    uint64

	tmpSizeCounter uint64
}

// check that statedb.StateTree interface is matched by GravitonTree
var _ statedb.StateTree = (*GravitonTree)(nil)

type VersionTree struct {
	Name  string
	tree  safeTree
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
	v.tree.gtree, err = s.GetTree(v.Name)
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
	v.tree.gtree, err = s.GetTreeWithVersion(v.Name, uint64(version))
	if err != nil {
		v.tree.gtree, err = s.GetTree(v.Name)
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
	c, deferFn := v.tree.Cursor()
	defer deferFn()
	for k, _, err := c.First(); err == nil; k, _, err = c.Next() {
		vt, _ := v.Get(string(k))
		s = fmt.Sprintf("%s %s:%d ", s, k, vt)
	}
	return s
}

func (g *GravitonState) Init(storagePath string, storageType statedb.StorageType) (err error) {
	switch storageType {
	case statedb.StorageTypeDisk, "":
		if g.store, err = graviton.NewDiskStore(storagePath); err != nil {
			return err
		}
	case statedb.StorageTypeMemory:
		if g.store, err = graviton.NewMemStore(); err != nil {
			return err
		}
	default:
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
		g.trees[k].tree.gtree = t2
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
	g.trees[name] = &GravitonTree{tree: safeTree{gtree: t}}
	if err := g.trees[name].Init(); err != nil {
		return err
	}
	return g.updateImmutable()
}

func (g *GravitonState) Tree(name string) statedb.StateTree {
	return g.trees[name]
}

func (g *GravitonState) TreeWithRoot(root []byte) statedb.StateTree {
	sn, err := g.store.LoadSnapshot(0)
	if err != nil {
		return nil
	}
	gt, err := sn.GetTreeWithRootHash(root)
	if err != nil {
		return nil
	}

	tree := &GravitonTree{tree: safeTree{gtree: gt}, version: g.Version()}
	if err = tree.Init(); err != nil {
		return nil
	}
	return tree
}

// KeyDiff returns the list of inserted keys on rootBig not present in rootSmall
func (g *GravitonState) KeyDiff(rootSmall, rootBig []byte) ([][]byte, error) {
	t1 := g.TreeWithRoot(rootSmall)
	t2 := g.TreeWithRoot(rootBig)
	diff := [][]byte{}
	if t1 == nil && t2 == nil {
		return diff, fmt.Errorf("tree with specified root not found")
	}
	if t2 == nil {
		return nil, nil
	}
	if t1 == nil {
		t2.Iterate(nil, func(k, v []byte) bool {
			diff = append(diff, k)
			return false
		})
		return diff, nil
	}
	err := graviton.Diff(t1.(*GravitonTree).tree.gtree, t2.(*GravitonTree).tree.gtree, nil, nil,
		func(k, v []byte) {
			diff = append(diff, k)
		})
	return diff, err
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
		g.imTrees[k] = &GravitonTree{tree: safeTree{gtree: t}, version: g.lastCommitVersion}
	}
	return nil
}

// ImmutableTree is a tree snapshot that won't change, useful for making queries
// on a state changing environment
func (g *GravitonState) ImmutableTree(name string) statedb.StateTree {
	return g.imTrees[name]
}

// Commit saves the current state of the trees and updates versions.
// Returns New Hash
func (g *GravitonState) Commit() ([]byte, error) {
	var err error

	// Commit current trees and save versions to the version Tree
	for name, t := range g.trees {
		if err = t.Commit(); err != nil {
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

	// Update immutable tree to last committed version, this tree should not change until next Commit
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

// Rollback discards the non-committed changes of the tree
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

func (t *GravitonTree) Init() error {
	atomic.StoreUint64(&t.size, t.count())
	t.tmpSizeCounter = 0
	return nil
}

func (t *GravitonTree) Get(key []byte) ([]byte, error) {
	b, err := t.tree.Get(key)
	return b, err
}

func (t *GravitonTree) Add(key, value []byte) error {
	// if already exist, just return
	if v, err := t.tree.Get(key); err == nil {
		if !bytes.Equal(v, value) {
			return t.tree.Put(key, value)
		}
		return nil
	}
	// if it does not exist, add, increase size counter and return
	err := t.tree.Put(key, value)
	if err == nil {
		atomic.AddUint64(&t.tmpSizeCounter, 1)
	}
	return err
}

func (t *GravitonTree) Version() uint64 {
	return t.version
}

func (t *GravitonTree) Iterate(prefix []byte, callback func(key, value []byte) bool) {
	c, deferFn := t.tree.Cursor()
	defer deferFn()
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
	c, deferFn := t.tree.Cursor()
	defer deferFn()
	for _, _, err := c.First(); err == nil; _, _, err = c.Next() {
		count++
	}
	return
}

func (t *GravitonTree) Commit() error {
	atomic.AddUint64(&t.size, atomic.SwapUint64(&t.tmpSizeCounter, 0))
	return t.tree.Commit()
}

func (t *GravitonTree) Count() uint64 {
	return atomic.LoadUint64(&t.size)
}

func (t *GravitonTree) Proof(key []byte) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("key is nil")
	}
	proof, err := t.tree.GenerateProof(key)
	if err != nil {
		return nil, err
	}
	if proof == nil {
		return nil, nil
	}
	proofBytes := proof.Marshal()
	root, err := t.tree.Hash()
	if err != nil {
		return nil, err
	}
	if !proof.VerifyMembership(root, key) {
		return nil, nil
	}

	return proofBytes, nil
}

func (t *GravitonTree) Verify(key, value, proof, root []byte) bool {
	if root == nil {
		root = t.Hash()
	}
	valid, err := Verify(key, value, proof, root)
	if err != nil {
		log.Debugf("graviton verify proof error: %v", err)
		return false
	}
	return valid
}

func Verify(key, value, proof, root []byte) (bool, error) {
	var p graviton.Proof
	var r [GravitonHashSizeBytes]byte
	if proof == nil || key == nil {
		return false, fmt.Errorf("proof and/or key is nil")
	}
	// Unmarshal() will generate a panic if the proof size is incorrect.
	// See https://go.vocdoni.io/dvote/-/issues/333
	// While this is not fixed upstream, we need to recover the panic.
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("recovered graviton verify panic: %v", r)
		}
	}()
	if err := p.Unmarshal(proof); err != nil {
		log.Error(err)
		return false, err
	}
	if !bytes.Equal(p.Value(), value) {
		return false, nil
	}
	if len(root) != GravitonHashSizeBytes {
		return false, fmt.Errorf("root hash size is not correct")
	}
	copy(r[:], root[:GravitonHashSizeBytes])
	return p.VerifyMembership(r, key), nil
}
