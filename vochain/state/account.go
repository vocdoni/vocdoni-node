package state

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// Account represents an amount of tokens, usually attached to an address.
// Account includes a Nonce which needs to be incremented by 1 on each transfer,
// an external URI link for metadata and a list of delegated addresses allowed
// to use the account on its behalf (in addition to himself).
type Account struct {
	models.Account
}

// Marshal encodes the Account and returns the serialized bytes.
func (a *Account) Marshal() ([]byte, error) {
	return proto.Marshal(a)
}

// Unmarshal decode a set of bytes.
func (a *Account) Unmarshal(data []byte) error {
	return proto.Unmarshal(data, a)
}

// Transfer moves amount from the origin Account to the dest Account.
func (a *Account) Transfer(dest *Account, amount uint64) error {
	if amount == 0 {
		return fmt.Errorf("cannot transfer zero amount")
	}
	if dest == nil {
		return fmt.Errorf("destination account nil")
	}
	if a.Balance < amount {
		return ErrNotEnoughBalance
	}
	if dest.Balance+amount <= dest.Balance {
		return ErrBalanceOverflow
	}
	dest.Balance += amount
	a.Balance -= amount
	return nil
}

// IsDelegate checks if an address is a delegate for an account
func (a *Account) IsDelegate(addr common.Address) bool {
	for _, d := range a.DelegateAddrs {
		if bytes.Equal(addr.Bytes(), d) {
			return true
		}
	}
	return false
}

// AddDelegate adds an address to the list of delegates for an account
func (a *Account) AddDelegate(addr common.Address) error {
	if a.IsDelegate(addr) {
		return fmt.Errorf("address %s is already a delegate", addr.String())
	}
	a.DelegateAddrs = append(a.DelegateAddrs, addr.Bytes())
	return nil
}

// DelDelegate removes an address from the list of delegates for an account
func (a *Account) DelDelegate(addr common.Address) error {
	for i, d := range a.DelegateAddrs {
		if !a.IsDelegate(addr) {
			return fmt.Errorf("address %s is not a delegate", addr.String())
		}
		if bytes.Equal(addr.Bytes(), d) {
			a.DelegateAddrs[i] = a.DelegateAddrs[len(a.DelegateAddrs)-1]
			a.DelegateAddrs = a.DelegateAddrs[:len(a.DelegateAddrs)-1]
		}
	}
	return nil
}

// GetAccount retrieves the Account for an address.
// Returns a nil account and no error if the account does not exist.
// Committed is relative to the state on which the function is executed.
func (v *State) GetAccount(address common.Address, committed bool) (*Account, error) {
	var acc Account
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	raw, err := v.mainTreeViewer(committed).DeepGet(address.Bytes(), StateTreeCfg(TreeAccounts))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &acc, acc.Unmarshal(raw)
}

// AccountFromSignature extracts an address from a signed message and returns an account if exists
func (v *State) AccountFromSignature(message, signature []byte) (*common.Address, *Account, error) {
	pubKey, err := ethereum.PubKeyFromSignature(message, signature)
	if err != nil {
		return &common.Address{}, nil, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	address, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return &common.Address{}, nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	acc, err := v.GetAccount(address, false)
	if err != nil {
		return &common.Address{}, nil, fmt.Errorf("cannot get account: %w", err)
	}
	if acc == nil {
		return &common.Address{}, nil, fmt.Errorf("%w %s", ErrAccountNotExist, address.Hex())
	}
	return &address, acc, nil
}

// SetAccountInfoURI sets a given account infoURI
func (v *State) SetAccountInfoURI(accountAddress common.Address, infoURI string) error {
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return err
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	if acc.InfoURI == infoURI {
		return fmt.Errorf("same infoURI")
	}
	if infoURI == "" || len(infoURI) > types.MaxURLLength {
		return fmt.Errorf("invalid infoURI")
	}
	acc.InfoURI = infoURI
	log.Debugf("setting account %s infoURI %s", accountAddress.String(), infoURI)
	return v.SetAccount(accountAddress, acc)
}

// IncrementAccountProcessIndex increments the process index by one and stores the value
func (v *State) IncrementAccountProcessIndex(accountAddress common.Address) error {
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return err
	}
	if acc == nil {
		// TODO: @jordipainan

		// This is an edge case atm: if the entityID is an EVM contract address
		// and an oracle generated the tx is possible that the account does
		// not exist, given that the entityID used is the contract address.
		// This edge case will be solved with the introduction of
		// the param sourceNetworkContractAddr.
		// For now just ignore this error.

		// return ErrAccountNotExist
		log.Debugf("account %s does not exist, skipping process index increment", accountAddress.String())
		return nil
	}
	// safety check for overflow protection, we allow a maximum of 4M of processes per account
	if acc.ProcessIndex > 1<<22 {
		acc.ProcessIndex = 0
	}
	acc.ProcessIndex++
	log.Debugf("setting account %s process index to %d", accountAddress.String(), acc.ProcessIndex)
	return v.SetAccount(accountAddress, acc)
}

