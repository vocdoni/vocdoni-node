package tree

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/tree/arbo"
)

// Tree defines the struct that implements the MerkleTree functionalities
type Tree struct {
	tree *arbo.Tree
	db   db.Database
}

// Options is used to pass the parameters to load a new Tree
type Options struct {
	// DB defines the database that will be used for the tree
	DB db.Database
	// MaxLevels that the Tree will have
	MaxLevels int
	// HashFunc defines the hash function that the tree will use
	HashFunc arbo.HashFunction
}

// New returns a new Tree, if there already is a Tree in the database, it will
// load it
func New(wTx db.WriteTx, opts Options) (*Tree, error) {
	givenTx := wTx != nil
	if !givenTx {
		wTx = opts.DB.WriteTx()
		defer wTx.Discard()
	}
	arboConfig := arbo.Config{
		Database:     opts.DB,
		MaxLevels:    opts.MaxLevels,
		HashFunction: opts.HashFunc,
		// ThresholdNLeafs: not specified, use the default
	}
	tree, err := arbo.NewTreeWithTx(wTx, arboConfig)
	if err != nil {
		return nil, err
	}

	if !givenTx {
		if err := wTx.Commit(); err != nil {
			return nil, err
		}
	}

	return &Tree{
		tree: tree,
		db:   opts.DB,
	}, nil
}

// DB returns the db.Database from the Tree
func (t *Tree) DB() db.Database {
	return t.db
}

// Get returns the value for a given key.
func (t *Tree) Get(rTx db.Reader, key []byte) ([]byte, error) {
	if rTx == nil {
		rTx = t.db
	}
	_, v, err := t.tree.GetWithTx(rTx, key)
	if err != nil {
		return nil, err
	}

	return v, nil
}

