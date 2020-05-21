// Package snarks implements hashing used for ZK-Snarks.
package snarks

import (
	i3utils "github.com/iden3/go-iden3-core/merkletree"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"gitlab.com/vocdoni/go-dvote/crypto"
)

// Also ensure that we implement the interface.

// Poseidon computes the Poseidon hash of inputs.
var Poseidon crypto.Hash = poseidonImpl{}

type poseidonImpl struct{}

func (poseidonImpl) Hash(message []byte) []byte {
	hashNum, err := poseidon.HashBytes(message)
	if err != nil {
		// This error should never happen; they only check for the
		// validity of the input bigints, but we don't pass in any
		// ourselves.
		panic(err)
	}
	return i3utils.BigIntToHash(hashNum).Bytes()
}
