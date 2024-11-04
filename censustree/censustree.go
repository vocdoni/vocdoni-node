package censustree

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/proto/build/go/models"
)

var censusWeightKey = []byte("censusWeight")

// Tree implements the Merkle Tree used for census
// Concurrent updates to the tree.Tree can lead to losing some of the updates,
// so we don't expose the tree.Tree directly, and lock it in every method that
// updates it.
type Tree struct {
	tree        *tree.Tree
	censusType  models.Census_Type
	hashFunc    func(...[]byte) ([]byte, error)
	hashLen     int
	updatesLock sync.RWMutex
}

type Options struct {
	// ParentDB is the Database under which all censuses are stored, each
	// with a different prefix.
	ParentDB   db.Database
	Name       string
	MaxLevels  int
	CensusType models.Census_Type
}

// DefaultMaxLevels is by default, the maximum number of levels will be 160, which allows to add
// up to 2^160 leaves to the tree, with keys with up to 20 bytes. However the
// number of levels could be set up during the tree initialization which
// allows to support uses cases with specific restrictions such as the
// zkweighted census, which needs to optimize some artifacts size that depends
// of the number of levels of the tree.
const DefaultMaxLevels = 160

// DefaultMaxKeyLen is the maximum length of a census tree key is defined by the number of levels of
// the census tree. It is the number of bytes that can fit into 'n' bits, where
// 'n' is the number of levels of the census tree.
const DefaultMaxKeyLen = DefaultMaxLevels / 8

// DeleteCensusTreeFromDatabase removes all the database entries for the census identified by name.
// Caller must take care of potential data races, the census must be closed before calling this method.
// Returns the number of removed items.
func DeleteCensusTreeFromDatabase(kv db.Database, name string) (int, error) {
	database := prefixeddb.NewPrefixedDatabase(kv, []byte(name))
	wTx := database.WriteTx()
	i := 0
	if err := database.Iterate(nil, func(k, _ []byte) bool {
		if err := wTx.Delete(k); err != nil {
			log.Warnf("could not remove key %x from database", k)
		} else {
			i++
		}
		return true
	}); err != nil {
		return 0, err
	}
	return i, wTx.Commit()
}