// Set adds or updates a key. First performs a Get, and if the key does not
// exist yet it will call Add, while if the key already exists, it will perform
// an Update. If the non-existence or existence of the key is already known, is
// more efficient to directly call Add or Update.
func (t *Tree) Set(wTx db.WriteTx, key, value []byte) error {
	givenTx := wTx != nil
	if !givenTx {
		wTx = t.DB().WriteTx()
		defer wTx.Discard()
	}
	err := t.tree.UpdateWithTx(wTx, key, value)
	if errors.Is(err, arbo.ErrKeyNotFound) {
		// key does not exist, use Add
		if err := t.tree.AddWithTx(wTx, key, value); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if !givenTx {
		if err := wTx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Add adds a new leaf, if the key already exists will return error
func (t *Tree) Add(wTx db.WriteTx, key, value []byte) error {
	givenTx := wTx != nil
	if !givenTx {
		wTx = t.DB().WriteTx()
		defer wTx.Discard()
	}
	if err := t.tree.AddWithTx(wTx, key, value); err != nil {
		return err
	}
	if !givenTx {
		if err := wTx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Del deletes a key from the Tree.
func (t *Tree) Del(wTx db.WriteTx, key []byte) error {
	givenTx := wTx != nil
	if !givenTx {
		wTx = t.DB().WriteTx()
		defer wTx.Discard()
	}
	if err := t.tree.DeleteWithTx(wTx, key); err != nil {
		return fmt.Errorf("could not remove key: %w", err)
	}
	if !givenTx {
		return wTx.Commit()
	}
	return nil
}

// AddBatch adds a batch of key-values to the Tree. Returns an array containing
// the indexes of the keys failed to add. Supports empty values as input
// parameters, which is equivalent to 0 valued byte array.
func (t *Tree) AddBatch(wTx db.WriteTx, keys, values [][]byte) ([]int, error) {
	givenTx := wTx != nil
	if !givenTx {
		wTx = t.DB().WriteTx()
		defer wTx.Discard()
	}
	invalid, err := t.tree.AddBatchWithTx(wTx, keys, values)
	invalidIndexes := func() []int {
		ii := []int{}
		for _, i := range invalid {
			ii = append(ii, i.Index)
		}
		return ii
	}()
	if err != nil {
		return invalidIndexes, err
	}

	if !givenTx {
		if err := wTx.Commit(); err != nil {
			return invalidIndexes, err
		}
	}
	return invalidIndexes, nil
}

// Iterate over all the database-encoded nodes of the tree.  When callback
// returns true, the iteration is stopped and this function returns.
func (t *Tree) Iterate(rTx db.Reader, callback func(key, value []byte) bool) error {
	if rTx == nil {
		rTx = t.db
	}
	callbackWrapper := func(currLvl int, k, v []byte) bool {
		return callback(k, v)
	}
	root, err := t.Root(rTx)
	if err != nil {
		return err
	}
	return t.tree.IterateWithStopWithTx(rTx, root, callbackWrapper)
}

// IterateLeaves iterates over all leafs of the tree.  When callback returns true,
// the iteration is stopped and this function returns.
func (t *Tree) IterateLeaves(rTx db.Reader, callback func(key, value []byte) bool) error {
	return t.Iterate(rTx, func(key, value []byte) bool {
		if value[0] != arbo.PrefixValueLeaf {
			return false
		}
		leafK, leafV := arbo.ReadLeafValue(value)
		return callback(leafK, leafV)
	})
}

// Root returns the current root.
func (t *Tree) Root(rTx db.Reader) ([]byte, error) {
	if rTx == nil {
		rTx = t.db
	}
	return t.tree.RootWithTx(rTx)
}

// Size returns the number of leafs under the current root
func (t *Tree) Size(rTx db.Reader) (uint64, error) {
	if rTx == nil {
		rTx = t.db
	}
	n, err := t.tree.GetNLeafsWithTx(rTx)
	return uint64(n), err
}

// GenProof returns a byte array with the necessary data to verify that the
// key&value are in a leaf under the current root
func (t *Tree) GenProof(rTx db.Reader, key []byte) ([]byte, []byte, error) {
	if rTx == nil {
		rTx = t.db
	}
	_, leafV, s, existence, err := t.tree.GenProofWithTx(rTx, key)
	if err != nil {
		return nil, nil, err
	}
	if !existence {
		// proof of non-existence currently not needed in vocdoni-node
		return nil, nil, fmt.Errorf("key does not exist %s", hex.EncodeToString(key))
	}
	return leafV, s, nil
}

// VerifyProof checks the proof for the given key, value and root, using the
// passed hash function
func VerifyProof(hashFunc arbo.HashFunction, key, value, proof, root []byte) error {
	return arbo.CheckProof(hashFunc, key, value, root, proof)
}

// VerifyProof checks the proof for the given key, value and root, using the
// hash function of the Tree
func (t *Tree) VerifyProof(key, value, proof, root []byte) error {
	return VerifyProof(t.tree.HashFunction(), key, value, proof, root)
}

// FromRoot returns a new read-only Tree for the given root, that uses the same
// underlying db.
func (t *Tree) FromRoot(root []byte) (*Tree, error) {
	tree, err := t.tree.Snapshot(root)
	if err != nil {
		return nil, err
	}
	return &Tree{
		tree: tree,
		db:   t.db,
	}, nil
}

func (t *Tree) SetRoot(wTx db.WriteTx, root []byte) error {
	givenTx := wTx != nil
	if !givenTx {
		wTx = t.DB().WriteTx()
		defer wTx.Discard()
	}
	if err := t.tree.SetRootWithTx(wTx, root); err != nil {
		return err
	}
	if !givenTx {
		if err := wTx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Dump exports all the Tree leafs in a byte array.
func (t *Tree) Dump() ([]byte, error) {
	return t.tree.Dump(nil)
}

// DumpWriter exports all the Tree leafs in a byte writer.
func (t *Tree) DumpWriter(w io.Writer) error {
	return t.tree.DumpWriter(nil, w)
}

// ImportDump imports the leafs (that have been exported with the Dump method)
// in the Tree.
func (t *Tree) ImportDump(b []byte) error {
	return t.tree.ImportDump(b)
}

// ImportDumpReader imports the leafs (that have been exported with the Dump method)
// in the Tree (using a byte reader)
func (t *Tree) ImportDumpReader(r io.Reader) error {
	return t.tree.ImportDumpReader(r)
}

// ImportDumpReaderWithTx imports the leafs (that have been exported with the Dump method)
// in the Tree (using a byte reader)
func (t *Tree) ImportDumpReaderWithTx(wTx db.WriteTx, r io.Reader) error {
	return t.tree.ImportDumpReaderWithTx(wTx, r)
}

func (t *Tree) PrintGraphviz(rTx db.Reader) error {
	if rTx == nil {
		rTx = t.db
	}
	return t.tree.PrintGraphvizFirstNLevels(rTx, nil, 0)
}
