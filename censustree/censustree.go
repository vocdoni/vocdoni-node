package censustree

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/proto/build/go/models"
)

var (
	censusWeightKey         = []byte("censusWeight")
	censusIndexKey          = []byte("censusIndex")
	isIndexAsKeysCensus     = []byte("isIndexAsKey")
	censusKeysToIndexPrefix = []byte("keyIndex")
)

// Tree implements the Merkle Tree used for census
// Concurrent updates to the tree.Tree can lead to losing some of the updates,
// so we don't expose the tree.Tree directly, and lock it in every method that
// updates it.
type Tree struct {
	tree *tree.Tree

	sync.Mutex
	public            atomic.Bool
	censusType        models.Census_Type
	hashFunc          func(...[]byte) ([]byte, error)
	hashLen           int
	updatesLock       sync.RWMutex
	indexAsKeysCensus bool
}

type Options struct {
	// ParentDB is the Database under which all censuses are stored, each
	// with a different prefix.
	ParentDB          db.Database
	Name              string
	MaxLevels         int
	CensusType        models.Census_Type
	IndexAsKeysCensus bool
}

// By default, the maximum number of levels will be 256, which allows to add
// up to 2^256 leaves to the tree, with keys with up to 32 bytes. However the
// number of levels could be setted up during the tree initialization which
// allows to support uses cases with specific restrictions such as the
// zkweighted census, which needs to optimize some artifacts size that depends
// of the number of levels of the tree.
const DefaultMaxLevels = 256

