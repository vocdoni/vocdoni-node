// Package testcsp provides a CSP (Credential Service Provider) signer able to
// produce all four models.ProofCA types, including the salted variants.
//
// This repository only ever verifies CSP proofs (see
// vochain/transaction/proofs/cspproof); the real signer lives in
// vocdoni/blind-csp and vocdoni/saas-backend. This package is the signer side
// needed to test the verifier, and doubles as executable documentation of the
// derivation those services must mirror.
//
// Sign takes the salt explicitly rather than deriving it, so a test states which
// salt it uses: saltedkey.Salt(pid, weight) for a V2-origin election, the raw
// processID for a legacy OFF_CHAIN_CA one, or a deliberately mismatched value to build the
// cross-election and weight-tampering proofs that must be rejected.
package testcsp

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"

	blind "github.com/arnaucube/go-blindsecp256k1"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/saltedkey"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// blindSignAttempts bounds the retry loop in blindSign. blind.BlindSign rejects
// a k or a blinded message that does not serialize to exactly 32 bytes, each
// failing with probability ~1/256, so a single attempt is flaky.
const blindSignAttempts = 32

// Signer holds the CSP keys: an ECDSA one for the plain proof types and a blind
// secp256k1 one for the blind types.
type Signer struct {
	ECDSA *ethereum.SignKeys
	Blind *blind.PrivateKey
}

// NewSigner returns a Signer with freshly generated keys.
func NewSigner() (*Signer, error) {
	ecdsaKey := ethereum.NewSignKeys()
	if err := ecdsaKey.Generate(); err != nil {
		return nil, fmt.Errorf("cannot generate CSP ECDSA key: %w", err)
	}
	blindKey, err := blind.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("cannot generate CSP blind key: %w", err)
	}
	return &Signer{ECDSA: ecdsaKey, Blind: blindKey}, nil
}

// CensusRoot returns the value to store in process.CensusRoot for proofType.
//
// Note the root is always the *unsalted* public key: for the salted types the
// verifier applies the salt to the root itself.
func (s *Signer) CensusRoot(proofType models.ProofCA_Type) (types.HexBytes, error) {
	switch proofType {
	case models.ProofCA_ECDSA, models.ProofCA_ECDSA_PIDSALTED:
		return s.ECDSA.PublicKey(), nil
	case models.ProofCA_ECDSA_BLIND, models.ProofCA_ECDSA_BLIND_PIDSALTED:
		// the verifier feeds the root through ethereum.DecompressPubKey and then
		// blind.NewPublicKeyFromECDSA, so it must be an ethereum-serialized key,
		// not blind.PublicKey.Bytes() (which is little-endian and will not parse)
		return ethcrypto.CompressPubkey(blindPubToECDSA(s.Blind.Public())), nil
	default:
		return nil, fmt.Errorf("unsupported CSP proof type %s", proofType)
	}
}

// Bundle builds the CA bundle a voter presents. A nil weight leaves voteWeight
// absent, which the chain reads as a weight of 1.
func Bundle(pid, voterAddr []byte, weight *big.Int) *models.CAbundle {
	bundle := &models.CAbundle{
		ProcessId: pid,
		Address:   voterAddr,
	}
	if weight != nil {
		bundle.VoteWeight = weight.Bytes()
	}
	return bundle
}

