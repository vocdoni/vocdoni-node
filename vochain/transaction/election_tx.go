package transaction

import (
	"bytes"
	"fmt"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/processid"
	"go.vocdoni.io/dvote/vochain/results"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/state/electionprice"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func (t *TransactionHandler) NewProcessTxCheck(vtx *vochaintx.Tx) (*models.Process, ethereum.Address, error) {
	if vtx.Tx == nil || vtx.Signature == nil || vtx.SignedBody == nil {
		return nil, ethereum.Address{}, ErrNilTx
	}
	tx := vtx.Tx.GetNewProcess()
	if tx.Process == nil {
		return nil, ethereum.Address{}, fmt.Errorf("new process data is empty")
	}
	// basic required fields check
	if tx.Process.VoteOptions == nil || tx.Process.EnvelopeType == nil || tx.Process.Mode == nil {
		return nil, ethereum.Address{}, fmt.Errorf("missing required fields (voteOptions, envelopeType or processMode)")
	}
	if tx.Process.VoteOptions.MaxCount == 0 {
		return nil, ethereum.Address{}, fmt.Errorf("missing vote maxCount parameter")
	}
	// check for maxCount/maxValue overflows
	if tx.Process.VoteOptions.MaxCount > results.MaxQuestions {
		return nil, ethereum.Address{},
			fmt.Errorf("maxCount overflows (%d, %d)",
				results.MaxQuestions, tx.Process.VoteOptions.MaxCount)
	}
	if !(tx.Process.GetStatus() == models.ProcessStatus_READY || tx.Process.GetStatus() == models.ProcessStatus_PAUSED) {
		return nil, ethereum.Address{}, fmt.Errorf("status must be READY or PAUSED")
	}
	// check vtx.Signature available
	if vtx.Signature == nil || tx == nil || vtx.SignedBody == nil {
		return nil, ethereum.Address{}, fmt.Errorf("missing vtx.Signature or new process transaction")
	}

	// time based process (duration and start time)
	if tx.Process.Duration > 0 {
		currentTimestamp, err := t.state.Timestamp(false)
		if err != nil {
			return nil, ethereum.Address{}, fmt.Errorf("cannot get current timestamp: %w", err)
		}
		// if start time is zero or one, the process will be enabled on the next block
		if tx.Process.StartTime == 0 {
			tx.Process.StartTime = currentTimestamp
		}
		if tx.Process.StartTime < currentTimestamp {
			return nil, ethereum.Address{}, fmt.Errorf("cannot add process with start time lower than the current timestamp")
		}
		if tx.Process.StartTime+tx.Process.Duration < currentTimestamp {
			return nil, ethereum.Address{}, fmt.Errorf("cannot add process with duration lower than the current timestamp")
		}
	} else {
		// block based process (block count and start block)
		// TODO: remove due deprecation
		if tx.Process.StartBlock == 0 || tx.Process.StartBlock == 1 {
			tx.Process.StartBlock = t.state.CurrentHeight() + 1
		} else if tx.Process.StartBlock < t.state.CurrentHeight() {
			return nil, ethereum.Address{}, fmt.Errorf(
				"cannot add process with start block lower than or equal to the current height")
		}
		if tx.Process.BlockCount <= 0 {
			return nil, ethereum.Address{}, fmt.Errorf(
				"cannot add process with duration lower than or equal to the current height")
		}
	}

	// check MaxCensusSize is properly set and within the allowed range
	txMaxCensusSize := tx.Process.GetMaxCensusSize()
	if txMaxCensusSize == 0 {
		return nil, ethereum.Address{}, fmt.Errorf("maxCensusSize is zero")
	}
	maxProcessSize, err := t.state.MaxProcessSize()
	if err != nil {
		return nil, ethereum.Address{}, fmt.Errorf("cannot get maxProcessSize: %w", err)
	}
	if maxProcessSize > 0 && txMaxCensusSize > maxProcessSize {
		return nil, ethereum.Address{},
			fmt.Errorf("maxCensusSize is greater than the maximum allowed (%d)", maxProcessSize)
	}
	// check that the census size is not bigger than the circuit levels
	if tx.Process.EnvelopeType.Anonymous && !circuit.Global().Config.SupportsCensusSize(txMaxCensusSize) {
		return nil, ethereum.Address{}, fmt.Errorf("maxCensusSize for anonymous envelope "+
			"cannot be bigger than the number of levels of the circuit (max:%d provided:%d)",
			circuit.Global().Config.MaxCensusSize().Int64(), txMaxCensusSize)
	}

	// check signature
	addr, acc, err := t.state.AccountFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return nil, ethereum.Address{}, fmt.Errorf("could not get account: %w", err)
	}
	if addr == nil {
		return nil, ethereum.Address{}, fmt.Errorf("cannot get account from vtx.Signature, nil result")
	}

	// get Tx cost, since it is a new process, we should use the election price calculator
	cost := t.txElectionCostFromProcess(tx.Process)

	// check balance and nonce
	if acc.Balance < cost {
		return nil, ethereum.Address{}, fmt.Errorf("%w: required %d, got %d", vstate.ErrNotEnoughBalance, cost, acc.Balance)
	}

	// if organization ID is not set, use the sender address
	if tx.Process.EntityId == nil {
		tx.Process.EntityId = addr.Bytes()
	} else if !bytes.Equal(tx.Process.EntityId, addr.Bytes()) { // check if process entityID matches tx sender
		// check for a delegate
		entityAddress := ethereum.AddrFromBytes(tx.Process.EntityId)
		entityAccount, err := t.state.GetAccount(entityAddress, false)
		if err != nil {
			return nil, ethereum.Address{}, fmt.Errorf(
				"cannot get organization account for checking if the sender is a delegate: %w", err,
			)
		}
		if entityAccount == nil {
			return nil, ethereum.Address{}, fmt.Errorf("organization account %s does not exists", addr.Hex())
		}
		if !entityAccount.IsDelegate(*addr) {
			return nil, ethereum.Address{}, fmt.Errorf(
				"account %s unauthorized to create a new election on this organization", addr.Hex())
		}
	}

	// build the deterministic process ID
	pid, err := processid.BuildProcessID(tx.Process, t.state)
	if err != nil {
		return nil, ethereum.Address{}, fmt.Errorf("cannot build processID: %w", err)
	}
	tx.Process.ProcessId = pid.Marshal()

	// TODO: Enable support for PreRegiser without Anonymous.  Figure out
	// all the required changes to support a process with a rolling census
	// that is not Anonymous.
	if tx.Process.EnvelopeType.Serial {
		return nil, ethereum.Address{}, fmt.Errorf("serial process not yet implemented")
	}

	if tx.Process.EnvelopeType.EncryptedVotes {
		// We consider the zero value as nil for security
		tx.Process.EncryptionPublicKeys = make([]string, types.KeyKeeperMaxKeyIndex)
		tx.Process.EncryptionPrivateKeys = make([]string, types.KeyKeeperMaxKeyIndex)
	}
	return tx.Process, ethereum.Address(*addr), nil
}

// SetProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func (t *TransactionHandler) SetProcessTxCheck(vtx *vochaintx.Tx) (ethereum.Address, error) {
	// check vtx.Signature available
	if vtx.Signature == nil || vtx.Tx == nil || vtx.SignedBody == nil {
		return ethereum.Address{}, ErrNilTx
	}
	tx := vtx.Tx.GetSetProcess()
	// get tx cost
	cost, err := t.state.TxBaseCost(tx.Txtype, false)
	if err != nil {
		return ethereum.Address{}, fmt.Errorf("cannot get %s transaction cost: %w", tx.Txtype, err)
	}
	addr, acc, err := t.state.AccountFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return ethereum.Address{}, err
	}
	// check balance and nonce
	if acc.Balance < cost {
		return ethereum.Address{}, vstate.ErrNotEnoughBalance
	}
	// get process
	process, err := t.state.Process(tx.ProcessId, false)
	if err != nil {
		return ethereum.Address{}, fmt.Errorf("cannot get process %x: %w", tx.ProcessId, err)
	}
	// check process entityID matches tx sender
	if !bytes.Equal(process.EntityId, addr.Bytes()) {
		// check if delegate
		entityIDAddress := ethereum.AddrFromBytes(process.EntityId)
		entityIDAccount, err := t.state.GetAccount(entityIDAddress, true)
		if err != nil {
			return ethereum.Address{}, fmt.Errorf(
				"cannot get entityID account for checking if the sender is a delegate: %w", err,
			)
		}
		if !entityIDAccount.IsDelegate(*addr) {
			return ethereum.Address{}, fmt.Errorf(
				"unauthorized to set process status, recovered addr is %s", addr.Hex(),
			)
		} // is delegate
	}
	switch tx.Txtype {
	case models.TxType_SET_PROCESS_STATUS:
		if tx.GetStatus() == models.ProcessStatus_RESULTS {
			// Status can only be set to RESULTS by the internal logic of the blockchain (see IST controller).
			return ethereum.Address{}, fmt.Errorf("not authorized to set process status to RESULTS")
		}
		return ethereum.Address(*addr), t.state.SetProcessStatus(process.ProcessId, tx.GetStatus(), false)
	case models.TxType_SET_PROCESS_CENSUS:
		return ethereum.Address(*addr), t.state.SetProcessCensus(process.ProcessId, tx.GetCensusRoot(), tx.GetCensusURI(), false)
	default:
		return ethereum.Address{}, fmt.Errorf("unknown setProcess tx type: %s", tx.Txtype)
	}
}

