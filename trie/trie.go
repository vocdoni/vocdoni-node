// Package tree provides the functions for creating and managing an iden3 merkletree
package trie

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/statedb"
	"gitlab.com/vocdoni/go-dvote/statedb/gravitonstate"
)

type Tree struct {
	Tree           statedb.StateTree
	store          statedb.StateDB
	dataDir        string
	name           string
	public         uint32
	lastAccessUnix int64 // a unix timestamp, used via sync/atomic
}

const (
	MaxIndexSize = 128
	MaxValueSize = 256
)

// NewTree opens or creates a merkle tree under the given storage.
// Note that the storage should be prefixed, since each tree should use an
// entirely separate namespace for its database keys.
func NewTree(name, storageDir string) (*Tree, error) {
	gs := new(gravitonstate.GravitonState)

	//name = strings.ReplaceAll(name, "/", "_")
	iname := name
	if len(iname) > 32 {
		iname = iname[:32]
	}
	dir := fmt.Sprintf("%s/%s", storageDir, name)
	log.Debugf("creating census tree %s on %s", iname, dir)

	if err := gs.Init(dir, "disk"); err != nil {
		return nil, err
	}
	if err := gs.AddTree(iname); err != nil {
		return nil, err
	}
	if err := gs.LoadVersion(0); err != nil {
		return nil, err
	}
	tr := &Tree{store: gs, Tree: gs.Tree(iname), name: iname, dataDir: dir}
	tr.updateAccessTime()
	return tr, nil
}

func (t *Tree) MaxClaimSize() int {
	return MaxIndexSize
}

// LastAccess returns the last time the Tree was accessed, in the form of a unix
// timestamp.
func (t *Tree) LastAccess() int64 {
	return atomic.LoadInt64(&t.lastAccessUnix)
}

// TODO(mvdan): use sync/atomic instead to avoid introducing a bottleneck
func (t *Tree) updateAccessTime() {
	atomic.StoreInt64(&t.lastAccessUnix, time.Now().Unix())
}

// Publish makes a merkle tree available for queries.
// Application layer should check IsPublish() before considering the Tree available.
func (t *Tree) Publish() {
	atomic.StoreUint32(&t.public, 1)
}

// UnPublish makes a merkle tree not available for queries
func (t *Tree) UnPublish() {
	atomic.StoreUint32(&t.public, 0)
}

// IsPublic returns true if the tree is available
func (t *Tree) IsPublic() bool {
	return atomic.LoadUint32(&t.public) == 1
}

// AddClaim adds a new claim to the merkle tree
// A claim is composed of two parts: index and value
//  1.index is mandatory, the data will be used for indexing the claim into to merkle tree
//  2.value is optional, the data will not affect the indexing
// Use value only if index is too small
func (t *Tree) AddClaim(index, value []byte) error {
	t.updateAccessTime()
	if len(index) < 4 {
		return fmt.Errorf("claim index too small (%d), minimum size is 4 bytes", len(index))
	}
	if len(index) > MaxIndexSize || len(value) > MaxValueSize {
		return fmt.Errorf("index or value claim data too big")
	}
	if err := t.Tree.Add(index, value); err != nil {
		return err
	}
	_, err := t.store.Commit()
	return err
}

// GenProof generates a merkle tree proof that can be later used on CheckProof() to validate it
func (t *Tree) GenProof(index, value []byte) (string, error) {
	t.updateAccessTime()
	proof, err := t.Tree.Proof(index)
	if err != nil {
		return "", err
	}
	if proof == nil {
		return "", nil
	}
	return fmt.Sprintf("%x", proof), nil
}

// CheckProof standalone function for checking a merkle proof
func CheckProof(root, mpHex string, index, value []byte) (bool, error) {
	p, err := hex.DecodeString(mpHex)
	if err != nil {
		return false, err
	}
	r, err := hex.DecodeString(root)
	if err != nil {
		return false, err
	}
	return gravitonstate.Verify(index, p, r)
}

// CheckProof validates a merkle proof and its data
func (t *Tree) CheckProof(index, value []byte, mpHex string) (bool, error) {
	t.updateAccessTime()
	proof, err := hex.DecodeString(mpHex)
	if err != nil {
		return false, err
	}
	return t.Tree.Verify(index, proof, nil), nil
}

// Root returns the current root hash of the merkle tree
func (t *Tree) Root() string {
	t.updateAccessTime()
	return fmt.Sprintf("%x", t.Tree.Hash())
}

func (t *Tree) treeWithRoot(root string) statedb.StateTree {
	if root == "" {
		return t.Tree
	}
	if len(root) >= 2 && root[0] == '0' && (root[1] == 'x' || root[1] == 'X') {
		root = root[2:]
	}
	r, err := hex.DecodeString(root)
	if err != nil {
		log.Warn(err)
		return nil
	}
	return t.store.TreeWithRoot(r)
}

// Dump returns the whole merkle tree serialized in a format that can be used on Import
func (t *Tree) Dump(root string) (claims []string, err error) {
	t.updateAccessTime()
	tree := t.treeWithRoot(root)
	if tree == nil {
		return nil, fmt.Errorf("dump: root not found %s", root)
	}
	tree.Iterate("", "", func(k, v []byte) bool {
		claims = append(claims, fmt.Sprintf("%x", k))
		return false
	})
	return
}

// Size returns the number of leaf nodes on the merkle tree
func (t *Tree) Size(root string) (int64, error) {
	tree := t.treeWithRoot(root)
	if tree == nil {
		return 0, fmt.Errorf("size: root not found %s", root)
	}
	return int64(tree.Count()), nil
}

// DumpPlain returns the entire list of added claims for a specific root hash
// First return parametre are the indexes and second the values
// If root is not specified, the current one is used
// If responseBase64 is true, the list will be returned base64 encoded
func (t *Tree) DumpPlain(root string, responseBase64 bool) ([]string, []string, error) {
	var indexes, values []string
	var err error
	t.updateAccessTime()

	tree := t.treeWithRoot(root)
	if tree == nil {
		return nil, nil, fmt.Errorf("dumpplain: root not found %s", root)
	}
	tree.Iterate("", "", func(k, v []byte) bool {
		if !responseBase64 {
			indexes = append(indexes, string(k))
			values = append(values, string(v))
		} else {
			indexes = append(indexes, base64.StdEncoding.EncodeToString(k))
			values = append(values, base64.StdEncoding.EncodeToString(v))

		}
		return false
	})

	return indexes, values, err
}

// ImportDump imports a partial or whole tree previously exported with Dump()
func (t *Tree) ImportDump(claims []string) error {
	t.updateAccessTime()
	var cb []byte
	var err error
	for _, c := range claims {
		cb, err = hex.DecodeString(c)
		if err != nil {
			t.store.Rollback()
			return err
		}
		if err = t.Tree.Add(cb, []byte{}); err != nil {
			t.store.Rollback()
			return err
		}
	}
	_, err = t.store.Commit()
	return err
}

// Snapshot returns a Tree instance of a exiting merkle root
func (t *Tree) Snapshot(root string) (*Tree, error) {
	tree := t.treeWithRoot(root)
	if tree == nil {
		return nil, fmt.Errorf("snapshot: root not valid or not found %s", root)
	}
	return &Tree{Tree: tree, public: t.public}, nil
}

// HashExist checks if a hash exists as a node in the merkle tree
func (t *Tree) HashExist(hash string) (bool, error) {
	t.updateAccessTime()
	tree := t.treeWithRoot(hash)
	if tree == nil {
		return false, nil
	}
	return true, nil
}
