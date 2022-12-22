package state

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

const (
	// treasurerKey is the key representing the Treasurer entry on the Extra subtree
	treasurerKey = "treasurer"
)

// TxTypeCostToStateKey translates models.TxType to a string which the State uses
// as a key internally under the Extra tree
var (
	TxTypeCostToStateKey = map[models.TxType]string{
		models.TxType_SET_PROCESS_STATUS:         "c_setProcessStatus",
		models.TxType_SET_PROCESS_CENSUS:         "c_setProcessCensus",
		models.TxType_SET_PROCESS_QUESTION_INDEX: "c_setProcessResults",
		models.TxType_SET_PROCESS_RESULTS:        "c_setProcessQuestionIndex",
		models.TxType_REGISTER_VOTER_KEY:         "c_registerKey",
		models.TxType_NEW_PROCESS:                "c_newProcess",
		models.TxType_SEND_TOKENS:                "c_sendTokens",
		models.TxType_SET_ACCOUNT_INFO_URI:       "c_setAccountInfoURI",
		models.TxType_CREATE_ACCOUNT:             "c_createAccount",
		models.TxType_ADD_DELEGATE_FOR_ACCOUNT:   "c_addDelegateForAccount",
		models.TxType_DEL_DELEGATE_FOR_ACCOUNT:   "c_delDelegateForAccount",
		models.TxType_COLLECT_FAUCET:             "c_collectFaucet",
	}
	ErrTxCostNotFound = fmt.Errorf("transaction cost is not set")
)

// SetTreasurer saves the Treasurer address to the state
func (v *State) SetTreasurer(address common.Address, nonce uint32) error {
	tBytes, err := proto.Marshal(
		&models.Treasurer{
			Address: address.Bytes(),
			Nonce:   nonce,
		},
	)
	if err != nil {
		return err
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet([]byte(treasurerKey), tBytes, StateTreeCfg(TreeExtra))
}

// Treasurer returns the address and the Treasurer nonce
// When committed is false, the operation is executed also on not yet commited
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last commited version.
func (v *State) Treasurer(committed bool) (*models.Treasurer, error) {
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	extraTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeExtra))
	if err != nil {
		return nil, err
	}
	var rawTreasurer []byte
	if rawTreasurer, err = extraTree.Get([]byte(treasurerKey)); err != nil {
		return nil, err
	}
	var t models.Treasurer
	if err := proto.Unmarshal(rawTreasurer, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// IsTreasurer returns true if the given address matches the Treasurer address
func (v *State) IsTreasurer(addr common.Address) (bool, error) {
	t, err := v.Treasurer(false)
	if err != nil {
		return false, err
	}
	return addr == common.BytesToAddress(t.Address), nil
}

// IncrementTreasurerNonce increments the treasurer nonce
func (v *State) IncrementTreasurerNonce() error {
	t, err := v.Treasurer(false)
	if err != nil {
		return fmt.Errorf("incrementTreasurerNonce(): %w", err)
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	t.Nonce++
	tBytes, err := proto.Marshal(t)
	if err != nil {
		return fmt.Errorf("incrementTreasurerNonce(): %w", err)
	}
	log.Debugf("incrementing treasurer nonce, new nonce is %d", t.Nonce)
	return v.Tx.DeepSet([]byte(treasurerKey), tBytes, StateTreeCfg(TreeExtra))
}

// VerifyTreasurer checks is an address is the treasurer and the
// nonce provided is the expected one
func (v *State) VerifyTreasurer(addr common.Address, txNonce uint32) error {
	// get treasurer
	treasurer, err := v.Treasurer(false)
	if err != nil {
		return fmt.Errorf("cannot check authorization")
	}
	log.Debugf("got treasurer addr %x", treasurer.Address)
	if !bytes.Equal(addr.Bytes(), treasurer.Address) {
		return fmt.Errorf("not authorized for executing admin transactions")
	}
	// check treasurer account
	if treasurer.Nonce != txNonce {
		return ErrAccountNonceInvalid
	}
	return nil
}

// SetTxCost sets the given transaction cost
func (v *State) SetTxCost(txType models.TxType, cost uint64) error {
	key, ok := TxTypeCostToStateKey[txType]
	if !ok {
		return fmt.Errorf("txType %v shouldn't cost anything", txType)
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	log.Debugf("setting tx cost %d for tx %s", cost, txType.String())
	costBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(costBytes[:], cost)
	return v.Tx.DeepSet([]byte(key), costBytes[:], StateTreeCfg(TreeExtra))
}

// TxCost returns the cost of a given transaction
// When committed is false, the operation is executed also on not yet commited
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last commited version.
func (v *State) TxCost(txType models.TxType, committed bool) (uint64, error) {
	key, ok := TxTypeCostToStateKey[txType]
	if !ok {
		return 0, fmt.Errorf("txType %v shouldn't cost anything", txType)
	}
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	extraTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeExtra))
	if err != nil {
		return 0, err
	}
	var cost []byte
	if cost, err = extraTree.Get([]byte(key)); err != nil {
		return 0, ErrTxCostNotFound
	}
	return binary.LittleEndian.Uint64(cost), nil
}

// FaucetNonce returns true if the key is found in the subtree
// key == hash(address, nonce)
// committed is relative to the state on which the function is executed
func (v *State) FaucetNonce(key []byte, committed bool) (bool, error) {
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	faucetNonceTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeFaucet))
	if err != nil {
		return false, err
	}
	var found bool
	if err := faucetNonceTree.Iterate(
		func(k, _ []byte) bool {
			if bytes.Equal(key, k) {
				found = true
				return true
			}
			return true
		},
	); err != nil {
		return false, err
	}
	return found, nil
}

