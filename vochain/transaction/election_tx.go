package transaction

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/nacl"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/processid"
	"go.vocdoni.io/dvote/vochain/results"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// NewProcessTxCheck is an abstraction of ABCI checkTx for creating a new process
func (t *TransactionHandler) NewProcessTxCheck(vtx *vochaintx.VochainTx,
	forCommit bool) (*models.Process, common.Address, error) {
	if vtx.Tx == nil || vtx.Signature == nil || vtx.SignedBody == nil {
		return nil, common.Address{}, ErrNilTx
	}
	tx := vtx.Tx.GetNewProcess()
	if tx.Process == nil {
		return nil, common.Address{}, fmt.Errorf("new process data is empty")
	}
	// basic required fields check
	if tx.Process.VoteOptions == nil || tx.Process.EnvelopeType == nil || tx.Process.Mode == nil {
		return nil, common.Address{}, fmt.Errorf("missing required fields (voteOptions, envelopeType or processMode)")
	}
	if tx.Process.VoteOptions.MaxCount == 0 {
		return nil, common.Address{}, fmt.Errorf("missing vote maxCount parameter")
	}
	// Check for maxCount/maxValue overflows
	if tx.Process.VoteOptions.MaxCount > results.MaxQuestions || tx.Process.VoteOptions.MaxValue > results.MaxOptions {
		return nil, common.Address{},
			fmt.Errorf("maxCount or maxValue overflows hardcoded maximums (%d, %d). Received (%d, %d)",
				results.MaxQuestions, results.MaxOptions, tx.Process.VoteOptions.MaxCount, tx.Process.VoteOptions.MaxValue)
	}
	if !(tx.Process.GetStatus() == models.ProcessStatus_READY || tx.Process.GetStatus() == models.ProcessStatus_PAUSED) {
		return nil, common.Address{}, fmt.Errorf("status must be READY or PAUSED")
	}
	// check vtx.Signature available
	if vtx.Signature == nil || tx == nil || vtx.SignedBody == nil {
		return nil, common.Address{}, fmt.Errorf("missing vtx.Signature or new process transaction")
	}
	// start and block count sanity check
	// if startBlock is zero or one, the process will be enabled on the next block
	if tx.Process.StartBlock == 0 || tx.Process.StartBlock == 1 {
		tx.Process.StartBlock = t.state.CurrentHeight() + 1
	} else if tx.Process.StartBlock < t.state.CurrentHeight() {
		return nil, common.Address{}, fmt.Errorf(
			"cannot add process with start block lower than or equal to the current height")
	}
	if tx.Process.BlockCount <= 0 {
		return nil, common.Address{}, fmt.Errorf(
			"cannot add process with duration lower than or equal to the current height")
	}

	// check MaxCensusSize is properly set and within the allowed range
	if tx.Process.GetMaxCensusSize() == 0 {
		log.Warnf("maxCensusSize is zero")
		return nil, common.Address{}, fmt.Errorf("maxCensusSize is zero")
	}
	maxProcessSize, err := t.state.MaxProcessSize()
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("cannot get maxProcessSize: %w", err)
	}
	if maxProcessSize > 0 && tx.Process.GetMaxCensusSize() > maxProcessSize {
		return nil, common.Address{},
			fmt.Errorf("maxCensusSize is greater than the maximum allowed (%d)", maxProcessSize)
	}

	// check tx cost
	cost, err := t.state.TxCost(models.TxType_NEW_PROCESS, false)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("cannot get NewProcessTx transaction cost: %w", err)
	}
	addr, acc, err := t.state.AccountFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("could not get account: %w", err)
	}
	if addr == nil {
		return nil, common.Address{}, fmt.Errorf("cannot get account from vtx.Signature, nil result")
	}
	// check balance and nonce
	if acc.Balance < cost {
		return nil, common.Address{}, vstate.ErrNotEnoughBalance
	}
	if acc.Nonce != tx.Nonce {
		return nil, common.Address{}, vstate.ErrAccountNonceInvalid
	}

	// if organization ID is not set, use the sender address
	if tx.Process.EntityId == nil {
		tx.Process.EntityId = addr.Bytes()
	}

	// check if the sender is an Oracle or a Delegate of the organization
	isOracle, err := t.state.IsOracle(*addr)
	if err != nil {
		return nil, common.Address{}, err
	}

	// check if process entityID matches tx sender
	if !bytes.Equal(tx.Process.EntityId, addr.Bytes()) && !isOracle {
		// if not oracle check delegate
		entityAddress := common.BytesToAddress(tx.Process.EntityId)
		entityAccount, err := t.state.GetAccount(entityAddress, false)
		if err != nil {
			return nil, common.Address{}, fmt.Errorf(
				"cannot get organization account for checking if the sender is a delegate: %w", err,
			)
		}
		if entityAccount == nil {
			return nil, common.Address{}, fmt.Errorf("organization account %s does not exists", addr.Hex())
		}
		if !entityAccount.IsDelegate(*addr) {
			return nil, common.Address{}, fmt.Errorf(
				"account %s unauthorized to create a new election on this organization", addr.Hex())
		}
	}

	// if no Oracle, build the processID (Oracles are allowed to use any processID)
	if !isOracle || tx.Process.ProcessId == nil {
		// if Oracle but processID empty, switch the entityID temporary to the Oracle address
		// this way we ensure the account creating the process exists
		entityID := tx.Process.EntityId
		if isOracle {
			tx.Process.EntityId = addr.Bytes()
		}
		pid, err := processid.BuildProcessID(tx.Process, t.state)
		if err != nil {
			return nil, common.Address{}, fmt.Errorf("cannot build processID: %w", err)
		}
		tx.Process.ProcessId = pid.Marshal()
		// restore original entityID
		tx.Process.EntityId = entityID
	}

	// check if process already exists
	_, err = t.state.Process(tx.Process.ProcessId, false)
	if err == nil {
		return nil, common.Address{}, fmt.Errorf("process with id (%x) already exists", tx.Process.ProcessId)
	}

	if tx.Process.Mode.PreRegister &&
		(tx.Process.MaxCensusSize == 0 || tx.Process.GetMaxCensusSize() <= 0) {
		return nil, common.Address{}, fmt.Errorf("pre-register mode requires setting " +
			"maxCensusSize to be > 0")
	}
	if tx.Process.Mode.PreRegister && tx.Process.EnvelopeType.Anonymous {
		if tx.Process.MaxCensusSize == 0 {
			return nil, common.Address{}, fmt.Errorf("maxCensusSize is not provided")
		}
		if tx.Process.GetMaxCensusSize() > uint64(t.ZkCircuit.Config.Levels) {
			return nil, common.Address{}, fmt.Errorf("maxCensusSize for anonymous envelope "+
				"cannot be bigger than the number of levels of the circuit (%d)",
				t.ZkCircuit.Config.Levels)
		}
	}

	// TODO: Enable support for PreRegiser without Anonymous.  Figure out
	// all the required changes to support a process with a rolling census
	// that is not Anonymous.
	if tx.Process.EnvelopeType.Serial {
		return nil, common.Address{}, fmt.Errorf("serial process not yet implemented")
	}

	if tx.Process.EnvelopeType.EncryptedVotes {
		// We consider the zero value as nil for security
		tx.Process.EncryptionPublicKeys = make([]string, types.KeyKeeperMaxKeyIndex)
		tx.Process.EncryptionPrivateKeys = make([]string, types.KeyKeeperMaxKeyIndex)
	}
	return tx.Process, *addr, nil
}

