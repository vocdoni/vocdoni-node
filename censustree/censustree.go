package censustree

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/badgerdb"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/proto/build/go/models"
)

// Tree implements the Merkle Tree used for census
type Tree struct {
	*tree.Tree

	public     uint32
	censusType models.Census_Type
}

type Options struct {
	Name       string
	StorageDir string
	MaxLevels  int
	CensusType models.Census_Type
}

// TMP to be defined the production circuit nLevels
const nLevels = 256

// New returns a new Tree, if there already is a Tree in the
// database, it will load it.
func New(wTx db.WriteTx, opts Options) (*Tree, error) {
	var hashFunc arbo.HashFunction

	switch opts.CensusType {
	case models.Census_ARBO_BLAKE2B:
		hashFunc = arbo.HashFunctionBlake2b
	case models.Census_ARBO_POSEIDON:
		hashFunc = arbo.HashFunctionPoseidon
	default:
		return nil, fmt.Errorf("unrecognized census type (%d)", opts.CensusType)
	}

	dbDir := filepath.Join(opts.StorageDir, "arbotree.db."+strings.TrimSpace(opts.Name))
	database, err := badgerdb.New(badgerdb.Options{Path: dbDir})
	if err != nil {
		return nil, err
	}

	t, err := tree.New(wTx, tree.Options{DB: database, MaxLevels: nLevels, HashFunc: hashFunc})
	if err != nil {
		database.Close()
		return nil, err
	}
	return &Tree{Tree: t, censusType: opts.CensusType}, nil
}

// Type returns the numeric identifier of the censustree implementation
func (t *Tree) Type() models.Census_Type {
	return t.censusType
}

// FromRoot returns a new read-only Tree for the given root, that uses the same
// underlying db.
func (t *Tree) FromRoot(root []byte) (*Tree, error) {
	treeFromRoot, err := t.Tree.FromRoot(root)
	if err != nil {
		return nil, err
	}

	return &Tree{Tree: treeFromRoot, censusType: t.Type()}, nil
}

// Publish makes a merkle tree available for queries.  Application layer should
// check IsPublic before considering the Tree available.
func (t *Tree) Publish() {
	atomic.StoreUint32(&t.public, 1)
}

// UnPublish makes a merkle tree not available for queries
func (t *Tree) Unpublish() {
	atomic.StoreUint32(&t.public, 0)
}

// IsPublic returns true if the tree is available
func (t *Tree) IsPublic() bool {
	return atomic.LoadUint32(&t.public) == 1
}
