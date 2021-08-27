package factory

import (
	"fmt"

	"github.com/vocdoni/arbo"
	censustree "go.vocdoni.io/dvote/censustreelegacy"
	"go.vocdoni.io/dvote/censustreelegacy/arbotree"
	"go.vocdoni.io/dvote/censustreelegacy/gravitontree"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	// TreeUnknown is the default value used for censusTree implementations
	// which are not part of the this factory.
	TreeTypeUnknown = models.Census_UNKNOWN
	// TreeTypeArboBlake2b defines a tree type that uses arbo with Blake2b
	// hash function. Is thought for being used when computation speed is
	// important.
	TreeTypeArboBlake2b = models.Census_ARBO_BLAKE2B
	// TreeTypeArboPoseidon defines a tree type that uses arbo with Poseidon
	// hash function. Is thought for being used when zkSNARK compatibility
	// is needed (with circomlib).
	TreeTypeArboPoseidon = models.Census_ARBO_POSEIDON
	// TreeTypeGraviton defines a tree type that uses graviton tree.
	TreeTypeGraviton = models.Census_GRAVITON
)

// TMP to be defined the production circuit nLevels
const nLevels = 32

// NewCensusTree creates a merkle tree using the given storage and hash
// function. Note that each tree should use an entirely separate namespace for
// its database keys.
func NewCensusTree(treeType models.Census_Type, name, storageDir string) (censustree.Tree, error) {
	var err error
	var tree censustree.Tree
	switch treeType {
	case models.Census_ARBO_BLAKE2B:
		if tree, err =
			arbotree.NewTree(name, storageDir, nLevels, arbo.HashFunctionBlake2b); err != nil {
			return nil, err
		}
	case models.Census_ARBO_POSEIDON:
		if tree, err =
			arbotree.NewTree(name, storageDir, nLevels, arbo.HashFunctionPoseidon); err != nil {
			return nil, err
		}
	case models.Census_GRAVITON:
		if tree, err = gravitontree.NewTree(name, storageDir); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unrecognized tree type (%d)", treeType)
	}

	return tree, nil
}
