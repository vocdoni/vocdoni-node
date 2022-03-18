package censustree

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/proto/build/go/models"
)

var censusWeightKey = []byte("censusWeight")

// Tree implements the Merkle Tree used for census
// Concurrent updates to the tree.Tree can lead to losing some of the updates,
// so we don't expose the tree.Tree directly, and lock it in every method that
// updates it.
type Tree struct {
	tree *tree.Tree

	sync.Mutex
	public     uint32
	censusType models.Census_Type
	hashFunc   func(...[]byte) ([]byte, error)
	hashLen    int
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
	return &Tree{tree: t, censusType: opts.CensusType, hashFunc: hashFunc.Hash, hashLen: hashFunc.Len()}, nil
}

// Type returns the numeric identifier of the censustree implementation
func (t *Tree) Type() models.Census_Type {
	return t.censusType
}

// Hash executes the tree hash function for input data and returns its output
func (t *Tree) Hash(data []byte) ([]byte, error) {
	return t.hashFunc(data)
}

// BytesToBigInt unmarshals a slice of bytes into a bigInt following the censusTree encoding rules
func (t *Tree) BytesToBigInt(data []byte) *big.Int {
	return arbo.BytesToBigInt(data)
}

// BigIntToBytes marshals a bigInt following the censusTree encoding rules
func (t *Tree) BigIntToBytes(b *big.Int) []byte {
	return arbo.BigIntToBytes(t.hashLen, b)
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

// Get wraps tree.Tree.Get.
func (t *Tree) Get(key []byte) ([]byte, error) {
	return t.tree.Get(nil, key)
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

func (t *Tree) updateCensusWeight(wTx db.WriteTx, w []byte) error {
	weightBytes, err := wTx.Get(censusWeightKey)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return fmt.Errorf("could not get census weight: %w", err)
	}
	weight := new(big.Int).Add(t.BytesToBigInt(weightBytes), t.BytesToBigInt(w))
	if err := wTx.Set(censusWeightKey, t.BigIntToBytes(weight)); err != nil {
		return fmt.Errorf("could not set census weight: %w", err)
	}
	return nil
}

// AddBatch wraps t.tree.AddBatch while acquiring the lock.
func (t *Tree) AddBatch(keys, values [][]byte) ([]int, error) {
	t.Lock()
	defer t.Unlock()

	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()

	invalids, err := t.tree.AddBatch(wTx, keys, values)
	if err != nil {
		return invalids, err
	}

	if values == nil || len(values) != len(keys) {
		if err := wTx.Commit(); err != nil {
			return nil, err
		}
		return invalids, err
	}

	// TODO Warning, this CensusWeight update should be done only for the
	// censuses that have weight. Other kind of censuses (eg.
	// AnonymousVoting) do not use the value parameter of the leaves as
	// weight, as it's used for other stuff, so the CensusWeight in those
	// cases would get wrong values.

	// get value of all added keys
	addedWeight := big.NewInt(0)
	for i := 0; i < len(keys); i++ {
		addedWeight = new(big.Int).Add(addedWeight, t.BytesToBigInt(values[i]))
	}
	// remove weight of invalid leafs
	for i := 0; i < len(invalids); i++ {
		addedWeight = new(big.Int).Sub(addedWeight, t.BytesToBigInt(values[invalids[i]]))
	}

	if err := t.updateCensusWeight(wTx, t.BigIntToBytes(addedWeight)); err != nil {
		return nil, err
	}

	if err := wTx.Commit(); err != nil {
		return nil, err
	}
	return invalids, err
}

// Add wraps t.tree.Add while acquiring the lock.
func (t *Tree) Add(key, value []byte) error {
	t.Lock()
	defer t.Unlock()

	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()

	// TODO Warning, this CensusWeight update should be done only for the
	// censuses that have weight. Other kind of censuses (eg.
	// AnonymousVoting) do not use the value parameter of the leaves as
	// weight, as it's used for other stuff, so the CensusWeight in those
	// cases would get wrong values.
	if err := t.updateCensusWeight(wTx, value); err != nil {
		return err
	}

	if err := t.tree.Add(wTx, key, value); err != nil {
		return err
	}
	return wTx.Commit()
}

// ImportDump wraps t.tree.ImportDump while acquiring the lock.
func (t *Tree) ImportDump(b []byte) error {
	t.Lock()
	defer t.Unlock()

	err := t.tree.ImportDump(b)
	if err != nil {
		return err
	}

	// TODO Warning, this CensusWeight update should be done only for the
	// censuses that have weight. Other kind of censuses (eg.
	// AnonymousVoting) do not use the value parameter of the leaves as
	// weight, as it's used for other stuff, so the CensusWeight in those
	// cases would get wrong values.

	// get the total addedWeight from parsing the dump byte array, and
	// adding the weight of its values
	addedWeight := big.NewInt(0)
	r := bytes.NewReader(b)
	for {
		l := make([]byte, 2)
		_, err := io.ReadFull(r, l)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		k := make([]byte, l[0])
		_, err = io.ReadFull(r, k)
		if err != nil {
			return err
		}
		v := make([]byte, l[1])
		_, err = io.ReadFull(r, v)
		if err != nil {
			return err
		}
		// add the weight (value of the leaf)
		addedWeight = new(big.Int).Add(addedWeight, t.BytesToBigInt(v))
	}

	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()
	if err := t.updateCensusWeight(wTx, t.BigIntToBytes(addedWeight)); err != nil {
		return err
	}
	if err := wTx.Commit(); err != nil {
		return err
	}
	return nil
}

// GetCensusWeight returns the current weight of the census.
func (t *Tree) GetCensusWeight() (*big.Int, error) {
	t.Lock()
	defer t.Unlock()
	weight, err := t.tree.DB().ReadTx().Get(censusWeightKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return big.NewInt(0), nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get census weight: %w", err)
	}

	return t.BytesToBigInt(weight), nil
}
