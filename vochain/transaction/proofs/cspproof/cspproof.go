package cspproof

import (
	"bytes"
	"fmt"
	"math/big"

	blind "github.com/arnaucube/go-blindsecp256k1"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/saltedkey"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// ProofVerifierCSP defines the interface for CSP (Credential Service Provider) proof verification systems.
type ProofVerifierCSP struct {
	// ChainID of the vochain, used to resolve the per-election activation of the
	// fixed salt derivation. An empty or unknown ChainID keeps the legacy one.
	ChainID string
}

// salt returns the salt applied to the CSP census root for this election.
//
// Before the fork point the salt is the raw processID, of which the salting
// primitives consume saltedkey.SaltSize (20) bytes: chainID[6] + orgAddr[:14].
// The nonce is cropped, so every election of the same organization on the same
// chain shares one salted key (issue #1424). Kept verbatim for elections that
// started before the fork, so their in-flight proofs stay valid.
//
// After it, the salt covers the whole processID and the CSP-authorized vote
// weight. See saltedkey.Salt.
func (v *ProofVerifierCSP) salt(process *models.Process, envelope *models.VoteEnvelope,
	bundle *models.CAbundle,
) ([]byte, error) {
	if !genesis.CSPSaltedProofV2Active(v.ChainID, process.StartTime) {
		return envelope.ProcessId, nil
	}
	return saltedkey.Salt(envelope.ProcessId, bundle.GetVoteWeight())
}

// VerifyProof verifies a proof with census origin OFF_CHAIN_CA.
// Returns verification result and weight.
func (v *ProofVerifierCSP) Verify(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error) {
	key := vID.Address()
	censusRoot := process.CensusRoot
	p := envelope.Proof.GetCa()
	if p == nil || p.Bundle == nil {
		return false, nil, fmt.Errorf("CSP proof or bundle are nil")
	}
	if !bytes.Equal(p.Bundle.Address, key) {
		return false, nil, fmt.Errorf(
			"CSP bundle address and key do not match: %x != %x", key, p.Bundle.Address)
	}
	if !bytes.Equal(p.Bundle.ProcessId, envelope.ProcessId) {
		return false, nil, fmt.Errorf("CSP bundle processID does not match")
	}
	cspBundle, err := proto.Marshal(p.Bundle)
	if err != nil {
		return false, nil, fmt.Errorf("cannot marshal CSP bundle to protobuf: %w", err)
	}

	// depending on signature type, use a mechanism for extracting the ca publickey from signature
	switch p.GetType() {
	case models.ProofCA_ECDSA, models.ProofCA_ECDSA_PIDSALTED:
		bundlePub, err := ethereum.PubKeyFromSignature(cspBundle, p.GetSignature())
		if err != nil {
			return false, nil, fmt.Errorf("cannot fetch CSP public key from signature: %w", err)
		}
		if p.GetType() == models.ProofCA_ECDSA_PIDSALTED {
			rootPub, err := ethereum.DecompressPubKey(censusRoot)
			if err != nil {
				return false, nil, fmt.Errorf("cannot decompress CSP public key: %w", err)
			}
			rootPubSalted, err := ethcrypto.UnmarshalPubkey(rootPub)
			if err != nil {
				return false, nil, fmt.Errorf("cannot unmarshal ECDSA CSP public key: %w", err)
			}
			salt, err := v.salt(process, envelope, p.Bundle)
			if err != nil {
				return false, nil, err
			}
			rootPubSalted, err = saltedkey.SaltECDSAPubKey(rootPubSalted, salt)
			if err != nil {
				return false, nil, fmt.Errorf("cannot salt ECDSA public key: %w", err)
			}
			censusRoot = ethcrypto.FromECDSAPub(rootPubSalted)
			// if salted, pubKey should be decompressed
			if bundlePub, err = ethereum.DecompressPubKey(bundlePub); err != nil {
				return false, nil, fmt.Errorf("unable to decompress proof pub key: %w", err)
			}
		}
		if !bytes.Equal(bundlePub, censusRoot) {
			return false, nil, fmt.Errorf("CSP bundle signature does not match")
		}
	case models.ProofCA_ECDSA_BLIND, models.ProofCA_ECDSA_BLIND_PIDSALTED:
		// Blind CSP check
		rootPubdesc, err := ethereum.DecompressPubKey(censusRoot)
		if err != nil {
			return false, nil, fmt.Errorf("cannot decompress CSP public key: %w", err)
		}
		rootPub, err := blind.NewPublicKeyFromECDSA(rootPubdesc)
		if err != nil {
			return false, nil, fmt.Errorf("cannot compute blind CSP public key: %w", err)
		}
		signature, err := blind.NewSignatureFromBytesUncompressed(p.GetSignature())
		if err != nil {
			return false, nil, fmt.Errorf("cannot compute blind CSP signature: %w", err)
		}
		// If pid salted, apply the salt (processId) to the censusRoot public key
		if p.GetType() == models.ProofCA_ECDSA_BLIND_PIDSALTED {
			salt, err := v.salt(process, envelope, p.Bundle)
			if err != nil {
				return false, nil, err
			}
			rootPub, err = saltedkey.SaltBlindPubKey(rootPub, salt)
			if err != nil {
				return false, nil, fmt.Errorf("cannot salt blind pubkey: %w", err)
			}
		}
		if !blind.Verify(new(big.Int).SetBytes(ethereum.HashRaw(cspBundle)), signature, rootPub) {
			return false, nil, fmt.Errorf("blind CSP verification failed for "+
				"pid %x with CSP key %x. CSP bundle {pid:%x, addr:%x, weight:%x}. CSP signature %x",
				envelope.ProcessId, rootPub.Bytes(), p.Bundle.ProcessId, p.Bundle.Address, p.Bundle.VoteWeight,
				signature.Bytes())
		}
	default:
		return false, nil, fmt.Errorf("CSP proof %s type not supported", p.Type)
	}
	if p.Bundle.GetVoteWeight() == nil {
		return true, big.NewInt(1), nil // since VoteWeight is optional, keep legacy behaviour if missing
	}
	return true, new(big.Int).SetBytes(p.Bundle.GetVoteWeight()), nil
}
