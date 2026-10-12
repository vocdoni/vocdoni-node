package apiclient

import (
	"math/big"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

// CensusProof represents proof for a voter in a census
type CensusProof struct {
	// Root contains census tree root
	Root types.HexBytes
	// Proof contains encoded and packed siblings as a census proof
	Proof types.HexBytes
	// LeafValue contains the associated value on the census tree
	LeafValue types.HexBytes
	// LeafWeight contains the decoded LeafValue as big.Int that represents
	// the associated weight for the voter.
	LeafWeight *big.Int
	// KeyType defines the type of the census tree key
	KeyType models.ProofArbo_KeyType
	// Siblings contains the decoded siblings keys
	Siblings []string
}
