package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"sync"

	vapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

// localCensuses holds the census trees the tests vote with. The node does not
// host census trees, so the tests build them locally and only send the root on
// chain; the voters' proofs are generated from these local trees.
var localCensuses = &censusStore{trees: map[string]*censustree.Tree{}}

type censusStore struct {
	mu    sync.Mutex
	db    db.Database
	trees map[string]*censustree.Tree // by census ID and, once published, by root
}

// newCensus creates an empty census tree of the given type and returns its ID.
func (s *censusStore) newCensus(censusType string) (types.HexBytes, error) {
	var treeType models.Census_Type
	switch censusType {
	case vapi.CensusTypeWeighted:
		treeType = models.Census_ARBO_BLAKE2B
	case vapi.CensusTypeZKWeighted:
		treeType = models.Census_ARBO_POSEIDON
	default:
		return nil, fmt.Errorf("unsupported census type %q", censusType)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		dir, err := os.MkdirTemp("", "end2endtest-census")
		if err != nil {
			return nil, fmt.Errorf("cannot create census dir: %w", err)
		}
		if s.db, err = metadb.New(db.TypePebble, dir); err != nil {
			return nil, fmt.Errorf("cannot open census db: %w", err)
		}
	}
	censusID := types.HexBytes(util.RandomBytes(32))
	tree, err := censustree.New(censustree.Options{
		Name:       censusID.String(),
		ParentDB:   s.db,
		MaxLevels:  censustree.DefaultMaxLevels,
		CensusType: treeType,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create census tree: %w", err)
	}
	s.trees[censusID.String()] = tree
	return censusID, nil
}

func (s *censusStore) tree(id types.HexBytes) (*censustree.Tree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tree, ok := s.trees[id.String()]
	if !ok {
		return nil, fmt.Errorf("census %s not found", id)
	}
	return tree, nil
}

// leafKey returns the tree key of a voter: the address itself for zk censuses,
// and its truncated hash otherwise.
func leafKey(tree *censustree.Tree, voterKey []byte) ([]byte, error) {
	if tree.Type() == models.Census_ARBO_POSEIDON {
		return voterKey, nil
	}
	key, err := tree.Hash(voterKey)
	if err != nil {
		return nil, fmt.Errorf("cannot hash census key: %w", err)
	}
	return key[:censustree.DefaultMaxKeyLen], nil
}

// addParticipants adds the voter keys with the given weight to a census.
func (s *censusStore) addParticipants(censusID types.HexBytes, voterKeys [][]byte, weight *big.Int) error {
	tree, err := s.tree(censusID)
	if err != nil {
		return err
	}
	keys := make([][]byte, 0, len(voterKeys))
	values := make([][]byte, 0, len(voterKeys))
	for _, voterKey := range voterKeys {
		key, err := leafKey(tree, voterKey)
		if err != nil {
			return err
		}
		keys = append(keys, key)
		values = append(values, tree.BigIntToBytes(weight))
	}
	failed, err := tree.AddBatch(keys, values)
	if err != nil {
		return fmt.Errorf("cannot add participants: %w", err)
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not add %d participants", len(failed))
	}
	return nil
}

// size returns the number of participants of a census.
func (s *censusStore) size(censusID types.HexBytes) (uint64, error) {
	tree, err := s.tree(censusID)
	if err != nil {
		return 0, err
	}
	return tree.Size()
}

// publish returns the root of a census and makes it addressable by that root.
func (s *censusStore) publish(censusID types.HexBytes) (types.HexBytes, error) {
	tree, err := s.tree(censusID)
	if err != nil {
		return nil, err
	}
	root, err := tree.Root()
	if err != nil {
		return nil, fmt.Errorf("cannot get census root: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trees[hex.EncodeToString(root)] = tree
	return root, nil
}

// genProof generates the census proof of a voter in a published census.
func (s *censusStore) genProof(censusRoot, voterKey types.HexBytes) (*apiclient.CensusProof, error) {
	tree, err := s.tree(censusRoot)
	if err != nil {
		return nil, err
	}
	key, err := leafKey(tree, voterKey)
	if err != nil {
		return nil, err
	}
	value, proof, err := tree.GenProof(key)
	if err != nil {
		return nil, fmt.Errorf("cannot generate census proof: %w", err)
	}
	cp := &apiclient.CensusProof{
		Root:       censusRoot,
		Proof:      proof,
		LeafValue:  value,
		LeafWeight: new(big.Int).SetUint64(1),
	}
	if len(value) > 0 {
		cp.LeafWeight = tree.BytesToBigInt(value)
	}
	return cp, nil
}