// Sign issues a CSP proof over bundle, signing with the key salted by salt.
//
// salt is ignored for the unsalted proof types. For the salted ones it must be
// what the verifier will derive for this election, otherwise the proof is
// expected to fail: that is precisely what the negative tests assert.
func (s *Signer) Sign(proofType models.ProofCA_Type, bundle *models.CAbundle, salt []byte) (*models.ProofCA, error) {
	bundleBytes, err := proto.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal CSP bundle: %w", err)
	}

	var signature []byte
	switch proofType {
	case models.ProofCA_ECDSA:
		if signature, err = s.ECDSA.SignEthereum(bundleBytes); err != nil {
			return nil, fmt.Errorf("cannot sign CSP bundle: %w", err)
		}
	case models.ProofCA_ECDSA_PIDSALTED:
		saltedKey, err := s.saltedECDSA(salt)
		if err != nil {
			return nil, err
		}
		if signature, err = saltedKey.SignEthereum(bundleBytes); err != nil {
			return nil, fmt.Errorf("cannot sign CSP bundle with salted key: %w", err)
		}
	case models.ProofCA_ECDSA_BLIND:
		sig, err := blindSign(s.Blind, bundleBytes)
		if err != nil {
			return nil, err
		}
		signature = sig.BytesUncompressed()
	case models.ProofCA_ECDSA_BLIND_PIDSALTED:
		saltedKey, err := saltedkey.SaltBlindPrivKey(s.Blind, salt)
		if err != nil {
			return nil, fmt.Errorf("cannot salt CSP blind key: %w", err)
		}
		sig, err := blindSign(saltedKey, bundleBytes)
		if err != nil {
			return nil, err
		}
		signature = sig.BytesUncompressed()
	default:
		return nil, fmt.Errorf("unsupported CSP proof type %s", proofType)
	}

	return &models.ProofCA{
		Type:      proofType,
		Bundle:    bundle,
		Signature: signature,
	}, nil
}

// SignProof is Sign wrapped into the marshalled models.Proof a VoteEnvelope
// carries.
func (s *Signer) SignProof(proofType models.ProofCA_Type, bundle *models.CAbundle, salt []byte) (*models.Proof, error) {
	caProof, err := s.Sign(proofType, bundle, salt)
	if err != nil {
		return nil, err
	}
	return &models.Proof{Payload: &models.Proof_Ca{Ca: caProof}}, nil
}

// saltedECDSA returns a SignKeys holding the salted ECDSA private key.
func (s *Signer) saltedECDSA(salt []byte) (*ethereum.SignKeys, error) {
	saltedPriv, err := saltedkey.SaltECDSAPrivKey(&s.ECDSA.Private, salt)
	if err != nil {
		return nil, fmt.Errorf("cannot salt CSP ECDSA key: %w", err)
	}
	saltedKey := ethereum.NewSignKeys()
	if err := saltedKey.AddHexKey(hex.EncodeToString(ethcrypto.FromECDSA(saltedPriv))); err != nil {
		return nil, fmt.Errorf("cannot load salted CSP ECDSA key: %w", err)
	}
	return saltedKey, nil
}

// blindSign runs the three-leg blind protocol locally, playing both the CSP and
// the voter. The message is the raw keccak of the bundle, matching what
// cspproof.Verify feeds to blind.Verify (note: no Ethereum signing prefix here,
// unlike the plain ECDSA branch).
func blindSign(sk *blind.PrivateKey, msg []byte) (*blind.Signature, error) {
	m := new(big.Int).SetBytes(ethereum.HashRaw(msg))
	for range blindSignAttempts {
		k, r, err := blind.NewRequestParameters()
		if err != nil {
			return nil, fmt.Errorf("cannot create blind request parameters: %w", err)
		}
		mBlinded, userSecret, err := blind.Blind(m, r)
		if err != nil {
			continue // rejected draw, see blindSignAttempts
		}
		sBlind, err := sk.BlindSign(mBlinded, k)
		if err != nil {
			continue
		}
		return blind.Unblind(sBlind, userSecret), nil
	}
	return nil, fmt.Errorf("blind signature failed after %d attempts", blindSignAttempts)
}

// blindPubToECDSA reinterprets a blind public key as an ECDSA one on secp256k1,
// so it can be serialized with the go-ethereum helpers the verifier uses.
func blindPubToECDSA(pk *blind.PublicKey) *ecdsa.PublicKey {
	return &ecdsa.PublicKey{Curve: ethcrypto.S256(), X: pk.X, Y: pk.Y}
}