// DeleteCensusTreeFromDatabase removes all the database entries for the census identified by name.
// Caller must take care of potential data races, the census must be closed before calling this method.
// Returns the number of removed items.
func DeleteCensusTreeFromDatabase(kv db.Database, name string) (int, error) {
	database := prefixeddb.NewPrefixedDatabase(kv, []byte(name))
	wTx := database.WriteTx()
	i := 0
	if err := database.Iterate(nil, func(k, v []byte) bool {
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
	var maxLevels = DefaultMaxLevels
	var hashFunc arbo.HashFunction
	switch opts.CensusType {
	case models.Census_ARBO_BLAKE2B:
		hashFunc = arbo.HashFunctionBlake2b
	case models.Census_ARBO_POSEIDON:
		// If an Arbo census tree is based on Poseidon hash to use it with a
		// circom circuit, it is necessary to increase the number of levels by one
		// (unless it has the maximum number of levels). This is done to
		// emulate the same behaviour than Circom SMT library, which increases
		// the desired number of levels by one.
		if opts.MaxLevels < maxLevels {
			maxLevels = opts.MaxLevels + 1
		}
		hashFunc = arbo.HashFunctionPoseidon
	default:
		return nil, fmt.Errorf("unrecognized census type (%d)", opts.CensusType)
	}

	kv := prefixeddb.NewPrefixedDatabase(opts.ParentDB, []byte(opts.Name))
	t, err := tree.New(nil, tree.Options{DB: kv, MaxLevels: maxLevels, HashFunc: hashFunc})
	if err != nil {
		return nil, err
	}

	// indexAsKeys option can only be set to true in the creation time (first time New is invoked for a census Name).
	// A database byte is set to 0xFF if the census is indexAsKeys type.
	// When the tree is loaded, there is a check to ensure the tree was created with indexAsKeys enabled if the option
	// is set to true. The option is automatically set to true if the byte is set to 0xFF.
	if indexAsKeysBytes, err := kv.ReadTx().Get(isIndexAsKeysCensus); errors.Is(err, db.ErrKeyNotFound) {
		wTx := kv.WriteTx()
		defer wTx.Commit()
		if opts.IndexAsKeysCensus {
			if err := wTx.Set(isIndexAsKeysCensus, []byte{0xFF}); err != nil {
				return nil, err
			}
		} else {
			if err := wTx.Set(isIndexAsKeysCensus, []byte{0x00}); err != nil {
				return nil, err
			}
		}
	} else if err != nil {
		return nil, err
	} else {
		if opts.IndexAsKeysCensus && bytes.Equal(indexAsKeysBytes, []byte{0x00}) {
			return nil, fmt.Errorf("census tree was not created with indexAsKeys option enabled")
		}
		opts.IndexAsKeysCensus = bytes.Equal(indexAsKeysBytes, []byte{0xFF})
	}

	cTree := &Tree{
		tree:              t,
		censusType:        opts.CensusType,
		hashFunc:          hashFunc.Hash,
		hashLen:           hashFunc.Len(),
		indexAsKeysCensus: opts.IndexAsKeysCensus}

	// ensure census index is created
	wTx := cTree.tree.DB().WriteTx()
	defer wTx.Discard()
	_, err = cTree.updateCensusIndex(wTx, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot update census index: %w", err)
	}
	return cTree, wTx.Commit()
}

// Type returns the numeric identifier of the censustree implementation
func (t *Tree) Type() models.Census_Type {
	return t.censusType
}

// IsIndexed returns true if the census uses index as keys.
func (t *Tree) IsIndexed() bool {
	return t.indexAsKeysCensus
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
// call Publish() before considering the Tree available.
func (t *Tree) Publish() { t.public.Store(true) }

// Unpublish makes a merkle tree not available for queries.
func (t *Tree) Unpublish() { t.public.Store(false) }

// IsPublic returns true if the tree is available.
func (t *Tree) IsPublic() bool { return t.public.Load() }

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
func (t *Tree) VerifyProof(key, value, proof, root []byte) (bool, error) {
	var err error
	if root == nil {
		root, err = t.Root()
		if err != nil {
			return false, fmt.Errorf("cannot get tree root: %w", err)
		}
	}
	indexedKey := key
	if t.indexAsKeysCensus {
		indexedKey, err = t.KeyToIndex(value)
		if err != nil {
			return false, fmt.Errorf("cannot get index key")
		}
	}
	return t.tree.VerifyProof(indexedKey, value, proof, root)
}

// GenProof generates a census proof for the provided key.
// The returned values are `value` and `siblings`.
// If the census is indexed, value will be equal to key.
func (t *Tree) GenProof(key []byte) ([]byte, []byte, error) {
	if t.indexAsKeysCensus {
		index, err := t.KeyToIndex(key)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot get index key")
		}
		return t.tree.GenProof(nil, index)
	}
	return t.tree.GenProof(nil, key)
}

// Size returns the census index (number of added leafs to the merkle tree).
func (t *Tree) Size() (uint64, error) {
	return t.tree.Size(t.tree.DB().ReadTx())
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

// updateCensusIndex increments the census tree index by delta and return the previous value.
func (t *Tree) updateCensusIndex(wTx db.WriteTx, delta uint32) (uint64, error) {
	t.updatesLock.Lock()
	defer t.updatesLock.Unlock()
	indexBytes, err := wTx.Get(censusIndexKey)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return 0, fmt.Errorf("could not get census index: %w", err)
	}
	currentIndex := big.NewInt(0)
	if indexBytes != nil {
		currentIndex = t.BytesToBigInt(indexBytes)
	}
	index := new(big.Int).Add(currentIndex, big.NewInt(int64(delta)))
	if err := wTx.Set(censusIndexKey, t.BigIntToBytes(index)); err != nil {
		return 0, fmt.Errorf("could not set census index: %w", err)
	}
	return currentIndex.Uint64(), nil
}

// AddBatch wraps t.tree.AddBatch while acquiring the lock.
func (t *Tree) AddBatch(keys, values [][]byte) ([]int, error) {
	t.Lock()
	defer t.Unlock()
	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()

	var newKeys, newValues [][]byte

	// if index as keys enabled, we need to rewrite keys and values
	if t.indexAsKeysCensus {
		if values != nil {
			return nil, fmt.Errorf("index as keys is enabled for this census tree, values are not allowed")
		}
		index, err := t.GetCensusIndex()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			indexBytes := [8]byte{}
			binary.LittleEndian.PutUint32(indexBytes[:], index)
			newKeys = append(newKeys, indexBytes[:])
			newValues = append(newValues, k)
			// store the key -> index relation
			if err := t.indexKey(k, indexBytes, wTx); err != nil {
				return nil, err
			}
			index++
		}
	} else {
		newKeys = keys
		newValues = values
	}

	invalids, err := t.tree.AddBatch(wTx, newKeys, newValues)
	if err != nil {
		return invalids, fmt.Errorf("addBatch failed: %w", err)
	}

	// update the census index
	if _, err := t.updateCensusIndex(wTx, uint32(len(keys)-len(invalids))); err != nil {
		return nil, err
	}

	// The census weight update should be done only for the
	// censuses that have weight. Other kind of censuses (eg.
	// AnonymousVoting) do not use the value parameter of the leaves as
	// weight, as it's used for other stuff, so the CensusWeight in those
	// cases would get wrong values.

	if values != nil && !t.indexAsKeysCensus {
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
	}

	return invalids, wTx.Commit()
}

// Add adds a new key and value to the census merkle tree.
// The key must ideally be hashed with the tree function. If not, caller must
// ensure the key is inside the hashing function field.
// The value is considered the weight for the voter. So a serialzied big.Int()
// is expected. Value must be inside the hasing function field too.
// If the census is indexed (indexAsKeysCensus), the value must be nil.
func (t *Tree) Add(key, value []byte) error {
	t.Lock()
	defer t.Unlock()

	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()

	index, err := t.updateCensusIndex(wTx, 1)
	if err != nil {
		return err
	}

	// Add key to census
	if t.indexAsKeysCensus {
		// if index as keys enabled, we store the last index as key and the provided key as value
		if value != nil {
			return fmt.Errorf("index as keys is enabled for this census tree, values are not allowed")
		}
		indexBytes := [8]byte{}
		binary.LittleEndian.PutUint64(indexBytes[:], index)
		if err := t.tree.Add(wTx, indexBytes[:], key); err != nil {
			return fmt.Errorf("cannot add (%x) to census: %w", key, err)
		}
		if err := t.indexKey(key, indexBytes, wTx); err != nil {
			return err
		}
	} else {
		if err := t.tree.Add(wTx, key, value); err != nil {
			return fmt.Errorf("cannot add (%x) to census: %w", key, err)
		}
	}

	// The censusWeight update should be done only for the
	// censuses that have weight. Other kind of censuses (eg.
	// AnonymousVoting) do not use the value parameter of the leaves as
	// weight, as it's used for other stuff, so the CensusWeight in those
	// cases would get wrong values.
	if value != nil && !t.indexAsKeysCensus {
		if err := t.updateCensusWeight(wTx, value); err != nil {
			return err
		}
	}

	return wTx.Commit()
}

// ImportDump wraps t.tree.ImportDump while acquiring the lock.
func (t *Tree) ImportDump(b []byte) error {
	t.Lock()
	defer t.Unlock()

	if err := t.tree.ImportDump(b); err != nil {
		return fmt.Errorf("could not import dump: %w", err)
	}

	wTx := t.tree.DB().WriteTx()
	defer wTx.Discard()

	if t.indexAsKeysCensus {
		if err := t.fillKeyToIndex(wTx); err != nil {
			return fmt.Errorf("could not generate key to index mapping")
		}
	} else {
		// get the total addedWeight from parsing the dump byte array, and
		// adding the weight of its values.
		// The weight is only updated on census that have a weight value.
		addedWeight := big.NewInt(0)
		if err := t.tree.IterateLeaves(t.tree.DB().ReadTx(), func(key, value []byte) bool {
			// add the weight (value of the leaf)
			addedWeight = new(big.Int).Add(addedWeight, t.BytesToBigInt(value))
			return false
		}); err != nil {
			return fmt.Errorf("could not add weight: %w", err)
		}

		if err := t.updateCensusWeight(wTx, t.BigIntToBytes(addedWeight)); err != nil {
			return fmt.Errorf("could not update census weight: %w", err)
		}
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
	weight, err := t.tree.DB().ReadTx().Get(censusWeightKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return big.NewInt(0), nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get census weight: %w", err)
	}

	return t.BytesToBigInt(weight), nil
}

// GetCensusIndex returns the current index of the census (number of elements).
func (t *Tree) GetCensusIndex() (uint32, error) {
	t.updatesLock.RLock()
	defer t.updatesLock.RUnlock()
	index, err := t.tree.DB().ReadTx().Get(censusIndexKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("could not get census index: %w", err)
	}

	return uint32(t.BytesToBigInt(index).Uint64()), nil
}

// KeyToIndex resolves the index of the key census.
// The returned index is 64 bit little endian encoded and can
// be used to query the tree.  This method is only available for
// IdexAsKeys censuses.
func (t *Tree) KeyToIndex(key []byte) ([]byte, error) {
	if !t.indexAsKeysCensus {
		return nil, fmt.Errorf("census is not indexed as keys")
	}
	tx := t.tree.DB().ReadTx()
	defer tx.Discard()
	return tx.Get(append(censusKeysToIndexPrefix, key...))
}

// indexKey stores a new entry for the key -> index relation.
// Must be only used if IndexAsKeys option is enabled
func (t *Tree) indexKey(key []byte, index [8]byte, wTx db.WriteTx) error {
	return wTx.Set(append(censusKeysToIndexPrefix, key...), index[:])
}

// fillKeyToIndex fills the mapping of key -> index.
// This function should be called when importing IndexAsKeys census.
// Census tree as we expect it to use a sequential index for the key, and a
// public key for the value.  Having this mapping will allow us to resolve the
// index given a public key.  The censusIndex value is also updated and stored.
func (t *Tree) fillKeyToIndex(tx db.WriteTx) error {
	type IndexKey struct {
		IndexLE [8]byte
		Key     []byte
	}
	indexKeys := make([]IndexKey, 0)
	t.IterateLeaves(func(indexLE, key []byte) bool {
		i := [8]byte{}
		copy(i[:], indexLE)
		indexKeys = append(indexKeys, IndexKey{IndexLE: i, Key: key})
		return false
	})
	for _, indexKey := range indexKeys {
		if err := t.indexKey(indexKey.Key, indexKey.IndexLE, tx); err != nil {
			return fmt.Errorf("error storing census key index by key: %w", err)
		}
	}
	// update the census index (number of leafs)
	currentIndex, err := t.GetCensusIndex()
	if err != nil {
		return err
	}
	if _, err := t.updateCensusIndex(tx, uint32(len(indexKeys)-int(currentIndex))); err != nil {
		return err
	}
	return nil
}

// GetCircomSiblings wraps the Arbo tree GetCircomSiblings function
func (t *Tree) GetCircomSiblings(key []byte) ([]string, error) {
	return t.tree.GetCircomSiblings(key)
}
