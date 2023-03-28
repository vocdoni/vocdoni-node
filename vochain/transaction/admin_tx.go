package transaction

import (
	"fmt"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func (t *TransactionHandler) AdminTxCheck(vtx *vochaintx.VochainTx) (ethereum.Address, error) {
	if vtx.SignedBody == nil || vtx.Signature == nil || vtx.Tx == nil {
		return ethereum.Address{}, ErrNilTx
	}
	tx := vtx.Tx.GetAdmin()

	// check vtx.Signature available and extract address
	if vtx.Signature == nil || tx == nil || vtx.SignedBody == nil {
		return ethereum.Address{}, fmt.Errorf("missing signature or transaction body")
	}
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return ethereum.Address{}, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	addr, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return ethereum.Address{}, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	log.Debugw("checking admin tx", "addr", addr.Hex(), "tx", log.FormatProto(tx))

	switch tx.Txtype {
	case models.TxType_ADD_PROCESS_KEYS, models.TxType_REVEAL_PROCESS_KEYS:
		if tx.ProcessId == nil {
			return ethereum.Address{}, fmt.Errorf("missing processId on adminTx")
		}

		// check process exists
		process, err := t.state.Process(tx.ProcessId, false)
		if err != nil {
			return ethereum.Address{}, err
		}
		if process == nil {
			return ethereum.Address{}, fmt.Errorf("process with id (%x) does not exist", tx.ProcessId)
		}

		// check if process actually requires keys
		if !process.EnvelopeType.EncryptedVotes && !process.EnvelopeType.Anonymous {
			return ethereum.Address{}, fmt.Errorf("process does not require keys")
		}

		// check if sender authorized (a current validator)
		validator, err := t.state.Validator(addr, false)
		if validator == nil || err != nil {
			if err != nil {
				return ethereum.Address{}, err
			}
			return ethereum.Address{}, fmt.Errorf(
				"not a validator, unauthorized to execute admin tx key management, address: %s", addr.Hex())
		}

		// if validator keyIndex is zero, it is disabled
		if validator.KeyIndex == 0 {
			return ethereum.Address{}, fmt.Errorf("validator key management is disabled")
		}

		// check keyIndex is not nil
		if tx.KeyIndex == nil {
			return ethereum.Address{}, fmt.Errorf("missing keyIndex on adminTx")
		}

		// check keyIndex in the transaction is correct for the validator
		if *tx.KeyIndex != validator.KeyIndex {
			return ethereum.Address{}, fmt.Errorf("transaction key index does not match with validator index")
		}

		// get current height
		height := t.state.CurrentHeight()

		// Specific checks
		switch tx.Txtype {
		case models.TxType_ADD_PROCESS_KEYS:
			// endblock is always greater than start block so that case is also included here
			if height > process.StartBlock {
				return ethereum.Address{}, fmt.Errorf(
					"cannot add keys to a process that has started or finished (%s)", process.Status.String())
			}
			// process is not canceled
			if process.Status == models.ProcessStatus_CANCELED ||
				process.Status == models.ProcessStatus_ENDED ||
				process.Status == models.ProcessStatus_RESULTS {
				return ethereum.Address{}, fmt.Errorf("cannot add process keys to a %s process", process.Status)
			}
			if len(process.EncryptionPublicKeys[tx.GetKeyIndex()]) > 0 {
				return ethereum.Address{}, fmt.Errorf("keys for process %x already added", tx.ProcessId)
			}
			// check included keys and keyindex are valid
			if err := checkAddProcessKeys(tx, process); err != nil {
				return ethereum.Address{}, err
			}
		case models.TxType_REVEAL_PROCESS_KEYS:
			// check process is finished
			if height < process.StartBlock+process.BlockCount &&
				!(process.Status == models.ProcessStatus_ENDED ||
					process.Status == models.ProcessStatus_CANCELED) {
				return ethereum.Address{}, fmt.Errorf("cannot reveal keys before the process is finished")
			}
			if len(process.EncryptionPrivateKeys[tx.GetKeyIndex()]) > 0 {
				return ethereum.Address{}, fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check the keys are valid
			if err := checkRevealProcessKeys(tx, process); err != nil {
				return ethereum.Address{}, err
			}
		}
	case models.TxType_ADD_ORACLE:
		err := t.state.VerifyTreasurer(addr, tx.Nonce)
		if err != nil {
			return ethereum.Address{}, fmt.Errorf("tx sender not authorized: %w", err)
		}
		// check not empty
		if len(tx.Address) != types.EthereumAddressSize {
			return ethereum.Address{}, fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		oracles, err := t.state.Oracles(false)
		if err != nil {
			return ethereum.Address{}, fmt.Errorf("cannot get oracles")
		}
		for idx, oracle := range oracles {
			if oracle == ethereum.AddrFromBytes(tx.Address) {
				return ethereum.Address{}, fmt.Errorf("oracle already added to oracle list at position %d", idx)
			}
		}
	case models.TxType_REMOVE_ORACLE:
		err := t.state.VerifyTreasurer(addr, tx.Nonce)
		if err != nil {
			return ethereum.Address{}, fmt.Errorf("tx sender not authorized: %w", err)
		}
		// check not empty
		if len(tx.Address) != types.EthereumAddressSize {
			return ethereum.Address{}, fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		oracles, err := t.state.Oracles(false)
		if err != nil {
			return ethereum.Address{}, fmt.Errorf("cannot get oracles")
		}
		var found bool
		for _, oracle := range oracles {
			if oracle == ethereum.AddrFromBytes(tx.Address) {
				found = true
				break
			}
		}
		if !found {
			return ethereum.Address{}, fmt.Errorf("cannot remove oracle, not found")
		}
	default:
		return ethereum.Address{}, fmt.Errorf("tx not supported")
	}
	return ethereum.Address(addr), nil
}
