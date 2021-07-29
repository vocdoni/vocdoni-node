package vochain

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// AddTx check the validity of a transaction and adds it to the state if commit=true.
// It returns a bytes value which depends on the transaction type:
//  Tx_Vote: vote nullifier
//  default: []byte{}
func AddTx(vtx *models.Tx, txBytes, signature []byte, state *State,
	txID [32]byte, commit bool) ([]byte, error) {
	if vtx == nil || state == nil || vtx.Payload == nil {
		return nil, fmt.Errorf("transaction, state, and/or transaction payload is nil")
	}
	switch vtx.Payload.(type) {
	case *models.Tx_Vote:
		v, err := VoteTxCheck(vtx, txBytes, signature, state, txID, commit)
		if err != nil || v == nil {
			return []byte{}, fmt.Errorf("voteTxCheck: %w", err)
		}
		if commit {
			return v.Nullifier, state.AddVote(v)
		}
		return v.Nullifier, nil
	case *models.Tx_Admin:
		if err := AdminTxCheck(vtx, txBytes, signature, state); err != nil {
			return []byte{}, fmt.Errorf("adminTxChek: %w", err)
		}
		tx := vtx.GetAdmin()
		if commit {
			switch tx.Txtype {
			case models.TxType_ADD_ORACLE:
				return []byte{}, state.AddOracle(common.BytesToAddress(tx.Address))
			case models.TxType_REMOVE_ORACLE:
				return []byte{}, state.RemoveOracle(common.BytesToAddress(tx.Address))
			case models.TxType_ADD_VALIDATOR:
				pk, err := hexPubKeyToTendermintEd25519(fmt.Sprintf("%x", tx.PublicKey))
				if err == nil {
					if tx.Power == nil {
						return []byte{}, fmt.Errorf("power not specified on addValidator transaction")
					}
					validator := &models.Validator{
						Address: pk.Address().Bytes(),
						PubKey:  pk.Bytes(),
						Power:   *tx.Power,
					}
					return []byte{}, state.AddValidator(validator)

				}
				return []byte{}, fmt.Errorf("addValidator: %w", err)

			case models.TxType_REMOVE_VALIDATOR:
				return []byte{}, state.RemoveValidator(tx.Address)
			case models.TxType_ADD_PROCESS_KEYS:
				return []byte{}, state.AddProcessKeys(tx)
			case models.TxType_REVEAL_PROCESS_KEYS:
				return []byte{}, state.RevealProcessKeys(tx)
			}
		}

	case *models.Tx_NewProcess:
		if p, err := NewProcessTxCheck(vtx, txBytes, signature, state); err == nil {
			if commit {
				tx := vtx.GetNewProcess()
				if tx.Process == nil {
					return []byte{}, fmt.Errorf("newProcess process is empty")
				}
				return []byte{}, state.AddProcess(p)
			}
		} else {
			return []byte{}, fmt.Errorf("newProcess: %w", err)
		}

	case *models.Tx_SetProcess:
		if err := SetProcessTxCheck(vtx, txBytes, signature, state); err != nil {
			return []byte{}, fmt.Errorf("setProcess: %w", err)
		}
		if commit {
			tx := vtx.GetSetProcess()
			switch tx.Txtype {
			case models.TxType_SET_PROCESS_STATUS:
				if tx.GetStatus() == models.ProcessStatus_PROCESS_UNKNOWN {
					return []byte{}, fmt.Errorf("set process status, status unknown")
				}
				return []byte{}, state.SetProcessStatus(tx.ProcessId, *tx.Status, true)
			case models.TxType_SET_PROCESS_RESULTS:
				if tx.GetResults() == nil {
					return []byte{}, fmt.Errorf("set process results, results is nil")
				}
				return []byte{}, state.SetProcessResults(tx.ProcessId, tx.Results, true)
			case models.TxType_SET_PROCESS_CENSUS:
				if tx.GetCensusRoot() == nil {
					return []byte{}, fmt.Errorf("set process census, census root is nil")
				}
				return []byte{}, state.SetProcessCensus(tx.ProcessId, tx.CensusRoot, tx.GetCensusURI(), true)
			default:
				return []byte{}, fmt.Errorf("unknown set process tx type")
			}
		}

	case *models.Tx_RegisterKey:
		if err := RegisterKeyTxCheck(vtx, txBytes, signature, state); err != nil {
			return []byte{}, fmt.Errorf("registerKeyTx %w", err)
		}
		if commit {
			tx := vtx.GetRegisterKey()
			return []byte{}, state.AddToRollingCensus(tx.ProcessId, tx.NewKey, new(big.Int).SetBytes(tx.Weight))
		}

	default:
		return []byte{}, fmt.Errorf("invalid transaction type")
	}
	return []byte{}, nil
}