// New returns a new Tree, if there already is a Tree in the
// database, it will load it.
func New(opts Options) (*Tree, error) {
	maxLevels := opts.MaxLevels
	if maxLevels > DefaultMaxLevels {
		maxLevels = DefaultMaxLevels
	}

	var hashFunc arbo.HashFunction
	switch opts.CensusType {
	case models.Census_ARBO_BLAKE2B:
		hashFunc = arbo.HashFunctionBlake2b
	case models.Census_ARBO_POSEIDON:
		// If an Arbo census tree is based on Poseidon hash to use it with a
		// circom circuit, it is necessary to increase the number of levels by
		// one. This is done to emulate the same behaviour than Circom SMT
		// library, which increases the desired number of levels by one.
		maxLevels++
		hashFunc = arbo.HashFunctionPoseidon
	default:
		return nil, fmt.Errorf("unrecognized census type (%d)", opts.CensusType)
	}

	kv := prefixeddb.NewPrefixedDatabase(opts.ParentDB, []byte(opts.Name))
	t, err := tree.New(nil, tree.Options{DB: kv, MaxLevels: maxLevels, HashFunc: hashFunc})
	if err != nil {
		return nil, err
	}

	cTree := &Tree{
		tree:       t,
		censusType: opts.CensusType,
		hashFunc:   hashFunc.Hash,
		hashLen:    hashFunc.Len(),
	}

	// ensure census index is created
	wTx := cTree.tree.DB().WriteTx()
	defer wTx.Discard()
	return cTree, wTx.Commit()
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
func (*Tree) BytesToBigInt(data []byte) *big.Int {
	return arbo.BytesLEToBigInt(data)
}

// BigIntToBytes marshals a bigInt following the censusTree encoding rules
func (t *Tree) BigIntToBytes(b *big.Int) []byte {
	return arbo.BigIntToBytesLE(t.hashLen, b)
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

// Root wraps tree.Tree.Root.
func (t *Tree) Root() ([]byte, error) {
	return t.tree.Root(nil)
}

// Close closes the database for the tree.
func (t *Tree) Close() error {
	return t.tree.DB().Close()
}

// Get wraps tree.Tree.Get.
func (t *Tree) Get(key []byte) ([]byte, error) {
	return t.tree.Get(nil, key)
}

// VerifyProof verifies a census proof.
// If the census is indexed key can be nil (value provides the key already).
// If root is nil the last merkle root is used for verify.
func (t *Tree) VerifyProof(key, value, proof, root []byte) error {
	var err error
	if root == nil {
		root, err = t.Root()
		if err != nil {
			return fmt.Errorf("cannot get tree root: %w", err)
		}
	}
	// If the provided key is longer than the defined maximum length truncate it
	// TODO: return an error if the other key lengths are deprecated
	leafKey := key
	if len(leafKey) > DefaultMaxKeyLen {
		leafKey = leafKey[:DefaultMaxKeyLen]
	}
	if err := t.tree.VerifyProof(leafKey, value, proof, root); err != nil {
		return err
	}
	return nil
}

// GenProof generates a census proof for the provided key.
// The returned values are `value` and `siblings`.
// If the census is indexed, value will be equal to key.
func (t *Tree) GenProof(key []byte) ([]byte, []byte, error) {
	// If the provided key is longer than the defined maximum length truncate it
	// TODO: return an error if the other key lengths are deprecated
	leafKey := key
	if len(leafKey) > DefaultMaxKeyLen {
		leafKey = leafKey[:DefaultMaxKeyLen]
	}
	return t.tree.GenProof(nil, leafKey)
}

// Size returns the census index (number of added leafs to the merkle tree).
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
	t.updatesLock.Lock()
	defer t.updatesLock.Unlock()
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
	if len(keys) != len(values) {
		return nil, fmt.Errorf("keys and values must have the same length")
	}
	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()
	var invalids []int
	weight := big.NewInt(0)
	for i := 0; i < len(keys); i++ {
		if err := t.tree.Add(wTx, keys[i], values[i]); err != nil {
			log.Debugw("could not add key to census",
				"key", hex.EncodeToString(keys[i]),
				"value", hex.EncodeToString(values[i]),
				"err", err)
			invalids = append(invalids, i)
			continue
		}
		weight = new(big.Int).Add(weight, t.BytesToBigInt(values[i]))
	}
	if err := t.updateCensusWeight(wTx, t.BigIntToBytes(weight)); err != nil {
		return nil, err
	}
	return invalids, wTx.Commit()
}

// Add adds a new key and value to the census merkle tree.
// The key must ideally be hashed with the tree function. If not, caller must
// ensure the key is inside the hashing function field.
// The value is considered the weight for the voter. So a serialized big.Int()
// is expected. Value must be inside the hashing function field too.
// If the census is indexed (indexAsKeysCensus), the value must be nil.
func (t *Tree) Add(key, value []byte) error {
	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()

	if err := t.tree.Add(wTx, key, value); err != nil {
		return fmt.Errorf("cannot add (%x) to census: %w", key, err)
	}

	// The censusWeight update should be done only for the
	// censuses that have weight.
	if value != nil {
		if err := t.updateCensusWeight(wTx, value); err != nil {
			return err
		}
	}

	return wTx.Commit()
}

// ImportDump wraps t.tree.ImportDump while acquiring the lock.
func (t *Tree) ImportDump(b []byte) error {
	if err := t.tree.ImportDump(b); err != nil {
		return fmt.Errorf("could not import dump: %w", err)
	}

	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()

	// get the total addedWeight from parsing the dump byte array, and
	// adding the weight of its values.
	// The weight is only updated on census that have a weight value.
	addedWeight := big.NewInt(0)
	if err := t.tree.IterateLeaves(nil, func(key, value []byte) bool {
		// add the weight (value of the leaf)
		addedWeight = new(big.Int).Add(addedWeight, t.BytesToBigInt(value))
		return false
	}); err != nil {
		return fmt.Errorf("could not add weight: %w", err)
	}

	if err := t.updateCensusWeight(wTx, t.BigIntToBytes(addedWeight)); err != nil {
		return fmt.Errorf("could not update census weight: %w", err)
	}

	if err := wTx.Commit(); err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	return nil
}

// GetCensusWeight returns the current weight of the census.
func (t *Tree) GetCensusWeight() (*big.Int, error) {
	t.updatesLock.RLock()
	defer t.updatesLock.RUnlock()
	weight, err := t.tree.DB().Get(censusWeightKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return big.NewInt(0), nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get census weight: %w", err)
	}

	return t.BytesToBigInt(weight), nil
}
