package transaction

import (
	"bytes"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/saltedkey"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree"
	"go.vocdoni.io/dvote/vochain/state"
	"google.golang.org/protobuf/proto"

	blind "github.com/arnaucube/go-blindsecp256k1"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/vocdoni/storage-proofs-eth-go/ethstorageproof"
	"github.com/vocdoni/storage-proofs-eth-go/token/mapbased"
	"github.com/vocdoni/storage-proofs-eth-go/token/minime"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/proto/build/go/models"
)

var (
	bigZero = big.NewInt(0)
	bigOne  = big.NewInt(1)
)

// VerifyProofFunc is the generic function type to verify a proof of belonging
// into a census within a process.
type VerifyProofFunc func(process *models.Process, proof *models.Proof,
	censusOrigin models.CensusOrigin,
	censusRoot, processID []byte, vID state.VoterID) (bool, *big.Int, error)

// VerifyProof is a wrapper over all VerifyProofFunc(s) available which uses the process.CensusOrigin
// to execute the correct verification function.
func VerifyProof(process *models.Process, proof *models.Proof, vID state.VoterID) (bool, *big.Int, error) {
	if proof == nil {
		return false, nil, fmt.Errorf("proof is nil")
	}
	if process == nil {
		return false, nil, fmt.Errorf("process is nil")
	}
	log.Debugw("verify proof",
		"censusOrigin", process.CensusOrigin,
		"electionID", fmt.Sprintf("%x", process.ProcessId),
		"voterID", fmt.Sprintf("%x", vID),
		"address", fmt.Sprintf("%x", vID.Address()),
		"censusRoot", fmt.Sprintf("%x", process.CensusRoot),
	)
	// check census origin and compute vote digest identifier
	var verifyProof VerifyProofFunc
	switch process.CensusOrigin {
	case models.CensusOrigin_OFF_CHAIN_TREE, models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED:
		verifyProof = VerifyProofOffChainTree
	case models.CensusOrigin_OFF_CHAIN_CA:
		verifyProof = VerifyProofOffChainCSP
	case models.CensusOrigin_ERC20:
		verifyProof = VerifyProofERC20
	case models.CensusOrigin_MINI_ME:
		verifyProof = VerifyProofMiniMe
	default:
		return false, nil, fmt.Errorf("census origin not compatible")
	}
	valid, weight, err := verifyProof(process, proof,
		process.CensusOrigin, process.CensusRoot, process.ProcessId,
		vID)
	if err != nil {
		return false, nil, fmt.Errorf("proof not valid: %w", err)
	}
	return valid, weight, nil
}

// VerifyProofOffChainTree verifies a proof with census origin OFF_CHAIN_TREE.
// Returns verification result and weight.
func VerifyProofOffChainTree(_ *models.Process, proof *models.Proof,
	_ models.CensusOrigin,
	censusRoot, _ []byte, vID state.VoterID) (bool, *big.Int, error) {
	switch proof.Payload.(type) {
	case *models.Proof_Arbo:
		p := proof.GetArbo()
		if p == nil {
			return false, nil, fmt.Errorf("arbo proof is empty")
		}
		// get the merkle tree hashing function
		var hashFunc arbo.HashFunction = arbo.HashFunctionBlake2b
		switch p.Type {
		case models.ProofArbo_BLAKE2B:
			hashFunc = arbo.HashFunctionBlake2b
		case models.ProofArbo_POSEIDON:
			hashFunc = arbo.HashFunctionPoseidon
		default:
			return false, nil, fmt.Errorf("not recognized ProofArbo type: %s", p.Type)
		}
		if vID == nil {
			return false, nil, fmt.Errorf("voterID is nil")
		}
		// check if the proof key is for an address (default) or a pubKey
		var err error
		key := vID.Address()
		// TODO (lucasmenendez): Remove hashing of the address
		if p.Type != models.ProofArbo_POSEIDON {
			if key, err = hashFunc.Hash(key); err != nil {
				return false, nil, fmt.Errorf("cannot hash proof key: %w", err)
			}
			if len(key) > censustree.DefaultMaxKeyLen {
				key = key[:censustree.DefaultMaxKeyLen]
			}
		}
		valid, err := tree.VerifyProof(hashFunc, key, p.AvailableWeight, p.Siblings, censusRoot)
		if !valid || err != nil {
			return false, nil, err
		}
		// Legacy: support p.LeafWeight == nil, assume then value=1
		if p.AvailableWeight == nil {
			return true, bigOne, nil
		}

		availableWeight := arbo.BytesToBigInt(p.AvailableWeight)
		if p.VoteWeight == nil {
			return true, availableWeight, nil
		}

		voteWeight := arbo.BytesToBigInt(p.VoteWeight)
		if voteWeight.Cmp(availableWeight) == 1 {
			return false, nil, fmt.Errorf("assigned weight exceeded")
		}
		return true, voteWeight, nil
	default:
		return false, nil, fmt.Errorf("unexpected proof.Payload type: %T",
			proof.Payload)
	}
}