// SetFaucetNonce stores an already used faucet nonce in the
// FaucetNonce subtree
func (v *State) SetFaucetNonce(key []byte) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(key, nil, StateTreeCfg(TreeFaucet))
}

// ConsumeFaucetPayload consumes a given faucet payload storing
// its key to the FaucetNonce tree so it can only be used once
func (v *State) ConsumeFaucetPayload(from common.Address, faucetPayload *models.FaucetPayload) error {
	// check faucet payload
	if faucetPayload == nil {
		return fmt.Errorf("invalid faucet payload")
	}
	// store faucet identifier
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	keyHash := ethereum.HashRaw(append(from.Bytes(), b...))
	if err := v.SetFaucetNonce(keyHash); err != nil {
		return err
	}
	log.Debugf("consuming faucet payload created by %s with amount %d and identifier %d (keyHash: %x)",
		from.String(),
		faucetPayload.Amount,
		faucetPayload.Identifier,
		keyHash,
	)
	return nil
}

// TransferBalance transfers balance from origin address to destination address,
// and updates the state with the new values (including nonce).
// If origin address acc is not enough, ErrNotEnoughBalance is returned.
func (v *State) TransferBalance(tx *vochaintx.TransferTokensMeta, burnTxCost bool) error {
	accFrom, err := v.GetAccount(tx.FromAddress, false)
	if err != nil {
		return err
	}
	if accFrom == nil {
		return ErrAccountNotExist
	}
	accTo, err := v.GetAccount(tx.ToAddress, false)
	if err != nil {
		return err
	}
	if accTo == nil {
		return ErrAccountNotExist
	}
	if err := accFrom.Transfer(accTo, tx.Amount); err != nil {
		return err
	}
	log.Debugf("transferring %d tokens from %s to %s", tx.Amount, tx.FromAddress.String(), tx.ToAddress.String())
	if err := v.SetAccount(tx.FromAddress, accFrom); err != nil {
		return err
	}
	if err := v.SetAccount(tx.ToAddress, accTo); err != nil {
		return err
	}
	if !burnTxCost {
		for _, l := range v.eventListeners {
			l.OnTransferTokens(&vochaintx.TransferTokensMeta{
				FromAddress: tx.FromAddress,
				ToAddress:   tx.ToAddress,
				Amount:      tx.Amount,
				TxHash:      tx.TxHash,
				Height:      uint64(v.CurrentHeight()),
			})
		}
	}
	return nil
}

// MintBalance increments the existing acc of address by amount
func (v *State) MintBalance(tx *vochaintx.TransferTokensMeta) error {
	if tx.Amount == 0 {
		return fmt.Errorf("cannot mint a zero amount balance")
	}
	acc, err := v.GetAccount(tx.ToAddress, false)
	if err != nil {
		return fmt.Errorf("mintBalance: %w", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	if acc.Balance+tx.Amount <= acc.Balance {
		return ErrBalanceOverflow
	}
	acc.Balance += tx.Amount
	log.Debugf("minting %d tokens to account %s", tx.Amount, tx.ToAddress.String())
	if err := v.SetAccount(tx.ToAddress, acc); err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnTransferTokens(&vochaintx.TransferTokensMeta{
			FromAddress: tx.FromAddress,
			ToAddress:   tx.ToAddress,
			Amount:      tx.Amount,
			TxHash:      tx.TxHash,
			Height:      uint64(v.CurrentHeight()),
		})
	}
	return nil
}

func (v *State) InitChainMintBalance(to common.Address, amount uint64) error {
	if amount == 0 {
		return nil
	}
	acc, err := v.GetAccount(to, false)
	if err != nil {
		return fmt.Errorf("mintBalance: %w", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	if acc.Balance+amount <= acc.Balance {
		return ErrBalanceOverflow
	}
	acc.Balance += amount
	log.Debugf("minting %d tokens to account %s", amount, to.String())
	return v.SetAccount(to, acc)
}

// BurnTxCost burns the cost of a transaction
// if cost is set to 0 just return
func (v *State) BurnTxCost(from common.Address, cost uint64) error {
	if cost != 0 {
		return v.TransferBalance(&vochaintx.TransferTokensMeta{
			FromAddress: from,
			ToAddress:   BurnAddress,
			Amount:      cost,
		}, true)
	}
	return nil
}
