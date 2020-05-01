package vochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

const (
	processIDsize = 32
	// size of eth addr
	entityIDsize = 20
	// legacy: in the past we used hash(addr)
	// this is a temporal work around to support both
	entityIDsizeV2    = 32
	voteNullifierSize = 32
)

// GenericTX represents any valid transaction
type GenericTX interface {
	TxType() string
}

// AddTx check the validity of a transaction and adds it to the state if commit=true
func AddTx(gtx GenericTX, state *State, commit bool) ([]byte, error) {
	switch gtx.TxType() {
	case "VoteTx":
		tx := gtx.(*types.VoteTx)
		v, err := VoteTxCheck(tx, state)
		if err != nil {
			return []byte{}, err
		}
		if commit {
			return []byte(v.Nullifier), state.AddVote(v)
		}
		return []byte(v.Nullifier), nil
	case "AdminTx":
		tx := gtx.(*types.AdminTx)
		if err := AdminTxCheck(tx, state); err != nil {
			return []byte{}, err
		}
		if commit {
			switch tx.Type {
			case "addOracle":
				return []byte{}, state.AddOracle(tx.Address)
			case "removeOracle":
				return []byte{}, state.RemoveOracle(tx.Address)
			case "addValidator":
				if pk, err := hexPubKeyToTendermintEd25519(tx.PubKey); err == nil {
					return []byte{}, state.AddValidator(pk, tx.Power)
				} else {
					return []byte{}, err
				}
			case "removeValidator":
				return []byte{}, state.RemoveValidator(tx.Address)
			case types.TxAddProcessKeys:
				return []byte{}, state.AddProcessKeys(tx)
			case types.TxRevealProcessKeys:
				return []byte{}, state.RevealProcessKeys(tx)
			}
		}
	case "CancelProcessTx":
		tx := gtx.(*types.CancelProcessTx)
		if err := CancelProcessTxCheck(tx, state); err != nil {
			return []byte{}, err
		}
		if commit {
			return []byte{}, state.CancelProcess(tx.ProcessID)
		}

	case "NewProcessTx":
		tx := gtx.(*types.NewProcessTx)
		if p, err := NewProcessTxCheck(tx, state); err == nil {
			if commit {
				return []byte{}, state.AddProcess(p, tx.ProcessID)
			}
		} else {
			return []byte{}, err
		}
	default:
		return []byte{}, fmt.Errorf("transaction type invalid")
	}
	return []byte{}, nil
}

// UnmarshalTx splits a tx into method and args parts and does some basic checks
func UnmarshalTx(content []byte) (GenericTX, error) {
	var txType types.Tx
	err := json.Unmarshal(content, &txType)
	if err != nil || len(txType.Type) < 1 {
		return nil, fmt.Errorf("cannot extract type (%s)", err)
	}
	structType := types.ValidateType(txType.Type)
	switch structType {
	case "VoteTx":
		var tx types.VoteTx
		if err := json.Unmarshal(content, &tx); err != nil {
			return nil, fmt.Errorf("cannot parse VoteTX")
		}
		return &tx, nil

	case "AdminTx":
		var tx types.AdminTx
		if err := json.Unmarshal(content, &tx); err != nil {
			return nil, fmt.Errorf("cannot parse AdminTx")
		}
		return &tx, nil
	case "NewProcessTx":
		var tx types.NewProcessTx
		if err := json.Unmarshal(content, &tx); err != nil {
			return nil, fmt.Errorf("cannot parse NewProcessTx")
		}
		return &tx, nil

	case "CancelProcessTx":
		var tx types.CancelProcessTx
		if err := json.Unmarshal(content, &tx); err != nil {
			return nil, fmt.Errorf("cannot parse CancelProcessTx")
		}
		return &tx, nil
	}
	return nil, fmt.Errorf("invalid transaction type")
}

