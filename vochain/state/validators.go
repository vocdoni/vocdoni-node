package state

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// AddValidator adds a tendemint validator. If it exists, it will be updated.
func (v *State) AddValidator(validator *models.Validator) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	validatorBytes, err := proto.Marshal(validator)
	if err != nil {
		return err
	}
	return v.tx.DeepSet(validator.Address, validatorBytes, StateTreeCfg(TreeValidators))
}

// RemoveValidator removes a tendermint validator identified by its address
func (v *State) RemoveValidator(address []byte) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	validators, err := v.tx.SubTree(StateTreeCfg(TreeValidators))
	if err != nil {
		return err
	}
	if _, err := validators.Get(address); errors.Is(err, arbo.ErrKeyNotFound) {
		return fmt.Errorf("validator not found: %w", err)
	} else if err != nil {
		return err
	}
	return validators.Set(address, nil)
}

// Validators returns a list of the chain validators
// When committed is false, the operation is executed also on not yet committed
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last committed version.
func (v *State) Validators(committed bool) (map[string]*models.Validator, error) {
	if !committed {
		v.tx.RLock()
		defer v.tx.RUnlock()
	}

	validatorsTree, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeValidators))
	if err != nil {
		return nil, err
	}

	validators := make(map[string]*models.Validator)
	var callbackErr error
	if err := validatorsTree.Iterate(func(key, value []byte) bool {
		// removed validators are still in the tree but with value set
		// to nil
		if len(value) == 0 {
			return true
		}
		validator := &models.Validator{}
		if err := proto.Unmarshal(value, validator); err != nil {
			callbackErr = err
			return false
		}
		validators[hex.EncodeToString(validator.GetAddress())] = validator
		return true
	}); err != nil {
		return nil, err
	}
	if callbackErr != nil {
		return nil, callbackErr
	}
	return validators, nil
}

// Validator returns an existing validator identified by the given signing address.
// If the validator is not found, returns nil and no error.
func (v *State) Validator(address common.Address, committed bool) (*models.Validator, error) {
	list, err := v.Validators(committed)
	if err != nil {
		return nil, err
	}
	return list[hex.EncodeToString(address.Bytes())], nil
}