func checkAddProcessKeys(tx *models.AdminTx, process *models.Process) error {
	if tx == nil {
		return ErrNilTx
	}
	if tx.KeyIndex == nil {
		return fmt.Errorf("key index is nil")
	}
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if (tx.EncryptionPublicKey == nil) ||
		tx.GetKeyIndex() < 1 || tx.GetKeyIndex() > types.KeyKeeperMaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex is not already used
	if len(process.EncryptionPublicKeys[tx.GetKeyIndex()]) > 0 {
		return fmt.Errorf("key index %d already exists", tx.KeyIndex)
	}
	return nil
}

func checkRevealProcessKeys(tx *models.AdminTx, process *models.Process) error {
	if tx == nil {
		return ErrNilTx
	}
	if process == nil {
		return fmt.Errorf("process is nil")
	}
	if tx.KeyIndex == nil {
		return fmt.Errorf("key index is nil")
	}
	// check if at leat 1 key is provided and the keyIndex do not over/under flow
	if (tx.EncryptionPrivateKey == nil) ||
		tx.GetKeyIndex() < 1 || tx.GetKeyIndex() > types.KeyKeeperMaxKeyIndex {
		return fmt.Errorf("no keys provided or invalid key index")
	}
	// check if provided keyIndex exists
	if len(process.EncryptionPublicKeys[tx.GetKeyIndex()]) < 1 {
		return fmt.Errorf("key index %d does not exist", tx.GetKeyIndex())
	}
	// check keys actually work
	if tx.EncryptionPrivateKey != nil {
		if priv, err := nacl.DecodePrivate(fmt.Sprintf("%x", tx.EncryptionPrivateKey)); err == nil {
			pub := priv.Public().Bytes()
			if fmt.Sprintf("%x", pub) != process.EncryptionPublicKeys[tx.GetKeyIndex()] {
				log.Debugf("%x != %s", pub, process.EncryptionPublicKeys[tx.GetKeyIndex()])
				return fmt.Errorf("the provided private key does not match "+
					"with the stored public key for index %d", tx.GetKeyIndex())
			}
		} else {
			return err
		}
	}
	return nil
}

func (t *TransactionHandler) txElectionCostFromProcess(process *models.Process) uint64 {
	return t.state.ElectionPriceCalc.Price(&electionprice.ElectionParameters{
		MaxCensusSize:    process.GetMaxCensusSize(),
		ElectionDuration: process.BlockCount,
		EncryptedVotes:   process.GetEnvelopeType().EncryptedVotes,
		AnonymousVotes:   process.GetEnvelopeType().Anonymous,
		MaxVoteOverwrite: process.GetVoteOptions().MaxVoteOverwrites,
	})
}
