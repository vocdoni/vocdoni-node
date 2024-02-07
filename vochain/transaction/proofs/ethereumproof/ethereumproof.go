package ethereumproof

import (
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/storage-proofs-eth-go/ethstorageproof"
	"github.com/vocdoni/storage-proofs-eth-go/token/mapbased"
	"github.com/vocdoni/storage-proofs-eth-go/token/minime"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

var bigZero = big.NewInt(0)

// ProofVerifierEthereumStorage defines the interface for Ethereum storage based proof verification systems.
type ProofVerifierEthereumStorage struct{}

// Verify verifies a proof with census origin ERC20 or MINI_ME.
func (*ProofVerifierEthereumStorage) Verify(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error) {
	switch process.CensusOrigin {
	case models.CensusOrigin_ERC20:
		return VerifyProofERC20(process, envelope, vID)

	case models.CensusOrigin_MINI_ME:
		return VerifyProofMiniMe(process, envelope, vID)
	default:
		return false, nil, fmt.Errorf("census origin not ethereum storage: %s", process.CensusOrigin)
	}
}

// VerifyProofERC20 verifies a proof with census origin ERC20 (mapbased).
// Returns verification result and weight.
func VerifyProofERC20(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error) {
	if process.EthIndexSlot == nil {
		return false, nil, fmt.Errorf("index slot not found for process %x", process.ProcessId)
	}
	p := envelope.Proof.GetEthereumStorage()
	if p == nil {
		return false, nil, fmt.Errorf("ethereum proof is empty")
	}

	balance := new(big.Int).SetBytes(p.Value)
	if balance.Cmp(bigZero) == 0 {
		return false, nil, fmt.Errorf("balance at proof is 0")
	}
	log.Debugf("validating erc20 storage proof for key %x and balance %v", p.Key, balance)
	err := mapbased.VerifyProof(ethereum.AddrFromBytes(vID.Address()), ethcommon.BytesToHash(process.CensusRoot),
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
func VerifyProofMiniMe(process *models.Process, envelope *models.VoteEnvelope, vID state.VoterID) (bool, *big.Int, error) {
	if process.EthIndexSlot == nil {
		return false, nil, fmt.Errorf("index slot not found for process %x", process.ProcessId)
	}
	if process.SourceBlockHeight == nil {
		return false, nil, fmt.Errorf("source block height not found for process %x",
			process.ProcessId)
	}
	p := envelope.Proof.GetMinimeStorage()
	if p == nil {
		return false, nil, fmt.Errorf("minime proof is empty")
	}

	_, proof0Balance, _ := minime.ParseMinimeValue(p.ProofPrevBlock.Value, 0)
	if proof0Balance.Cmp(bigZero) == 0 {
		return false, nil, fmt.Errorf("balance at proofPrevBlock is 0")
	}
	log.Debugf("validating minime storage proof for key %x and balance %v",
		p.ProofPrevBlock.Key, proof0Balance)
	err := minime.VerifyProof(ethereum.AddrFromBytes(vID.Address()), ethcommon.BytesToHash(process.CensusRoot),
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