// SetProcessTxCheck is an abstraction of ABCI checkTx for canceling an existing process
func (t *TransactionHandler) SetProcessTxCheck(vtx *vochaintx.VochainTx, forCommit bool) (common.Address, error) {
	// check vtx.Signature available
	if vtx.Signature == nil || vtx.Tx == nil || vtx.SignedBody == nil {
		return common.Address{}, ErrNilTx
	}
	tx := vtx.Tx.GetSetProcess()
	// get tx cost
	cost, err := t.state.TxCost(tx.Txtype, false)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot get %s transaction cost: %w", tx.Txtype.String(), err)
	}
	addr, acc, err := t.state.AccountFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return common.Address{}, err
	}
	// check balance and nonce
	if acc.Balance < cost {
		return common.Address{}, vstate.ErrNotEnoughBalance
	}
	if acc.Nonce != tx.Nonce {
		return common.Address{}, vstate.ErrAccountNonceInvalid
	}
	// get process
	process, err := t.state.Process(tx.ProcessId, false)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot get process %x: %w", tx.ProcessId, err)
	}
	// check process entityID matches tx sender
	isOracle := false
	if !bytes.Equal(process.EntityId, addr.Bytes()) {
		// Check if the transaction comes from an oracle
		// Oracles can create processes with any entityID
		isOracle, err = t.state.IsOracle(*addr)
		if err != nil {
			return common.Address{}, err
		}
		if !isOracle {
			// check if delegate
			entityIDAddress := common.BytesToAddress(process.EntityId)
			entityIDAccount, err := t.state.GetAccount(entityIDAddress, true)
			if err != nil {
				return common.Address{}, fmt.Errorf(
					"cannot get entityID account for checking if the sender is a delegate: %w", err,
				)
			}
			if !entityIDAccount.IsDelegate(*addr) {
				return common.Address{}, fmt.Errorf(
					"unauthorized to set process status, recovered addr is %s", addr.Hex(),
				)
			} // is delegate
		} // is oracle
	}
	switch tx.Txtype {
	case models.TxType_SET_PROCESS_RESULTS:
		if !isOracle {
			return common.Address{}, fmt.Errorf("only oracles can execute set process results transaction")
		}
		if acc.Balance < cost {
			return common.Address{}, vstate.ErrNotEnoughBalance
		}
		if acc.Nonce != tx.Nonce {
			return common.Address{}, vstate.ErrAccountNonceInvalid
		}
		results := tx.GetResults()
		if !bytes.Equal(results.OracleAddress, addr.Bytes()) {
			return common.Address{}, fmt.Errorf("cannot set results, oracle address provided in results does not match")
		}
		return *addr, t.state.SetProcessResults(process.ProcessId, results, false)
	case models.TxType_SET_PROCESS_STATUS:
		return *addr, t.state.SetProcessStatus(process.ProcessId, tx.GetStatus(), false)
	case models.TxType_SET_PROCESS_CENSUS:
		return *addr, t.state.SetProcessCensus(process.ProcessId, tx.GetCensusRoot(), tx.GetCensusURI(), false)
	default:
		return common.Address{}, fmt.Errorf("unknown setProcess tx type: %s", tx.Txtype)
	}
}

