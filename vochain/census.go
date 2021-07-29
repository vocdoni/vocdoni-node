package vochain

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	models "go.vocdoni.io/proto/build/go/models"
)

// AddToRollingCensus adds a new key to an existing rolling census.
// If census does not exist yet it will be created.
func (s *State) AddToRollingCensus(pid []byte, key []byte, weight *big.Int) error {
	/*
		// In the state we only store the last census root (as value) using key as index
		s.Lock()
		err := s.Store.Tree(CensusTree).Add(p.ProcessId, root)
		s.Unlock()
		if err != nil {
			return err
		}
	*/
	return fmt.Errorf("TODO")
}

// PurgeRollingCensus removes a rolling census from the permanent store
// If the census does not exist, it does nothing.
func (s *State) PurgeRollingCensus(pid []byte) error {
	return fmt.Errorf("TODO")
}

// GetRollingCensusRoot returns the last rolling census root for a process id
func (s *State) GetRollingCensusRoot(pid []byte, isQuery bool) ([]byte, error) {
	/*	s.Lock()
		err := s.Store.Tree(CensusTree).Add(p.ProcessId, root)
		s.Unlock()
		if err != nil {
			return err
		}GetCensusRoot
	*/
	return nil, fmt.Errorf("TODO")
}

// RegisterKeyTxCheck validates a registerKeyTx transaction against the state
func RegisterKeyTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	tx := vtx.GetRegisterKey()

	// Sanity checks
	if tx == nil {
		return fmt.Errorf("register key transaction is nil")
	}
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return fmt.Errorf("cannot fetch processId: %w", err)
	}
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return fmt.Errorf("process %x malformed", tx.ProcessId)
	}
	if state.Height() >= process.StartBlock {
		return fmt.Errorf("process %x already started", tx.ProcessId)
	}
	if process.Status != models.ProcessStatus_READY {
		return fmt.Errorf("process %x not in READY state", tx.ProcessId)
	}
	if tx.Proof == nil {
		return fmt.Errorf("proof missing on registerKeyTx")
	}
	if signature == nil {
		return fmt.Errorf("signature missing on voteTx")
	}
	if len(tx.NewKey) < 32 { // TODO: check the correctnes of the new public key
		return fmt.Errorf("newKey wrong size")
	}

	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature: (%w)", err)
	}
	var addr common.Address
	addr, err = ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: (%w)", err)
	}

	var valid bool
	var weight *big.Int
	valid, weight, err = VerifyProof(process, tx.Proof,
		process.CensusOrigin,
		process.CensusRoot,
		process.ProcessId,
		pubKey,
		addr,
	)
	if err != nil {
		return fmt.Errorf("proof not valid: (%w)", err)
	}
	if !valid {
		return fmt.Errorf("proof not valid")
	}
	tx.Weight = weight.Bytes() // TODO: support weight

	return nil
}
