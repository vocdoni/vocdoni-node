package vochain

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/arbo"
)

// Account represents an amount of tokens, usually attached to an address.
// Account includes a Nonce which needs to be incremented by 1 on each transfer.
type Account struct {
	Balance uint64
	Nonce   uint32
}

// Marshal encodes the Account and returns the serialized bytes.
func (a *Account) Marshal() []byte {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint64(data[:8], a.Balance)
	binary.LittleEndian.PutUint32(data[8:], a.Nonce)
	return data
}

// Unmarshal decode a set of bytes.
func (a *Account) Unmarshal(data []byte) error {
	if len(data) != 12 {
		return fmt.Errorf("wrong acc data lenght: %d", len(data))
	}
	a.Balance = binary.LittleEndian.Uint64(data[:8])
	a.Nonce = binary.LittleEndian.Uint32(data[8:])
	return nil
}

// Transfer moves amount from the origin Account to the dest Account.
func (a *Account) Transfer(dest *Account, amount uint64, nonce uint32) error {
	if amount == 0 {
		return fmt.Errorf("cannot transfer zero amount")
	}
	if dest == nil {
		return fmt.Errorf("destination account nil")
	}
	if a.Nonce != nonce {
		return ErrAccountNonceInvalid
	}
	if a.Balance < amount {
		return ErrNotEnoughBalance
	}
	if dest.Balance+amount <= dest.Balance {
		return ErrBalanceOverflow
	}
	dest.Balance += amount
	a.Balance -= amount
	a.Nonce++
	return nil
}

// TransferBalance transfers balance from origin address to destination address,
// and updates the state with the new values (including nonce).
// If origin address acc is not enough, ErrNotEnoughBalance is returned.
// If provided nonce does not match origin address nonce+1, ErrAccountNonceInvalid is returned.
// If isQuery is set to true, this method will only check against the current state (no changes will be stored)
func (v *State) TransferBalance(from, to common.Address, amount uint64, nonce uint32, isQuery bool) error {
	var accFrom, accTo Account
	if !isQuery {
		v.Tx.Lock()
		defer v.Tx.Unlock()
	}
	accFromRaw, err := v.Tx.DeepGet(from.Bytes(), AccountsCfg)
	if err != nil {
		return err
	}
	if err := accFrom.Unmarshal(accFromRaw); err != nil {
		return err
	}
	accToRaw, err := v.Tx.DeepGet(to.Bytes(), AccountsCfg)
	if err != nil && !errors.Is(err, arbo.ErrKeyNotFound) {
		return err
	} else if err == nil {
		if err := accTo.Unmarshal(accToRaw); err != nil {
			return err
		}
	}
	if err := accFrom.Transfer(&accTo, amount, nonce); err != nil {
		return err
	}
	if !isQuery {
		if err := v.Tx.DeepSet(from.Bytes(), accFrom.Marshal(), AccountsCfg); err != nil {
			return err
		}
		if err := v.Tx.DeepSet(to.Bytes(), accTo.Marshal(), AccountsCfg); err != nil {
			return err
		}
	}
	return nil
}

// MintBalance increments the existing acc of address by amount
func (v *State) MintBalance(address common.Address, amount uint64) error {
	if amount == 0 {
		return fmt.Errorf("cannot mint a zero amount balance")
	}
	var acc Account
	v.Tx.Lock()
	defer v.Tx.Unlock()
	raw, err := v.Tx.DeepGet(address.Bytes(), AccountsCfg)
	if err != nil && !errors.Is(err, arbo.ErrKeyNotFound) {
		return err
	} else if err == nil {
		if err := acc.Unmarshal(raw); err != nil {
			return err
		}
	}
	if acc.Balance+amount <= acc.Balance {
		return ErrBalanceOverflow
	}
	acc.Balance += amount
	return v.Tx.DeepSet(address.Bytes(), acc.Marshal(), AccountsCfg)
}

// GetAccount retrives the Account for an address
func (v *State) GetAccount(address common.Address, isQuery bool) (*Account, error) {
	var acc Account
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	raw, err := v.mainTreeViewer(isQuery).DeepGet(address.Bytes(), AccountsCfg)
	if err != nil {
		return nil, err
	}
	return &acc, acc.Unmarshal(raw)
}
