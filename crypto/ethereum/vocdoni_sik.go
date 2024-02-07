package ethereum

import (
	"fmt"
	"math/big"

	"github.com/iden3/go-iden3-crypto/poseidon"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/util"
)

// SIKsignature signs the default vocdoni sik payload. It envolves the
// SignEthereum method, using the DefaultSIKPayload and discarding the last
// byte of the signature (used for recovery), different that the same byte of a
// signature generated with javascript.
func (k *SignKeys) SIKsignature() ([]byte, error) {
	sign, err := k.SignEthereum([]byte(DefaultSIKPayload))
	if err != nil {
		return nil, err
	}
	return sign[:SignatureLength-1], nil
}

// AccountSIK method generates the Secret Identity Key for the current SignKeys
// with the signature of the DefaultSIKPayload and the user secret (if it is
// provided) following the definition:
//
//	SIK = poseidon(address, signature, secret)
//
// The secret could be nil.
func (k *SignKeys) AccountSIK(secret []byte) ([]byte, error) {
	if secret == nil {
		secret = []byte{0}
	}
	sign, err := k.SIKsignature()
	if err != nil {
		return nil, fmt.Errorf("error signing sik payload: %w", err)
	}
	seed := []*big.Int{
		arbo.BytesToBigInt(k.Address().Bytes()),
		util.BigToFF(new(big.Int).SetBytes(secret)),
		util.BigToFF(new(big.Int).SetBytes(sign)),
	}
	hash, err := poseidon.Hash(seed)
	if err != nil {
		return nil, err
	}
	return arbo.BigIntToBytes(arbo.HashFunctionPoseidon.Len(), hash), nil
}

// AccountSIKnullifier method composes the nullifier of the current SignKeys
// for the desired election id and the secret provided.
func (k *SignKeys) AccountSIKnullifier(electionID, secret []byte) ([]byte, error) {
	// sign the default Secret Identity Key seed
	sign, err := k.SIKsignature()
	if err != nil {
		return nil, fmt.Errorf("error signing default sik seed: %w", err)
	}
	// get the representation of the signature on the finite field and repeat
	// the same with the secret if it is provided, if not add a zero
	seed := []*big.Int{util.BigToFF(new(big.Int).SetBytes(sign))}
	if secret != nil {
		seed = append(seed, util.BigToFF(new(big.Int).SetBytes(secret)))
	} else {
		seed = append(seed, big.NewInt(0))
	}
	// encode the election id for circom and include it into the nullifier
	seed = append(seed, util.BytesToArboSplit(electionID)...)
	// calculate the poseidon image --> H(signature + secret + electionId)
	hash, err := poseidon.Hash(seed)
	if err != nil {
		return nil, err
	}
	return hash.Bytes(), nil
}