// RegisterKeyTxCheck validates a registerKeyTx transaction against the state
func (t *TransactionHandler) RegisterKeyTxCheck(vtx *vochaintx.VochainTx, forCommit bool) error {
	if vtx.Signature == nil || vtx.Tx == nil || vtx.SignedBody == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetRegisterKey()
	// Sanity checks
	if tx == nil {
		return fmt.Errorf("register key transaction is nil")
	}
	process, err := t.state.Process(tx.ProcessId, false)
	if err != nil {
		return fmt.Errorf("cannot fetch processId: %w", err)
	}
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return fmt.Errorf("process %x malformed", tx.ProcessId)
	}
	if t.state.CurrentHeight() >= process.StartBlock {
		return fmt.Errorf("process %x already started", tx.ProcessId)
	}
	if !(process.Mode.PreRegister && process.EnvelopeType.Anonymous) {
		return fmt.Errorf("RegisterKeyTx only supported with " +
			"Mode.PreRegister and EnvelopeType.Anonymous")
	}
	if process.Status != models.ProcessStatus_READY {
		return fmt.Errorf("process %x not in READY state", tx.ProcessId)
	}
	if tx.Proof == nil {
		return fmt.Errorf("proof missing on registerKeyTx")
	}
	if vtx.Signature == nil {
		return fmt.Errorf("vtx.Signature missing on voteTx")
	}
	if len(tx.NewKey) != 32 {
		return fmt.Errorf("newKey wrong size")
	}
	// Verify that we are not over maxCensusSize
	censusSize, err := t.state.GetRollingCensusSize(tx.ProcessId, false)
	if err != nil {
		return err
	}
	if censusSize >= process.MaxCensusSize {
		return fmt.Errorf("maxCensusSize already reached")
	}

	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	var addr common.Address
	addr, err = ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
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
		return fmt.Errorf("proof not valid: %w", err)
	}
	if !valid {
		return fmt.Errorf("proof not valid")
	}

	// Validate that this user is not registering more keys than possible
	// with the users weight.
	usedWeight, err := t.state.GetPreRegisterAddrUsedWeight(process.ProcessId, addr)
	if err != nil {
		return fmt.Errorf("cannot get address used weight: %w", err)
	}
	txWeight, ok := new(big.Int).SetString(tx.Weight, 10)
	if !ok {
		return fmt.Errorf("cannot parse tx weight %s", txWeight)
	}
	usedWeight.Add(usedWeight, txWeight)

	// TODO: In order to support tx.Weight != 1 for anonymous voting, we
	// need to add the weight to the leaf in the CensusPoseidon Tree, and
	// also add the weight as a public input in the circuit to verify it anonymously.
	// The following check ensures that weight != 1 is not used, once the above is
	// implemented we can remove it
	if usedWeight.Cmp(bigOne) != 0 {
		return fmt.Errorf("weight != 1 is not yet supported, received %s, used weight: %s", txWeight, usedWeight)
	}

	if usedWeight.Cmp(weight) > 0 {
		return fmt.Errorf("cannot register more keys: "+
			"usedWeight + RegisterKey.Weight (%v) > proof weight (%v)",
			usedWeight, weight)
	}
	if forCommit {
		if err := t.state.SetPreRegisterAddrUsedWeight(process.ProcessId, addr, usedWeight); err != nil {
			return fmt.Errorf("cannot set address used weight: %w", err)
		}
	}

	// TODO: Add cache like in VoteEnvelopeCheck for the registered key so that:
	// A. We can skip the proof verification when forCommiting
	// B. We can detect invalid key registration (due to no more weight
	//    available) at mempool tx insertion

	return nil
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
