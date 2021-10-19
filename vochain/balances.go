package vochain

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/arbo"
)

// Balance represents an amount of tokens, usually attached to an address.
// Balance includes a Nonce which needs to be incremented by 1 on each transfer.
type Balance struct {
	Amount uint64
	Nonce  uint32
}

// Marshal encodes the Balance and returns the serialized bytes.
func (b *Balance) Marshal() []byte {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint64(data[:8], b.Amount)
	binary.LittleEndian.PutUint32(data[8:], b.Nonce)
	return data
}

// Unmarshal decode a set of bytes.
func (b *Balance) Unmarshal(data []byte) error {
	if len(data) != 12 {
		return fmt.Errorf("wrong balance data lenght: %d", len(data))
	}
	b.Amount = binary.LittleEndian.Uint64(data[:8])
	b.Nonce = binary.LittleEndian.Uint32(data[8:])
	return nil
}

// Transfer moves amount from the origin Balance to the dest Balance.
func (b *Balance) Transfer(dest *Balance, amount uint64, nonce uint32) error {
      if amount == 0 {
              return ErrBalanceAmountZero
      }
	if dest == nil {
		return fmt.Errorf("destination balance wallet nil")
	}
	if b.Nonce+1 != nonce {
		return ErrBalanceNonceInvalid
	}
	if b.Amount < amount {
		return ErrNotEnoughBalance
	}
	dest.Amount += amount
	b.Amount -= amount
	b.Nonce++
	return nil
}

// TransferBalance transfers balance from origin address to destination address,
// and updates the state with the new values (including nonce).
// If origin address balance is not enough, ErrNotEnoughBalance is returned.
// If provided nonce does not match origin address nonce+1, ErrBalanceNonceInvalid is returned.
// If isQuery is set to true, this method will only check against the current state (no changes will be stored)
func (v *State) TransferBalance(from, to common.Address, amount uint64, nonce uint32, isQuery bool) error {
	var balanceFrom, balanceTo Balance
	if !isQuery {
		v.Tx.Lock()
		defer v.Tx.Unlock()
	}
	balanceFromRaw, err := v.Tx.DeepGet(from.Bytes(), BalancesCfg)
	if err != nil {
		return err
	}
	if err := balanceFrom.Unmarshal(balanceFromRaw); err != nil {
		return err
	}
	balanceToRaw, err := v.Tx.DeepGet(to.Bytes(), BalancesCfg)
	if err != nil && !errors.Is(err, arbo.ErrKeyNotFound) {
		return err
	} else if err == nil {
		if err := balanceTo.Unmarshal(balanceToRaw); err != nil {
			return err
		}
	}
	if err := balanceFrom.Transfer(&balanceTo, amount, nonce); err != nil {
		return err
	}
	if !isQuery {
		if err := v.Tx.DeepSet(from.Bytes(), balanceFrom.Marshal(), BalancesCfg); err != nil {
			return err
		}
		if err := v.Tx.DeepSet(to.Bytes(), balanceTo.Marshal(), BalancesCfg); err != nil {
			return err
		}
	}
	return nil
}

// MintBalance increments the existing balance of address by amount
func (v *State) MintBalance(address common.Address, amount uint64) error {
    if amount == 0 {
        return ErrBalanceAmountZero
    }
	var balance Balance
	v.Tx.Lock()
	defer v.Tx.Unlock()
	raw, err := v.Tx.DeepGet(address.Bytes(), BalancesCfg)
	if err != nil && !errors.Is(err, arbo.ErrKeyNotFound) {
		return err
	} else if err == nil {
		if err := balance.Unmarshal(raw); err != nil {
			return err
		}
	}
	if balance.Amount+amount <= balance.Amount {
		return ErrBalanceOverflow
	}
	balance.Amount += amount
	return v.Tx.DeepSet(address.Bytes(), balance.Marshal(), BalancesCfg)
}

// GetBalance retrives the Balance for an address
func (v *State) GetBalance(address common.Address, isQuery bool) (*Balance, error) {
	var balance Balance
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	raw, err := v.mainTreeViewer(isQuery).DeepGet(address.Bytes(), BalancesCfg)
	if err != nil {
		return nil, err
	}
	return &balance, balance.Unmarshal(raw)
}
