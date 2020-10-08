package vochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/crypto/nacl"
	"gitlab.com/vocdoni/go-dvote/crypto/snarks"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
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
		v, err := VoteTxCheck(tx, state, commit)
		if err != nil {
			return []byte{}, err
		}
		if commit {
			return v.Nullifier, state.AddVote(v)
		}
		return v.Nullifier, nil
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
				pk, err := hexPubKeyToTendermintEd25519(tx.PubKey)
				if err == nil {
					return []byte{}, state.AddValidator(pk, tx.Power)
				}
				return []byte{}, err

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
			pid, err := hex.DecodeString(util.TrimHex(tx.ProcessID))
			if err != nil {
				return []byte{}, err
			}
			return []byte{}, state.CancelProcess(pid)
		}

	case "NewProcessTx":
		tx := gtx.(*types.NewProcessTx)
		if p, err := NewProcessTxCheck(tx, state); err == nil {
			if commit {
				pid, err := hex.DecodeString(tx.ProcessID)
				if err != nil {
					return []byte{}, err
				}
				return []byte{}, state.AddProcess(*p, pid, tx.MkURI)
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
	switch types.ValidateType(txType.Type) {
	case "VoteTx":
		var tx types.VoteTx
		if err := json.Unmarshal(content, &tx); err != nil {
			return nil, fmt.Errorf("cannot parse VoteTX")
		}
		// In order to extract the signed bytes we need to remove the signature and txtype fields
		signature := tx.Signature
		tx.Signature = ""
		txtype := tx.Type
		tx.Type = ""
		signedBytes, err := json.Marshal(tx)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal voteTX (%s)", err)
		}
		tx.SignedBytes = signedBytes
		tx.Signature = signature
		tx.Type = txtype
		tx.Nullifier = util.TrimHex(tx.Nullifier)
		tx.ProcessID = util.TrimHex(tx.ProcessID)
		return &tx, nil

	case "AdminTx":
		var tx types.AdminTx
		if err := json.Unmarshal(content, &tx); err != nil {
			return nil, fmt.Errorf("cannot parse AdminTx")
		}
		signature := tx.Signature
		tx.Signature = ""
		signedBytes, err := json.Marshal(tx)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal voteTX (%s)", err)
		}
		tx.SignedBytes = signedBytes
		tx.Signature = signature
		tx.EncryptionPrivateKey = util.TrimHex(tx.EncryptionPrivateKey)
		tx.ProcessID = util.TrimHex(tx.ProcessID)
		tx.EncryptionPublicKey = util.TrimHex(tx.EncryptionPublicKey)
		tx.RevealKey = util.TrimHex(tx.RevealKey)
		tx.CommitmentKey = util.TrimHex(tx.CommitmentKey)
		return &tx, nil

	case "NewProcessTx":
		var tx types.NewProcessTx
		if err := json.Unmarshal(content, &tx); err != nil {
			return nil, fmt.Errorf("cannot parse NewProcessTx")
		}
		signature := tx.Signature
		tx.Signature = ""
		signedBytes, err := json.Marshal(tx)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal: (%s)", err)
		}
		tx.SignedBytes = signedBytes
		tx.Signature = signature
		tx.EntityID = util.TrimHex(tx.EntityID)
		tx.ProcessID = util.TrimHex(tx.ProcessID)
		tx.MkRoot = util.TrimHex(tx.MkRoot)
		return &tx, nil

	case "CancelProcessTx":
		var tx types.CancelProcessTx
		if err := json.Unmarshal(content, &tx); err != nil {
			return nil, fmt.Errorf("cannot parse CancelProcessTx")
		}
		signature := tx.Signature
		tx.Signature = ""
		signedBytes, err := json.Marshal(tx)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal: (%s)", err)
		}
		tx.SignedBytes = signedBytes
		tx.Signature = signature
		tx.ProcessID = util.TrimHex(tx.ProcessID)
		return &tx, nil
	}
	return nil, fmt.Errorf("invalid transaction type")
}

