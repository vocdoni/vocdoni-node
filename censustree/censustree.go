package censustree

import (
	"sync/atomic"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/tree"
)

// CensusTree implements the Tree used for census
type CensusTree struct {
	*tree.Tree

	public uint32
}

// New returns a new CensusTree, if there already is a CensusTree in the
// database, it will load it.
func New(wTx db.WriteTx, opts tree.Options) (*CensusTree, error) {
	t, err := tree.New(wTx, opts)
	if err != nil {
		return nil, err
	}
	return &CensusTree{Tree: t}, nil
}

// Publish makes a merkle tree available for queries.  Application layer should
// check IsPublic before considering the Tree available.
func (t *CensusTree) Publish() {
	atomic.StoreUint32(&t.public, 1)
}

// UnPublish makes a merkle tree not available for queries
func (t *CensusTree) Unpublish() {
	atomic.StoreUint32(&t.public, 0)
}

// IsPublic returns true if the tree is available
func (t *CensusTree) IsPublic() bool {
	return atomic.LoadUint32(&t.public) == 1
}
