package cspproof

import (
	"bytes"
	"fmt"
	"math/big"

	blind "github.com/arnaucube/go-blindsecp256k1"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/saltedkey"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

var bigOne = big.NewInt(1)

// ProofVerifierCSP defines the interface for CSP (Credential Service Provider) proof verification systems.
type ProofVerifierCSP struct{}

// VerifyProof verifies a proof with census origin OFF_CHAIN_CA.
// Returns verification result and weight.
func (*ProofVerifierCSP) Verify(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error) {
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
			rootPubSalted, err = saltedkey.SaltECDSAPubKey(rootPubSalted, envelope.ProcessId)
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
			rootPub, err = saltedkey.SaltBlindPubKey(rootPub, envelope.ProcessId)
			if err != nil {
				return false, nil, fmt.Errorf("cannot salt blind pubkey: %w", err)
			}
		}
		if !blind.Verify(new(big.Int).SetBytes(ethereum.HashRaw(cspBundle)), signature, rootPub) {
			cspbundleDec := &models.CAbundle{}
			if err := proto.Unmarshal(cspBundle, cspbundleDec); err != nil {
				log.Warnf("cannot unmarshal CSP bundle: %v", err)
			}
			return false, nil, fmt.Errorf("blind CSP verification failed for "+
				"pid %x with CSP key %x. CSP bundle {pid:%x, addr:%x}. CSP signature %x",
				envelope.ProcessId, rootPub.Bytes(), cspbundleDec.ProcessId, cspbundleDec.Address, signature.Bytes())
		}
	default:
		return false, nil, fmt.Errorf("CSP proof %s type not supported", p.Type)
	}
	return true, bigOne, nil
}