// VerifyProofOffChainCSP verifies a proof with census origin OFF_CHAIN_CA.
// Returns verification result and weight.
func VerifyProofOffChainCSP(_ *models.Process, proof *models.Proof,
	_ models.CensusOrigin,
	censusRoot, processID []byte, vID state.VoterID) (bool, *big.Int, error) {
	key := vID.Address()

	p := proof.GetCa()
	if p == nil || p.Bundle == nil {
		return false, nil, fmt.Errorf("CSP proof or bundle are nil")
	}
	if !bytes.Equal(p.Bundle.Address, key) {
		return false, nil, fmt.Errorf(
			"CSP bundle address and key do not match: %x != %x", key, p.Bundle.Address)
	}
	if !bytes.Equal(p.Bundle.ProcessId, processID) {
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
			rootPubSalted, err = saltedkey.SaltECDSAPubKey(rootPubSalted, processID)
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
			rootPub, err = saltedkey.SaltBlindPubKey(rootPub, processID)
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
				processID, rootPub.Bytes(), cspbundleDec.ProcessId, cspbundleDec.Address, signature.Bytes())
		}
	default:
		return false, nil, fmt.Errorf("CSP proof %s type not supported", p.Type)
	}
	return true, bigOne, nil
}

// VerifyProofERC20 verifies a proof with census origin ERC20 (mapbased).
// Returns verification result and weight.
func VerifyProofERC20(process *models.Process, proof *models.Proof,
	_ models.CensusOrigin,
	censusRoot, _ []byte, vID state.VoterID) (bool, *big.Int, error) {
	if process.EthIndexSlot == nil {
		return false, nil, fmt.Errorf("index slot not found for process %x", process.ProcessId)
	}
	p := proof.GetEthereumStorage()
	if p == nil {
		return false, nil, fmt.Errorf("ethereum proof is empty")
	}

	balance := new(big.Int).SetBytes(p.Value)
	if balance.Cmp(bigZero) == 0 {
		return false, nil, fmt.Errorf("balance at proof is 0")
	}
	log.Debugf("validating erc20 storage proof for key %x and balance %v", p.Key, balance)
	err := mapbased.VerifyProof(ethereum.AddrFromBytes(vID.Address()), ethcommon.BytesToHash(censusRoot),
		ethstorageproof.StorageResult{
			Key:   p.Key,
			Proof: p.Siblings,
			Value: p.Value,
		},
		int(*process.EthIndexSlot),
		balance,
		nil)
	return err == nil, balance, err
}

// VerifyProofMiniMe verifies a proof with census origin MiniMe.
// Returns verification result and weight.
func VerifyProofMiniMe(process *models.Process, proof *models.Proof,
	_ models.CensusOrigin,
	censusRoot, _ []byte, vID state.VoterID) (bool, *big.Int, error) {
	if process.EthIndexSlot == nil {
		return false, nil, fmt.Errorf("index slot not found for process %x", process.ProcessId)
	}
	if process.SourceBlockHeight == nil {
		return false, nil, fmt.Errorf("source block height not found for process %x",
			process.ProcessId)
	}
	p := proof.GetMinimeStorage()
	if p == nil {
		return false, nil, fmt.Errorf("minime proof is empty")
	}

	_, proof0Balance, _ := minime.ParseMinimeValue(p.ProofPrevBlock.Value, 0)
	if proof0Balance.Cmp(bigZero) == 0 {
		return false, nil, fmt.Errorf("balance at proofPrevBlock is 0")
	}
	log.Debugf("validating minime storage proof for key %x and balance %v",
		p.ProofPrevBlock.Key, proof0Balance)
	err := minime.VerifyProof(ethereum.AddrFromBytes(vID.Address()), ethcommon.BytesToHash(censusRoot),
		[]ethstorageproof.StorageResult{
			{
				Key:   p.ProofPrevBlock.Key,
				Proof: p.ProofPrevBlock.Siblings,
				Value: p.ProofPrevBlock.Value,
			},
			{
				Key:   p.ProofNextBlock.Key,
				Proof: p.ProofNextBlock.Siblings,
				Value: p.ProofNextBlock.Value,
			},
		},
		int(*process.EthIndexSlot),
		proof0Balance,
		new(big.Int).SetUint64(*process.SourceBlockHeight))
	return err == nil, proof0Balance, err
}
