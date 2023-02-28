package transaction

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// AdminTxCheck is an abstraction of ABCI checkTx for an admin transaction
func (t *TransactionHandler) AdminTxCheck(vtx *vochaintx.VochainTx) (common.Address, error) {
	if vtx.SignedBody == nil || vtx.Signature == nil || vtx.Tx == nil {
		return common.Address{}, ErrNilTx
	}
	tx := vtx.Tx.GetAdmin()
	// check vtx.Signature available
	if vtx.Signature == nil || tx == nil || vtx.SignedBody == nil {
		return common.Address{}, fmt.Errorf("missing vtx.Signature and/or admin transaction")
	}

	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	addr, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	log.Debugf("checking admin signed tx %s by addr %x", log.FormatProto(tx), addr)

	switch tx.Txtype {
	// TODO: @jordipainan make keykeeper independent of oracles
	case models.TxType_ADD_PROCESS_KEYS, models.TxType_REVEAL_PROCESS_KEYS:
		if tx.ProcessId == nil {
			return common.Address{}, fmt.Errorf("missing processId on AdminTxCheck")
		}
		// check process exists
		process, err := t.state.Process(tx.ProcessId, false)
		if err != nil {
			return common.Address{}, err
		}
		if process == nil {
			return common.Address{}, fmt.Errorf("process with id (%x) does not exist", tx.ProcessId)
		}
		// check process actually requires keys
		if !process.EnvelopeType.EncryptedVotes && !process.EnvelopeType.Anonymous {
			return common.Address{}, fmt.Errorf("process does not require keys")
		}
		// check if sender authorized
		if !t.state.IsValidator(addr.Hex()) {
			return common.Address{}, fmt.Errorf(
				"not a validator, unauthorized to perform key management, address: %s", addr.Hex())
		}
		// TODO: @pau add cost for this transaction so miners cannot spam

		// get current height
		height := t.state.CurrentHeight()
		// Specific checks
		switch tx.Txtype {
		case models.TxType_ADD_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return common.Address{}, fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// endblock is always greater than start block so that case is also included here
			if height > process.StartBlock {
				return common.Address{}, fmt.Errorf("cannot add process keys to a process that has started or finished status (%s)", process.Status.String())
			}
			// process is not canceled
			if process.Status == models.ProcessStatus_CANCELED ||
				process.Status == models.ProcessStatus_ENDED ||
				process.Status == models.ProcessStatus_RESULTS {
				return common.Address{}, fmt.Errorf("cannot add process keys to a %s process", process.Status)
			}
			if len(process.EncryptionPublicKeys[tx.GetKeyIndex()]) > 0 {
				return common.Address{}, fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check included keys and keyindex are valid
			if err := checkAddProcessKeys(tx, process); err != nil {
				return common.Address{}, err
			}
		case models.TxType_REVEAL_PROCESS_KEYS:
			if tx.KeyIndex == nil {
				return common.Address{}, fmt.Errorf("missing keyIndex on AdminTxCheck")
			}
			// check process is finished
			if height < process.StartBlock+process.BlockCount &&
				!(process.Status == models.ProcessStatus_ENDED ||
					process.Status == models.ProcessStatus_CANCELED) {
				return common.Address{}, fmt.Errorf("cannot reveal keys before the process is finished")
			}
			if len(process.EncryptionPrivateKeys[tx.GetKeyIndex()]) > 0 {
				return common.Address{}, fmt.Errorf("keys for process %x already revealed", tx.ProcessId)
			}
			// check the keys are valid
			if err := checkRevealProcessKeys(tx, process); err != nil {
				return common.Address{}, err
			}
		}
	case models.TxType_ADD_ORACLE:
		err := t.state.VerifyTreasurer(addr, tx.Nonce)
		if err != nil {
			return common.Address{}, fmt.Errorf("tx sender not authorized: %w", err)
		}
		// check not empty, correct length and not 0x0 addr
		if (bytes.Equal(tx.Address, []byte{})) ||
			(len(tx.Address) != types.EthereumAddressSize) ||
			(bytes.Equal(tx.Address, common.Address{}.Bytes())) {
			return common.Address{}, fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		oracles, err := t.state.Oracles(false)
		if err != nil {
			return common.Address{}, fmt.Errorf("cannot get oracles")
		}
		for idx, oracle := range oracles {
			if oracle == common.BytesToAddress(tx.Address) {
				return common.Address{}, fmt.Errorf("oracle already added to oracle list at position %d", idx)
			}
		}
	case models.TxType_REMOVE_ORACLE:
		err := t.state.VerifyTreasurer(addr, tx.Nonce)
		if err != nil {
			return common.Address{}, fmt.Errorf("tx sender not authorized: %w", err)
		}
		// check not empty, correct length and not 0x0 addr
		if (bytes.Equal(tx.Address, []byte{})) ||
			(len(tx.Address) != types.EthereumAddressSize) ||
			(bytes.Equal(tx.Address, common.Address{}.Bytes())) {
			return common.Address{}, fmt.Errorf("invalid oracle address: %x", tx.Address)
		}
		oracles, err := t.state.Oracles(false)
		if err != nil {
			return common.Address{}, fmt.Errorf("cannot get oracles")
		}
		var found bool
		for _, oracle := range oracles {
			if oracle == common.BytesToAddress(tx.Address) {
				found = true
				break
			}
		}
		if !found {
			return common.Address{}, fmt.Errorf("cannot remove oracle, not found")
		}
	default:
		return common.Address{}, fmt.Errorf("tx not supported")
	}
	return addr, nil
}