// VoteTxCheck is an abstraction of ABCI checkTx for submitting a vote
// All hexadecimal strings should be already sanitized (without 0x)
func VoteTxCheck(tx *types.VoteTx, state *State) (*types.Vote, error) {
	process, err := state.Process(tx.ProcessID, false)
	if err != nil {
		return nil, err
	}
	if process == nil {
		return nil, fmt.Errorf("process with id (%s) does not exist", tx.ProcessID)
	}
	if !util.IsHexEncodedStringWithLength(tx.ProcessID, processIDsize) {
		return nil, fmt.Errorf("malformed processId")
	}
	header := state.Header(false)
	if header == nil {
		return nil, fmt.Errorf("cannot obtain state header")
	}
	height := header.Height
	endBlock := process.StartBlock + process.NumberOfBlocks

	if (height >= process.StartBlock && height <= endBlock) && !process.Canceled && !process.Paused {
		switch process.Type {
		case types.SnarkVote:
			// TODO check snark
			return nil, fmt.Errorf("snark vote not implemented")
		case types.PollVote, types.PetitionSign, types.EncryptedPoll:
			var vote types.Vote
			vote.Nonce = tx.Nonce
			vote.ProcessID = tx.ProcessID
			vote.Proof = tx.Proof
			vote.VotePackage = tx.VotePackage
			vote.KeyIndexes = tx.KeyIndexes

			voteBytes, err := json.Marshal(vote)
			if err != nil {
				return nil, fmt.Errorf("cannot marshal vote (%s)", err)
			}
			pubKey, err := signature.PubKeyFromSignature(voteBytes, tx.Signature)
			if err != nil {
				return nil, fmt.Errorf("cannot extract public key from signature (%s)", err)
			}
			addr, err := signature.AddrFromPublicKey(pubKey)
			if err != nil {
				return nil, fmt.Errorf("cannot extract address from public key: (%s)", err)
			}

			// assign a nullifier
			vote.Nullifier, err = GenerateNullifier(addr, vote.ProcessID)
			if err != nil {
				return nil, fmt.Errorf("cannot generate nullifier: (%s)", err)
			}
			log.Debugf("generated nullifier: %s", vote.Nullifier)

			// check if vote exists
			if state.EnvelopeExists(fmt.Sprintf("%s_%s", vote.ProcessID, vote.Nullifier)) {
				return nil, fmt.Errorf("vote already exists")
			}

			// check merkle proof
			log.Debugf("extracted pubkey: %s", pubKey)
			pubKeyDec, err := hex.DecodeString(pubKey)
			if err != nil {
				return nil, err
			}
			pubKeyHash := signature.HashPoseidon(pubKeyDec)
			if len(pubKeyHash) != 32 {
				return nil, fmt.Errorf("cannot compute Poseidon hash: (%s)", err)
			}
			valid, err := checkMerkleProof(process.MkRoot, vote.Proof, pubKeyHash)
			if err != nil {
				return nil, fmt.Errorf("cannot check merkle proof: (%s)", err)
			}
			if !valid {
				return nil, fmt.Errorf("proof not valid")
			}
			return &vote, nil
		default:
			return nil, fmt.Errorf("invalid process type")
		}
	}
	return nil, fmt.Errorf("cannot add vote, invalid blocks frame or process canceled/paused")
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func NewProcessTxCheck(tx *types.NewProcessTx, state *State) (*types.Process, error) {
	// check format
	if !util.IsHexEncodedStringWithLength(tx.ProcessID, processIDsize) {
		return nil, fmt.Errorf("malformed processId")
	}
	if !util.IsHexEncodedStringWithLength(tx.EntityID, entityIDsize) &&
		!util.IsHexEncodedStringWithLength(tx.EntityID, entityIDsizeV2) {
		return nil, fmt.Errorf("malformed entityId")
	}

	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return nil, fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}

	header := state.Header(false)
	if header == nil {
		return nil, fmt.Errorf("cannot fetch state header")
	}
	// start and endblock sanity check
	if tx.StartBlock < header.Height {
		return nil, fmt.Errorf("cannot add process with start block lower or equal than the current tendermint height")
	}
	if tx.NumberOfBlocks <= 0 {
		return nil, fmt.Errorf("cannot add process with duration lower or equal than the current tendermint height")
	}

	// for checking the signature we need to remove the Signature from the transaction
	sign := tx.Signature
	tx.Signature = ""
	defer func() { tx.Signature = sign }() // in order to not modify the original tx, put signature back

	processBytes, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal process (%s)", err)
	}

	authorized, addr, err := verifySignatureAgainstOracles(oracles, processBytes, sign)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, fmt.Errorf("unauthorized to create a process, recovered addr: %s", addr)
	}
	// get process
	_, err = state.Process(tx.ProcessID, false)
	if err == nil {
		return nil, fmt.Errorf("process with id (%s) already exists", tx.ProcessID)
	}
	// check type
	switch tx.ProcessType {
	case types.SnarkVote, types.PollVote, types.PetitionSign, types.EncryptedPoll:
		// ok
	default:
		return nil, fmt.Errorf("process type (%s) not valid", tx.ProcessType)
	}
	p := &types.Process{
		EntityID:       tx.EntityID,
		MkRoot:         tx.MkRoot,
		NumberOfBlocks: tx.NumberOfBlocks,
		StartBlock:     tx.StartBlock,
		Type:           tx.ProcessType,
	}

	if p.RequireKeys() {
		// Size+1 because we are considering the zero value as nil for security
		p.EncryptionPublicKeys = make([]string, types.MaxKeyIndex+1)
		p.EncryptionPrivateKeys = make([]string, types.MaxKeyIndex+1)
		p.CommitmentKeys = make([]string, types.MaxKeyIndex+1)
		p.RevealKeys = make([]string, types.MaxKeyIndex+1)
	}
	return p, nil
}

// CancelProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func CancelProcessTxCheck(tx *types.CancelProcessTx, state *State) error {
	// check format
	if !util.IsHexEncodedStringWithLength(tx.ProcessID, processIDsize) {
		return fmt.Errorf("malformed processId")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	// check signature
	sign := tx.Signature
	tx.Signature = ""
	defer func() { tx.Signature = sign }() // in order to not modify the original tx, put signature back
	processBytes, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("cannot marshal cancel process info (%s)", err)
	}
	authorized, addr, err := verifySignatureAgainstOracles(oracles, processBytes, sign)
	if err != nil {
		return err
	}
	if !authorized {
		return fmt.Errorf("unauthorized to cancel a process, message: %s, recovered addr: %s", string(processBytes), addr)
	}
	// get process
	process, err := state.Process(tx.ProcessID, false)
	if err != nil {
		return fmt.Errorf("cannot cancel the process: %s", err)
	}
	// check process not already canceled or finalized
	if process.Canceled {
		return fmt.Errorf("cannot cancel an already canceled process")
	}
	endBlock := process.StartBlock + process.NumberOfBlocks
	var height int64
	if h := state.Header(false); h != nil {
		height = h.Height
	}

	if endBlock < height {
		return fmt.Errorf("cannot cancel a finalized process")
	}
	return nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(tx *types.AdminTx, state *State) error {
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}

	sign := tx.Signature
	tx.Signature = ""
	defer func() { tx.Signature = sign }() // in order to not modify the original tx, put signature back

	adminTxBytes, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("cannot marshal adminTx (%s)", err)
	}

	if authorized, addr, err := verifySignatureAgainstOracles(oracles, adminTxBytes, sign); err != nil {
		return err
	} else if !authorized {
		return fmt.Errorf("unauthorized to perform an adminTx, address: %s", addr)
	}

	switch {
	case tx.Type == types.TxAddProcessKeys || tx.Type == types.TxRevealProcessKeys:
		// check process exists
		process, err := state.Process(tx.ProcessID, false)
		if err != nil {
			return err
		}
		if process == nil {
			return fmt.Errorf("process with id (%s) does not exist", tx.ProcessID)
		}
		// check process actually requires keys
		if !process.RequireKeys() {
			return fmt.Errorf("process does not require keys")
		}
		// get the current blockchain header
		header := state.Header(false)
		if header == nil {
			return fmt.Errorf("cannot get blockchain header")
		}
		// Specific checks
		if tx.Type == types.TxAddProcessKeys {
			// endblock is always greater than start block so that case is also included here
			if header.Height > process.StartBlock {
				return fmt.Errorf("cannot add process keys in a started or finished process")
			}
			// process is not canceled
			if process.Canceled {
				return fmt.Errorf("cannot add process keys in a canceled process")
			}
			// check included keys and keyindex are valid
			if err := checkAddProcessKeys(tx, process); err != nil {
				return err
			}
		}
		if tx.Type == types.TxRevealProcessKeys {
			if process.StartBlock+process.NumberOfBlocks < header.Height && !process.Canceled {
				return fmt.Errorf("cannot reveal keys before the process is finished")
			}
			// check the keys are valid
			if err := checkRevealProcessKeys(tx, process); err != nil {
				return err
			}
		}
	}
	return nil
}
