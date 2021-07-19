package vochain

import (
	"bytes"
	"fmt"
	"math/big"

	"go.vocdoni.io/dvote/censustree/gravitontree"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"google.golang.org/protobuf/proto"

	blind "github.com/arnaucube/go-blindsecp256k1"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/vocdoni/storage-proofs-eth-go/ethstorageproof"
	"github.com/vocdoni/storage-proofs-eth-go/token/mapbased"
	"github.com/vocdoni/storage-proofs-eth-go/token/minime"
	"go.vocdoni.io/proto/build/go/models"
)

// VerifyProofFunc is the generic function type to verify a proof of belonging
// into a census within a process.
type VerifyProofFunc func(process *models.Process, proof *models.Proof,
	censusOrigin models.CensusOrigin,
	censusRoot, processID, pubKey []byte, addr ethcommon.Address) (bool, *big.Int, error)

// VerifyProofOffChainTree verifies a proof with census origin OFF_CHAIN_TREE.
// Returns verification result and weight.
func VerifyProofOffChainTree(process *models.Process, proof *models.Proof,
	censusOrigin models.CensusOrigin,
	censusRoot, processID, pubKey []byte, addr ethcommon.Address) (bool, *big.Int, error) {
	var key []byte
	if process.EnvelopeType.Anonymous {
		// TODO Poseidon hash of pubKey
		// pubKeyDigested = snarks.Poseidon.Hash(pubKey)
		// NOT IMPLEMENTED
		return false, nil, fmt.Errorf("census origin OFF_CHAIN_TREE with " +
			"EnvelopeType.Anonymous not implemented")
	} else {
		key = pubKey
	}
	switch proof.Payload.(type) {
	case *models.Proof_Graviton:
		p := proof.GetGraviton()
		if p == nil {
			return false, nil, fmt.Errorf("graviton proof is empty")
		}
		valid, err := gravitontree.CheckProof(key, []byte{}, censusRoot, p.Siblings)
		return valid, big.NewInt(1), err
	case *models.Proof_Iden3:
		// NOT IMPLEMENTED
		return false, nil, fmt.Errorf("iden3 proof not implemented")
	default:
		return false, nil, fmt.Errorf("unexpected proof.Payload type: %T",
			proof.Payload)
	}
}

// VerifyProofOffChainCA verifies a proof with census origin OFF_CHAIN_CA.
// Returns verification result and weight.
func VerifyProofOffChainCA(process *models.Process, proof *models.Proof,
	censusOrigin models.CensusOrigin,
	censusRoot, processID, pubKey []byte, addr ethcommon.Address) (bool, *big.Int, error) {
	key := addr.Bytes()

	p := proof.GetCa()
	if !bytes.Equal(p.Bundle.Address, key) {
		return false, nil, fmt.Errorf(
			"CA bundle address and key do not match: %x != %x", key, p.Bundle.Address)
	}
	if !bytes.Equal(p.Bundle.ProcessId, processID) {
		return false, nil, fmt.Errorf("CA bundle processID does not match")
	}
	caBundle, err := proto.Marshal(p.Bundle)
	if err != nil {
		return false, nil, fmt.Errorf("cannot marshal ca bundle to protobuf: %w", err)
	}
	var caPubk []byte

	// depending on signature type, use a mechanism for extracting the ca publickey from signature
	switch p.GetType() {
	case models.ProofCA_ECDSA:
		caPubk, err = ethereum.PubKeyFromSignature(caBundle, p.GetSignature())
		if err != nil {
			return false, nil, fmt.Errorf("cannot fetch ca address from signature: %w", err)
		}
		if !bytes.Equal(caPubk, censusRoot) {
			return false, nil, fmt.Errorf("ca bundle signature does not match")
		}
	case models.ProofCA_ECDSA_BLIND:
		// Blind CA check
		pubdesc, err := ethereum.DecompressPubKey(censusRoot)
		if err != nil {
			return false, nil, fmt.Errorf("cannot decompress CA public key: %w", err)
		}
		pub, err := blind.NewPublicKeyFromECDSA(pubdesc)
		if err != nil {
			return false, nil, fmt.Errorf("cannot compute blind CA public key: %w", err)
		}
		signature, err := blind.NewSignatureFromBytes(p.GetSignature())
		if err != nil {
			return false, nil, fmt.Errorf("cannot compute blind CA signature: %w", err)
		}
		if !blind.Verify(new(big.Int).SetBytes(ethereum.HashRaw(caBundle)), signature, pub) {
			return false, nil, fmt.Errorf("blind CA verification failed %s", log.FormatProto(p.Bundle))
		}
	default:
		return false, nil, fmt.Errorf("ca proof %s type not supported", p.Type.String())
	}
	return true, big.NewInt(1), nil
}

// VerifyProofERC20 verifies a proof with census origin ERC20 (mapbased).
// Returns verification result and weight.
func VerifyProofERC20(process *models.Process, proof *models.Proof,
	censusOrigin models.CensusOrigin,
	censusRoot, processID, pubKey []byte, addr ethcommon.Address) (bool, *big.Int, error) {
	if process.EthIndexSlot == nil {
		return false, nil, fmt.Errorf("index slot not found for process %x", process.ProcessId)
	}
	p := proof.GetEthereumStorage()
	if p == nil {
		return false, nil, fmt.Errorf("ethereum proof is empty")
	}

	balance := new(big.Int).SetBytes(p.Value)
	if balance.Cmp(big.NewInt(0)) == 0 {
		return false, nil, fmt.Errorf("balance at proof is 0")
	}
	log.Debugf("validating erc20 storage proof for key %x and balance %v", p.Key, balance)
	err := mapbased.VerifyProof(addr, ethcommon.BytesToHash(censusRoot),
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
	censusOrigin models.CensusOrigin,
	censusRoot, processID, pubKey []byte, addr ethcommon.Address) (bool, *big.Int, error) {
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
	if proof0Balance.Cmp(big.NewInt(0)) == 0 {
		return false, nil, fmt.Errorf("balance at proofPrevBlock is 0")
	}
	log.Debugf("validating minime storage proof for key %x and balance %v",
		p.ProofPrevBlock.Key, proof0Balance)
	err := minime.VerifyProof(addr, ethcommon.BytesToHash(censusRoot),
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
