// Package saltedkey derives the salted CSP keys used by the salted vote proofs
// (models.ProofCA_ECDSA_PIDSALTED and models.ProofCA_ECDSA_BLIND_PIDSALTED).
//
// A salted proof binds a CSP authorization to one election: the CSP signs with
// the private key d' = (d + salt) mod n, and the chain verifies against the
// public key Q' = Q + salt*G, where both sides derive salt independently from
// the election. See Salt for the derivation and its rationale.
package saltedkey

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	blind "github.com/arnaucube/go-blindsecp256k1"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// SaltSize is the size (in bytes) of the salt word
const SaltSize = 20

// MaxVoteWeightBits is the maximum bit length accepted for a CSP-authorized
// vote weight by Salt.
const MaxVoteWeightBits = SaltSize * 8

// ErrVoteWeightOverflow is returned by Salt when the vote weight does not fit
// in MaxVoteWeightBits bits.
var ErrVoteWeightOverflow = errors.New("vote weight exceeds the maximum salt size")

// weightEncodingBytes is the fixed width the vote weight is rendered to before
// hashing, so every byte encoding of the same integer yields the same salt.
const weightEncodingBytes = 32

// Salt derives the salt applied to the CSP key for a salted proof
// (models.ProofCA_ECDSA_PIDSALTED and models.ProofCA_ECDSA_BLIND_PIDSALTED) of
// the given election and CSP-authorized vote weight. It is the derivation used
// by elections with census origin OFF_CHAIN_CA_V2; legacy OFF_CHAIN_CA
// elections use the raw processID as salt instead.
//
//	w    = big-endian unsigned integer of voteWeight; 1 when voteWeight is absent
//	         reject if w >= 2^160
//	salt = keccak256( processID || w-as-32-big-endian-bytes )[:20]
//
// The signer derives the matching private key as d' = (d + uint(salt)) mod n
// (SaltECDSAPrivKey, SaltBlindPrivKey); the verifier derives the matching public
// key as Q' = Q + uint(salt)*G (SaltECDSAPubKey, SaltBlindPubKey).
//
// Why the whole processID and the weight go through the hash: the processID
// layout is chainID[6] + orgAddr[20] + censusOrigin[1] + envType[1] + nonce[4],
// so its first 20 bytes — all the salting primitives consume — are chainID[6] +
// orgAddr[:14]. Passing the raw processID (the legacy derivation) means every
// election of the same organization on the same chain derives one salt, hence
// one salted keypair; because the CSP signs a message it cannot read in the
// blind flow, a voter authorized for one election can have a bundle for a
// sibling election blind-signed and it verifies there. Hashing spreads all 32
// bytes, including the nonce, over the salt, and binds the CSP-authorized
// weight into it too — a voter who alters bundle.voteWeight derives a different
// salt, so the signature no longer verifies.
//
// Why hash both inputs rather than add them. An earlier design used
// salt = keccak256(pid)[:20] + w mod 2^160. That is additively malleable: the
// verifier re-derives the salt from the voter-chosen w, so a voter holding a
// blind signature valid for saltA = keccak256(pidA)[:20] + 1 can present a
// bundle for election B declaring w' = (saltA - keccak256(pidB)[:20]) mod 2^160,
// making election B derive the identical salt. Both the cross-election and the
// weight binding collapse, with no crypto — the attacker just chooses the
// verifier's salt input. Hashing both inputs removes that handle: hitting a
// target salt needs a keccak preimage collision.
//
// Why a fixed 32-byte weight: bundle.voteWeight is variable-length, and 0x05 and
// 0x0005 both decode to (and are recorded on-chain as) the integer 5. Hashing
// the raw bytes would give two salts for one recorded weight — a malleability
// seam. Rendering the parsed integer to a fixed width collapses every encoding
// to one preimage, so the salt binds exactly the integer the chain records.
//
// Why an absent weight is treated as 1: an absent voteWeight and a voteWeight of
// 1 already yield the same on-chain weight, so they must yield the same salt.
//
// The MaxVoteWeightBits guard is a sane cap and keeps FillBytes safe (it would
// panic if w did not fit in weightEncodingBytes); it is not otherwise
// load-bearing. SaltSize stays 20, so the salting primitives and every
// already-deployed CSP keep working unchanged — only the hash preimage differs
// from the legacy derivation.
//
// There is no domain-separation tag: this is the only hashed derivation, and
// the salt is only ever consumed by the key-salting primitives in this package.
// A collision with a legacy salt (a raw processID prefix) cannot be exploited
// either way, because legacy and V2 processIDs differ at the censusOrigin byte
// and the bundle binds the full processID.
//
// Mirrored by vocdoni/blind-csp, vocdoni/saas-backend (csp/sign.go,
// csp/sign_blind.go) and vocdoni/vocdoni-sdk, keyed on the election's census
// origin.
func Salt(processID, voteWeight []byte) ([]byte, error) {
	weight := big.NewInt(1)
	if voteWeight != nil {
		weight = new(big.Int).SetBytes(voteWeight)
	}
	if weight.BitLen() > MaxVoteWeightBits {
		return nil, ErrVoteWeightOverflow
	}
	return ethcrypto.Keccak256(processID, weight.FillBytes(make([]byte, weightEncodingBytes)))[:SaltSize], nil
}