// CreateAccount creates an account
func (v *State) CreateAccount(accountAddress common.Address, infoURI string, delegates [][]byte, initialBalance uint64) error {
	newAccount := &Account{}
	if infoURI != "" && len(infoURI) <= types.MaxURLLength {
		newAccount.InfoURI = infoURI
	}
	if len(delegates) > 0 {
		newAccount.DelegateAddrs = append(newAccount.DelegateAddrs, delegates...)
	}
	newAccount.Balance = initialBalance
	log.Debugf("creating account %s with infoURI %s balance %d and delegates %+v",
		accountAddress.String(),
		newAccount.InfoURI,
		newAccount.Balance,
		printPrettierDelegates(newAccount.DelegateAddrs),
	)
	return v.SetAccount(accountAddress, newAccount)
}

// SetAccount sets the given account data to the state
func (v *State) SetAccount(accountAddress common.Address, account *Account) error {
	accBytes, err := proto.Marshal(account)
	if err != nil {
		return err
	}
	log.Debugf("setAccount: address %s, nonce %d, infoURI %s, balance: %d, delegates: %+v, processIndex: %d",
		accountAddress.String(),
		account.Nonce,
		account.InfoURI,
		account.Balance,
		printPrettierDelegates(account.DelegateAddrs),
		account.ProcessIndex,
	)
	for _, l := range v.eventListeners {
		l.OnSetAccount(accountAddress.Bytes(), &Account{
			models.Account{
				Nonce:         account.Nonce,
				InfoURI:       account.InfoURI,
				Balance:       account.Balance,
				DelegateAddrs: account.DelegateAddrs,
				ProcessIndex:  account.ProcessIndex,
			},
		})
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(accountAddress.Bytes(), accBytes, StateTreeCfg(TreeAccounts))
}

// BurnTxCostIncrementNonce reduces the transaction cost from the account balance and increments nonce
func (v *State) BurnTxCostIncrementNonce(accountAddress common.Address, txType models.TxType) error {
	// get tx cost
	cost, err := v.TxCost(txType, false)
	if err != nil {
		return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
	}
	// get account
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	if cost != 0 {
		// send cost to burn address
		burnAcc, err := v.GetAccount(BurnAddress, false)
		if err != nil {
			return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
		}
		if burnAcc == nil {
			return fmt.Errorf("burnTxCostIncrementNonce: burn account does not exist")
		}
		if err := acc.Transfer(burnAcc, cost); err != nil {
			return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
		}
		log.Debugf("burning fee for tx %s with cost %d from account %s", txType.String(), cost, accountAddress.String())
		if err := v.SetAccount(BurnAddress, burnAcc); err != nil {
			return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
		}
	}
	acc.Nonce++
	if err := v.SetAccount(accountAddress, acc); err != nil {
		return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
	}
	return nil
}

// SetAccountDelegate sets a set of delegates for a given account
func (v *State) SetAccountDelegate(accountAddr common.Address,
	delegateAddrs [][]byte,
	txType models.TxType) error {
	acc, err := v.GetAccount(accountAddr, false)
	if err != nil {
		return err
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	switch txType {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		log.Debugf("adding delegates %+v for account %s", delegateAddrs, accountAddr.String())
		for _, delegate := range delegateAddrs {
			if err := acc.AddDelegate(common.BytesToAddress(delegate)); err != nil {
				return fmt.Errorf("cannot add delegate, AddDelegate: %w", err)
			}
		}
		return v.SetAccount(accountAddr, acc)
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		log.Debugf("deleting delegates %+v for account %s", delegateAddrs, accountAddr.String())
		for _, delegate := range delegateAddrs {
			if err := acc.DelDelegate(common.BytesToAddress(delegate)); err != nil {
				return fmt.Errorf("cannot delete delegate, DelDelegate: %w", err)
			}
		}
		return v.SetAccount(accountAddr, acc)
	default:
		return fmt.Errorf("invalid tx type")
	}
}
