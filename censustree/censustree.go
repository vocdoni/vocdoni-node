package censustree

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/proto/build/go/models"
)

// Tree implements the Merkle Tree used for census
// Concurrent updates to the tree.Tree can lead to losing some of the updates,
// so we don't expose the tree.Tree directly, and lock it in every method that
// updates it.
type Tree struct {
	tree *tree.Tree

	sync.Mutex
	public     uint32
	censusType models.Census_Type
}

type Options struct {
	// ParentDB is the Database under which all censuses are stored, each
	// with a different prefix.
	ParentDB   db.Database
	Name       string
	MaxLevels  int
	CensusType models.Census_Type
}

// TMP to be defined the production circuit nLevels
const nLevels = 256

// New returns a new Tree, if there already is a Tree in the
// database, it will load it.
func New(opts Options) (*Tree, error) {
	var hashFunc arbo.HashFunction

	switch opts.CensusType {
	case models.Census_ARBO_BLAKE2B:
		hashFunc = arbo.HashFunctionBlake2b
	case models.Census_ARBO_POSEIDON:
		hashFunc = arbo.HashFunctionPoseidon
	default:
		return nil, fmt.Errorf("unrecognized census type (%d)", opts.CensusType)
	}

	db := prefixeddb.NewPrefixedDatabase(opts.ParentDB, []byte(opts.Name))
	t, err := tree.New(nil, tree.Options{DB: db, MaxLevels: nLevels, HashFunc: hashFunc})
	if err != nil {
		return nil, err
	}
	return &Tree{tree: t, censusType: opts.CensusType}, nil
}

// Type returns the numeric identifier of the censustree implementation
func (t *Tree) Type() models.Census_Type {
	return t.censusType
}

// FromRoot returns a new read-only Tree for the given root, that uses the same
// underlying db.
func (t *Tree) FromRoot(root []byte) (*Tree, error) {
	treeFromRoot, err := t.tree.FromRoot(root)
	if err != nil {
		return nil, err
	}

	return &Tree{tree: treeFromRoot, censusType: t.Type()}, nil
}

// Publish makes a merkle tree available for queries.  Application layer should
// check IsPublic before considering the Tree available.
func (t *Tree) Publish() {
	atomic.StoreUint32(&t.public, 1)
}

// UnPublish makes a merkle tree not available for queries.
func (t *Tree) Unpublish() {
	atomic.StoreUint32(&t.public, 0)
}

// IsPublic returns true if the tree is available.
func (t *Tree) IsPublic() bool {
	return atomic.LoadUint32(&t.public) == 1
}

// Root wraps tree.Tree.Root.
func (t *Tree) Root() ([]byte, error) {
	return t.tree.Root(nil)
}

// Root wraps t.tree.VerifyProof.
func (t *Tree) VerifyProof(key, value, proof, root []byte) (bool, error) {
	return t.tree.VerifyProof(key, value, proof, root)
}

// GenProof wraps t.tree.GenProof.
func (t *Tree) GenProof(key []byte) ([]byte, []byte, error) {
	return t.tree.GenProof(nil, key)
}

// Size wraps t.tree.Size.
func (t *Tree) Size() (uint64, error) {
	return t.tree.Size(nil)
}

// Dump wraps t.tree.Dump.
func (t *Tree) Dump() ([]byte, error) {
	return t.tree.Dump()
}

// IterateLeaves wraps t.tree.IterateLeaves.
func (t *Tree) IterateLeaves(callback func(key, value []byte) bool) error {
	return t.tree.IterateLeaves(nil, callback)
}

// AddBatch wraps t.tree.AddBatch while acquiring the lock.
func (t *Tree) AddBatch(keys, values [][]byte) ([]int, error) {
	t.Lock()
	defer t.Unlock()
	return t.tree.AddBatch(nil, keys, values)
}

// Add wraps t.tree.Add while acquiring the lock.
func (t *Tree) Add(key, value []byte) error {
	t.Lock()
	defer t.Unlock()
	return t.tree.Add(nil, key, value)
}

// ImportDump wraps t.tree.ImportDump while acquiring the lock.
func (t *Tree) ImportDump(b []byte) error {
	t.Lock()
	defer t.Unlock()
	return t.tree.ImportDump(b)
}