// SaltBlindPubKey returns the salted blind public key of pubKey applying the salt.
func SaltBlindPubKey(pubKey *blind.PublicKey, salt []byte) (*blind.PublicKey, error) {
	if len(salt) < SaltSize {
		return nil, fmt.Errorf("provided salt is not large enough (need %d bytes)", SaltSize)
	}
	if pubKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	var salt2 [SaltSize]byte
	copy(salt2[:], salt[:SaltSize])
	x, y := ethcrypto.S256().ScalarBaseMult(salt2[:])
	s := blind.Point{
		X: x,
		Y: y,
	}
	return (*blind.PublicKey)(pubKey.Point().Add(&s)), nil
}

// SaltECDSAPubKey returns the salted plain public key of pubKey applying the salt.
func SaltECDSAPubKey(pubKey *ecdsa.PublicKey, salt []byte) (*ecdsa.PublicKey, error) {
	if len(salt) < SaltSize {
		return nil, fmt.Errorf("provided salt is not large enough (need %d bytes)", SaltSize)
	}
	if pubKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	var salt2 [SaltSize]byte
	copy(salt2[:], salt[:SaltSize])
	x, y := pubKey.Curve.ScalarBaseMult(salt2[:])
	saltedX, saltedY := pubKey.Curve.Add(pubKey.X, pubKey.Y, x, y)
	return &ecdsa.PublicKey{Curve: pubKey.Curve, X: saltedX, Y: saltedY}, nil
}

// SaltBlindPrivKey returns the salted blind private key of privKey applying the
// salt. It is the signer-side counterpart of SaltBlindPubKey.
func SaltBlindPrivKey(privKey *blind.PrivateKey, salt []byte) (*blind.PrivateKey, error) {
	if privKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	d, err := saltScalar(privKey.BigInt(), salt)
	if err != nil {
		return nil, err
	}
	return (*blind.PrivateKey)(d), nil
}

// SaltECDSAPrivKey returns the salted plain private key of privKey applying the
// salt. It is the signer-side counterpart of SaltECDSAPubKey.
func SaltECDSAPrivKey(privKey *ecdsa.PrivateKey, salt []byte) (*ecdsa.PrivateKey, error) {
	if privKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	d, err := saltScalar(privKey.D, salt)
	if err != nil {
		return nil, err
	}
	return ethcrypto.ToECDSA(d.FillBytes(make([]byte, 32)))
}

// saltScalar returns (d + salt) mod n, the private-key tweak matching the
// Q + salt*G applied by the public-key salting functions. As they do, it
// consumes the first SaltSize bytes of salt.
func saltScalar(d *big.Int, salt []byte) (*big.Int, error) {
	if len(salt) < SaltSize {
		return nil, fmt.Errorf("provided salt is not large enough (need %d bytes)", SaltSize)
	}
	s := new(big.Int).SetBytes(salt[:SaltSize])
	return s.Add(s, d).Mod(s, ethcrypto.S256().Params().N), nil
}