// VoteTxCheck is an abstraction of ABCI checkTx for submitting a vote
// All hexadecimal strings should be already sanitized (without 0x)
func VoteTxCheck(tx *types.VoteTx, state *State, forCommit bool) (*types.Vote, error) {
	pid, err := hex.DecodeString(tx.ProcessID)
	if err != nil {
		return nil, err
	}
	process, err := state.Process(pid, false)
	if err != nil {
		return nil, err
	}
	if process == nil {
		return nil, fmt.Errorf("process with id (%x) does not exist", pid)
	}
	header := state.Header(false)
	if header == nil {
		return nil, fmt.Errorf("cannot obtain state header")
	}
	height := header.Height
	endBlock := process.StartBlock + process.NumberOfBlocks

	if (height >= process.StartBlock && height <= endBlock) && !process.Canceled && !process.Paused {

		// Check in case of keys required, they have been sent by some keykeeper
		if process.RequireKeys() && process.KeyIndex < 1 {
			return nil, fmt.Errorf("no keys available, voting is not possible")
		}

		switch process.Type {

		case types.SnarkVote:
			// TODO check snark
			return nil, fmt.Errorf("snark vote not implemented")

		case types.PollVote, types.PetitionSign, types.EncryptedPoll:
			var vote types.Vote
			vote.ProcessID, err = hex.DecodeString(tx.ProcessID)
			if err != nil {
				return nil, err
			}
			vote.VotePackage = tx.VotePackage

			if types.ProcessIsEncrypted[process.Type] {
				if len(tx.EncryptionKeyIndexes) == 0 {
					return nil, fmt.Errorf("no key indexes provided on vote package")
				}
				vote.EncryptionKeyIndexes = tx.EncryptionKeyIndexes
			}

			// In order to avoid double vote check (on checkTx and deliverTx), we use a memory vote cache.
			// An element can only be added to the vote cache during checkTx.
			// Every 60 seconds (6 blocks) the old votes which are not yet in the blockchain will be removed from the cache.
			// If the same vote (but different transaction) is send to the mempool, the cache will detect it and vote will be discarted.
			uid := tx.UniqID(process.Type)
			vp := state.VoteCacheGet(uid)

			if forCommit && vp != nil {
				// if vote is in cache, lazy check and remove it from cache
				defer state.VoteCacheDel(uid)
				if state.EnvelopeExists(vote.ProcessID, vp.Nullifier) {
					return nil, fmt.Errorf("vote already exists")
				}
			} else {
				if vp != nil {
					return nil, fmt.Errorf("vote already exist in cache")
				}
				// if not in cache, extract pubKey, generate nullifier and check merkle proof
				vp = new(types.VoteProof)

				log.Debugf("vote Payload: %s", tx.SignedBytes)
				vp.PubKey, err = ethereum.PubKeyFromSignature(tx.SignedBytes, tx.Signature)
				if err != nil {
					return nil, fmt.Errorf("cannot extract public key from signature (%s)", err)
				}
				addr, err := ethereum.AddrFromPublicKey(vp.PubKey)
				if err != nil {
					return nil, fmt.Errorf("cannot extract address from public key: (%s)", err)
				}
				log.Debugf("extracted public key: %s", vp.PubKey)

				// assign a nullifier
				vp.Nullifier = GenerateNullifier(addr, vote.ProcessID)
				log.Debugf("generated new vote nullifier: %x", vp.Nullifier)

				// check if vote exists
				if state.EnvelopeExists(vote.ProcessID, vp.Nullifier) {
					return nil, fmt.Errorf("vote already exists")
				}

				// check merkle proof
				vp.Proof = tx.Proof
				pubKeyDec, err := hex.DecodeString(vp.PubKey)
				if err != nil {
					return nil, err
				}
				vp.PubKeyDigest = snarks.Poseidon.Hash(pubKeyDec)
				if len(vp.PubKeyDigest) != 32 {
					return nil, fmt.Errorf("cannot compute Poseidon hash: (%s)", err)
				}
				valid, err := checkMerkleProof(process.MkRoot, vp.Proof, vp.PubKeyDigest)
				if err != nil {
					return nil, fmt.Errorf("cannot check merkle proof: (%s)", err)
				}
				if !valid {
					return nil, fmt.Errorf("proof not valid")
				}
				vp.Created = time.Now()
				state.VoteCacheAdd(uid, vp)
			}
			vote.Nullifier = vp.Nullifier
			return &vote, nil

		default:
			return nil, fmt.Errorf("invalid process type")
		}
	}
	return nil, fmt.Errorf("cannot add vote, invalid block frame or process canceled/paused")
}

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func NewProcessTxCheck(tx *types.NewProcessTx, state *State) (*types.Process, error) {
	// check format
	if !util.IsHexEncodedStringWithLength(tx.ProcessID, types.ProcessIDsize) {
		return nil, fmt.Errorf("malformed processId")
	}
	if !util.IsHexEncodedStringWithLength(tx.EntityID, types.EntityIDsize) &&
		!util.IsHexEncodedStringWithLength(tx.EntityID, types.EntityIDsizeV2) {
		return nil, fmt.Errorf("malformed entityId")
	}
	pid, err := hex.DecodeString(tx.ProcessID)
	if err != nil {
		return nil, err
	}
	eid, err := hex.DecodeString(tx.EntityID)
	if err != nil {
		return nil, err
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

	authorized, addr, err := verifySignatureAgainstOracles(oracles, tx.SignedBytes, tx.Signature)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, fmt.Errorf("unauthorized to create a process, recovered addr: %s\nProcessTX: %s", addr.Hex(), tx.SignedBytes)
	}
	// get process
	_, err = state.Process(pid, false)
	if err == nil {
		return nil, fmt.Errorf("process with id (%x) already exists", pid)
	}
	// check type
	switch tx.ProcessType {
	case types.SnarkVote, types.PollVote, types.PetitionSign, types.EncryptedPoll:
		// ok
	default:
		return nil, fmt.Errorf("process type (%s) not valid", tx.ProcessType)
	}
	p := &types.Process{
		EntityID:       eid,
		MkRoot:         tx.MkRoot,
		NumberOfBlocks: tx.NumberOfBlocks,
		StartBlock:     tx.StartBlock,
		Type:           tx.ProcessType,
	}

	if p.RequireKeys() {
		// We consider the zero value as nil for security
		p.EncryptionPublicKeys = make([]string, types.MaxKeyIndex)
		p.EncryptionPrivateKeys = make([]string, types.MaxKeyIndex)
		p.CommitmentKeys = make([]string, types.MaxKeyIndex)
		p.RevealKeys = make([]string, types.MaxKeyIndex)
	}
	return p, nil
}

// CancelProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func CancelProcessTxCheck(tx *types.CancelProcessTx, state *State) error {
	// check format
	if !util.IsHexEncodedStringWithLength(tx.ProcessID, types.ProcessIDsize) {
		return fmt.Errorf("malformed processId")
	}
	pid, err := hex.DecodeString(tx.ProcessID)
	if err != nil {
		return err
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}
	// check signature
	authorized, addr, err := verifySignatureAgainstOracles(oracles, tx.SignedBytes, tx.Signature)
	if err != nil {
		return err
	}
	if !authorized {
		return fmt.Errorf("unauthorized to cancel a process, recovered addr: %s\nProcessTx: %s", addr.Hex(), tx.SignedBytes)
	}
	// get process
	process, err := state.Process(pid, false)
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

	if authorized, addr, err := verifySignatureAgainstOracles(oracles, tx.SignedBytes, tx.Signature); err != nil {
		return err
	} else if !authorized {
		return fmt.Errorf("unauthorized to perform an adminTx, address: %s", addr.Hex())
	}

	switch {
	case tx.Type == types.TxAddProcessKeys || tx.Type == types.TxRevealProcessKeys:
		pid, err := hex.DecodeString(tx.ProcessID)
		if err != nil {
			return err
		}
		// check process exists
		process, err := state.Process(pid, false)
		if err != nil {
			return err
		}
		if process == nil {
			return fmt.Errorf("process with id (%x) does not exist", pid)
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
			if len(process.EncryptionPublicKeys[tx.KeyIndex])+len(process.CommitmentKeys[tx.KeyIndex]) > 0 {
				return fmt.Errorf("keys for process %s already revealed", tx.ProcessID)
			}
			// check included keys and keyindex are valid
			if err := checkAddProcessKeys(tx, process); err != nil {
				return err
			}
		}
		if tx.Type == types.TxRevealProcessKeys {
			if header.Height < process.StartBlock+process.NumberOfBlocks && !process.Canceled {
				return fmt.Errorf("cannot reveal keys before the process is finished (%d < %d)", header.Height, process.StartBlock+process.NumberOfBlocks)
			}
			if len(process.EncryptionPrivateKeys[tx.KeyIndex])+len(process.RevealKeys[tx.KeyIndex]) > 0 {
				return fmt.Errorf("keys for process %s already revealed", tx.ProcessID)
			}
			// check the keys are valid
			if err := checkRevealProcessKeys(tx, process); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkAddProcessKeys(tx *types.AdminTx, process *types.Process) error {
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if len(tx.CommitmentKey)+len(tx.EncryptionPublicKey) == 0 || tx.KeyIndex < 1 || tx.KeyIndex > types.MaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex is not already used
	if len(process.EncryptionPublicKeys[tx.KeyIndex]) > 0 || len(process.CommitmentKeys[tx.KeyIndex]) > 0 {
		return fmt.Errorf("key index %d alrady exist", tx.KeyIndex)
	}
	// TBD check that provided keys are correct (ed25519 for encryption and size for Commitment)
	return nil
}

func checkRevealProcessKeys(tx *types.AdminTx, process *types.Process) error {
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if len(tx.RevealKey)+len(tx.EncryptionPrivateKey) == 0 || tx.KeyIndex < 1 || tx.KeyIndex > types.MaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex exists
	if len(process.EncryptionPublicKeys[tx.KeyIndex]) < 1 || len(process.CommitmentKeys[tx.KeyIndex]) < 1 {
		return fmt.Errorf("key index %d does not exist", tx.KeyIndex)
	}
	// check keys actually work
	if len(tx.EncryptionPrivateKey) > 0 {
		if priv, err := nacl.DecodePrivate(tx.EncryptionPrivateKey); err == nil {
			pub := priv.Public().Bytes()
			if fmt.Sprintf("%x", pub) != process.EncryptionPublicKeys[tx.KeyIndex] {
				log.Debugf("%x != %s", pub, process.EncryptionPublicKeys[tx.KeyIndex])
				return fmt.Errorf("the provided private key does not match with the stored public key on index %d", tx.KeyIndex)
			}
		} else {
			return err
		}
	}
	if len(tx.RevealKey) > 0 {
		rb, err := hex.DecodeString(tx.RevealKey)
		if err != nil {
			return err
		}
		commitment := snarks.Poseidon.Hash(rb)
		if fmt.Sprintf("%x", commitment) != process.CommitmentKeys[tx.KeyIndex] {
			log.Debugf("%x != %s", commitment, process.CommitmentKeys[tx.KeyIndex])
			return fmt.Errorf("the provided commitment reveal key does not match with the stored on index %d", tx.KeyIndex)
		}

	}
	return nil
}