// UnmarshalTx unarshal the content of a bytes serialized transaction.
// Returns the transaction struct, the original bytes and the signature
// of those bytes.
func UnmarshalTx(content []byte) (*models.Tx, []byte, []byte, error) {
	stx := new(models.SignedTx)
	if err := proto.Unmarshal(content, stx); err != nil {
		return nil, nil, nil, err
	}
	vtx := new(models.Tx)
	return vtx, stx.GetTx(), stx.GetSignature(), proto.Unmarshal(stx.GetTx(), vtx)
}

// VoteTxCheck is an abstraction of ABCI checkTx for submitting a vote
// All hexadecimal strings should be already sanitized (without 0x)
func VoteTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State,
	txID [32]byte, forCommit bool) (*models.Vote, error) {
	tx := vtx.GetVote()

	// Perform basic/general checks
	if tx == nil {
		return nil, fmt.Errorf("vote envelope transaction is nil")
	}
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch processId: %w", err)
	}
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return nil, fmt.Errorf("process %x malformed", tx.ProcessId)
	}
	height := state.Height()
	endBlock := process.StartBlock + process.BlockCount

	if height < process.StartBlock || height > endBlock {
		return nil, fmt.Errorf("process %x not started or finished", tx.ProcessId)
	}

	if process.Status != models.ProcessStatus_READY {
		return nil, fmt.Errorf("process %x not in READY state", tx.ProcessId)
	}

	// Check in case of keys required, they have been sent by some keykeeper
	if process.EnvelopeType.EncryptedVotes &&
		process.KeyIndex != nil &&
		*process.KeyIndex < 1 {
		return nil, fmt.Errorf("no keys available, voting is not possible")
	}

	var vote *models.Vote
	switch {
	case process.EnvelopeType.Anonymous:
		// TODO check snark
		return nil, fmt.Errorf("snark vote not implemented")
	default: // Signature based voting
		if signature == nil {
			return nil, fmt.Errorf("signature missing on voteTx")
		}
		// In order to avoid double vote check (on checkTx and deliverTx), we use a memory vote cache.
		// An element can only be added to the vote cache during checkTx.
		// Every N seconds the old votes which are not yet in the blockchain will be removed from cache.
		// If the same vote (but different transaction) is send to the mempool, the cache will detect it
		// and vote will be discarted.
		vote = state.CacheGet(txID)

		// if vote is in cache, lazy check and remove it from cache
		if forCommit && vote != nil {
			vote.Height = height // update vote height
			defer state.CacheDel(txID)
			if exist, err := state.EnvelopeExists(vote.ProcessId,
				vote.Nullifier, false); err != nil || exist {
				if err != nil {
					return nil, err
				}
				return nil, fmt.Errorf("vote %x already exists", vote.Nullifier)
			}
			return vote, nil
		}

		// if not forCommit, it is a mempool check,
		// reject it since we already processed the transaction before.
		if !forCommit && vote != nil {
			return nil, fmt.Errorf("vote %x already exists in cache", vote.Nullifier)
		}

		// if not in cache, full check
		// extract pubKey, generate nullifier and check census proof.
		// add the transaction in the cache
		if tx.Proof == nil {
			return nil, fmt.Errorf("proof not found on transaction")
		}

		vote = &models.Vote{
			Height:      height,
			ProcessId:   tx.ProcessId,
			VotePackage: tx.VotePackage,
		}
		// If process encrypted, check the vote is encrypted (includes at least one key index)
		if process.EnvelopeType.EncryptedVotes {
			if len(tx.EncryptionKeyIndexes) == 0 {
				return nil, fmt.Errorf("no key indexes provided on vote package")
			}
			vote.EncryptionKeyIndexes = tx.EncryptionKeyIndexes
		}
		pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
		if err != nil {
			return nil, fmt.Errorf("cannot extract public key from signature: %w", err)
		}
		addr, err := ethereum.AddrFromPublicKey(pubKey)
		if err != nil {
			return nil, fmt.Errorf("cannot extract address from public key: %w", err)
		}

		// assign a nullifier
		vote.Nullifier = GenerateNullifier(addr, vote.ProcessId)

		// check that nullifier does not exist in cache already, this avoids
		// processing multiple transactions with same nullifier.
		if state.CacheHasNullifier(vote.Nullifier) {
			return nil, fmt.Errorf("nullifier %x already exists in cache", vote.Nullifier)
		}

		// check if vote already exists
		if exist, err := state.EnvelopeExists(vote.ProcessId,
			vote.Nullifier, false); err != nil || exist {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("vote %x already exists", vote.Nullifier)
		}
		log.Debugf("new vote %x for address %s and process %x", vote.Nullifier, addr.Hex(), tx.ProcessId)

		valid, weight, err := VerifyProof(process, tx.Proof,
			process.CensusOrigin, process.CensusRoot, process.ProcessId,
			pubKey, addr)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, fmt.Errorf("proof not valid")
		}
		vote.Weight = weight.Bytes()

		// add the vote to cache
		state.CacheAdd(txID, vote)
	}
	return vote, nil
}

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func AdminTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	tx := vtx.GetAdmin()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return fmt.Errorf("missing signature and/or admin transaction")
	}
	// get oracles
	oracles, err := state.Oracles(false)
	if err != nil || len(oracles) == 0 {
		return fmt.Errorf("cannot check authorization against a nil or empty oracle list")
	}

	if authorized, addr, err := verifySignatureAgainstOracles(
		oracles, txBytes, signature); err != nil {
		return err
	} else if !authorized {
		return fmt.Errorf("unauthorized to perform an adminTx, address: %s", addr.Hex())
	}

	switch tx.Txtype {
	case models.TxType_ADD_PROCESS_KEYS, models.TxType_REVEAL_PROCESS_KEYS:
		if tx.ProcessId == nil {
			return fmt.Errorf("missing processId on AdminTxCheck")
		}
		// check process exists
		process, err := state.Process(tx.ProcessId, false)
		if err != nil {
			return err
		}
		if process == nil {
			return fmt.Errorf("process with id (%x) does not exist", tx.ProcessId)
		}
		// check process actually requires keys
		if !process.EnvelopeType.EncryptedVotes && !process.EnvelopeType.Anonymous {
			return fmt.Errorf("process does not require keys")
		}

		height := state.Height()
		// Specific checks
		switch tx.Txtype {
		case models.TxType_ADD_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// endblock is always greater than start block so that case is also included here
			if height > process.StartBlock {
				return fmt.Errorf("cannot add process keys to a process that has started or finished")
			}
			// process is not canceled
			if process.Status == models.ProcessStatus_CANCELED ||
				process.Status == models.ProcessStatus_ENDED ||
				process.Status == models.ProcessStatus_RESULTS {
				return fmt.Errorf("cannot add process keys to a %s process", process.Status)
			}
			if len(process.EncryptionPublicKeys[*tx.KeyIndex])+
				len(process.CommitmentKeys[*tx.KeyIndex]) > 0 {
				return fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check included keys and keyindex are valid
			if err := checkAddProcessKeys(tx, process); err != nil {
				return err
			}
		case models.TxType_REVEAL_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// check process is finished
			if height < process.StartBlock+process.BlockCount &&
				!(process.Status == models.ProcessStatus_ENDED ||
					process.Status == models.ProcessStatus_CANCELED) {
				return fmt.Errorf("cannot reveal keys before the process is finished")
			}
			if len(process.EncryptionPrivateKeys[*tx.KeyIndex])+len(process.RevealKeys[*tx.KeyIndex]) > 0 {
				return fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check the keys are valid
			if err := checkRevealProcessKeys(tx, process); err != nil {
				return err
			}
		}
	case models.TxType_ADD_ORACLE:
		// check not empty, correct length and not 0x0 addr
		if (bytes.Equal(tx.Address, []byte{})) ||
			(len(tx.Address) != types.EthereumAddressSize) ||
			(bytes.Equal(tx.Address, common.Address{}.Bytes())) {
			return fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		for idx, oracle := range oracles {
			if oracle == common.BytesToAddress(tx.Address) {
				return fmt.Errorf("oracle already added to oracle list at position %d", idx)
			}
		}
	case models.TxType_REMOVE_ORACLE:
		// check not empty, correct length and not 0x0 addr
		if (bytes.Equal(tx.Address, []byte{})) ||
			(len(tx.Address) != types.EthereumAddressSize) ||
			(bytes.Equal(tx.Address, common.Address{}.Bytes())) {
			return fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		var found bool
		for _, oracle := range oracles {
			if oracle == common.BytesToAddress(tx.Address) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("cannot remove oracle, not found")
		}
	}
	return nil
}

func checkAddProcessKeys(tx *models.AdminTx, process *models.Process) error {
	if tx.KeyIndex == nil {
		return fmt.Errorf("key index is nil")
	}
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if (tx.CommitmentKey == nil && tx.EncryptionPublicKey == nil) ||
		*tx.KeyIndex < 1 || *tx.KeyIndex > types.KeyKeeperMaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex is not already used
	if len(process.EncryptionPublicKeys[*tx.KeyIndex]) > 0 ||
		len(process.CommitmentKeys[*tx.KeyIndex]) > 0 {
		return fmt.Errorf("key index %d already exists", tx.KeyIndex)
	}
	// TBD check that provided keys are correct (ed25519 for encryption and size for Commitment)
	return nil
}

func checkRevealProcessKeys(tx *models.AdminTx, process *models.Process) error {
	if tx.KeyIndex == nil {
		return fmt.Errorf("key index is nil")
	}
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if (tx.RevealKey == nil && tx.EncryptionPrivateKey == nil) ||
		*tx.KeyIndex < 1 || *tx.KeyIndex > types.KeyKeeperMaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex exists
	if len(process.EncryptionPublicKeys[*tx.KeyIndex]) < 1 ||
		len(process.CommitmentKeys[*tx.KeyIndex]) < 1 {
		return fmt.Errorf("key index %d does not exist", *tx.KeyIndex)
	}
	// check keys actually work
	if tx.EncryptionPrivateKey != nil {
		if priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", tx.EncryptionPrivateKey)); err == nil {
			pub := priv.Public().Bytes()
			if fmt.Sprintf("%x", pub) != process.EncryptionPublicKeys[*tx.KeyIndex] {
				log.Debugf("%x != %s", pub, process.EncryptionPublicKeys[*tx.KeyIndex])
				return fmt.Errorf("the provided private key does not match "+
					"with the stored public key for index %d", *tx.KeyIndex)
			}
		} else {
			return err
		}
	}
	if tx.RevealKey != nil {
		commitment := ethereum.HashRaw(tx.RevealKey)
		if fmt.Sprintf("%x", commitment) != process.CommitmentKeys[*tx.KeyIndex] {
			log.Debugf("%x != %s", commitment, process.CommitmentKeys[*tx.KeyIndex])
			return fmt.Errorf("the provided commitment reveal key does not match "+
				"with the stored for index %d", *tx.KeyIndex)
		}

	}
	return nil
}
