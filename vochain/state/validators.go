package state

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/VictoriaMetrics/metrics"
	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

func labelsFrom(v *models.Validator) string {
	return fmt.Sprintf(`{address="%x",validator_address="%X",name=%q}`,
		v.GetAddress(), v.GetValidatorAddress(), v.GetName())
}

func metricsUpdateValidator(validator *models.Validator) {
	metrics.GetOrCreateCounter("vochain_validator_power" + labelsFrom(validator)).Set(validator.GetPower())
	metrics.GetOrCreateCounter("vochain_validator_proposals" + labelsFrom(validator)).Set(validator.GetProposals())
	metrics.GetOrCreateCounter("vochain_validator_score" + labelsFrom(validator)).Set(uint64(validator.GetScore()))
	metrics.GetOrCreateCounter("vochain_validator_votes" + labelsFrom(validator)).Set(validator.GetVotes())
}

func metricsDeleteValidator(validator *models.Validator) {
	metrics.UnregisterMetric("vochain_validator_power" + labelsFrom(validator))
	metrics.UnregisterMetric("vochain_validator_proposals" + labelsFrom(validator))
	metrics.UnregisterMetric("vochain_validator_score" + labelsFrom(validator))
	metrics.UnregisterMetric("vochain_validator_votes" + labelsFrom(validator))
}

// AddValidator adds a tendemint validator. If it exists, it will be updated.
func (v *State) AddValidator(validator *models.Validator) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	validatorBytes, err := proto.Marshal(validator)
	if err != nil {
		return err
	}
	if err := v.tx.DeepSet(validator.GetAddress(), validatorBytes, StateTreeCfg(TreeValidators)); err != nil {
		return err
	}
	go metricsUpdateValidator(validator)
	return nil
}

// RemoveValidator removes a tendermint validator identified by its validator.Address
func (v *State) RemoveValidator(validator *models.Validator) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	validators, err := v.tx.SubTree(StateTreeCfg(TreeValidators))
	if err != nil {
		return err
	}
	if _, err := validators.Get(validator.GetAddress()); errors.Is(err, arbo.ErrKeyNotFound) {
		return fmt.Errorf("validator not found: %w", err)
	} else if err != nil {
		return err
	}
	if err := validators.Set(validator.GetAddress(), nil); err != nil {
		return err
	}
	go metricsDeleteValidator(validator)
	return nil
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

// validatorInactiveSinceKey namespaces the TreeExtra entry that records the
// block height at which a validator first reached the minimum consensus power
// floor. The prefix is short so the key fits inside TreeExtra's 32-byte cap.
const validatorInactiveSinceKeyPrefix = "vldIS/"

func validatorInactiveSinceKey(addr []byte) []byte {
	k := make([]byte, 0, len(validatorInactiveSinceKeyPrefix)+len(addr))
	k = append(k, validatorInactiveSinceKeyPrefix...)
	return append(k, addr...)
}

// SetValidatorInactiveSince records the block height at which the validator
// with the given signing address first fell to the minimum power floor. The
// IST uses this marker to enforce a grace period before removing the validator
// from the set.
func (v *State) SetValidatorInactiveSince(addr []byte, height uint32) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], height)
	return v.tx.DeepSet(validatorInactiveSinceKey(addr), b[:], StateTreeCfg(TreeExtra))
}

// ValidatorInactiveSince returns the height recorded by SetValidatorInactiveSince
// for the given validator address. The second result is false when no marker
// exists (either because it was never set or because it was cleared).
func (v *State) ValidatorInactiveSince(addr []byte, committed bool) (uint32, bool, error) {
	if !committed {
		v.tx.RLock()
		defer v.tx.RUnlock()
	}
	extra, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeExtra))
	if err != nil {
		return 0, false, err
	}
	value, err := extra.Get(validatorInactiveSinceKey(addr))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	// A cleared marker is written as an empty leaf.
	if len(value) == 0 {
		return 0, false, nil
	}
	if len(value) != 4 {
		return 0, false, fmt.Errorf("validator inactive-since: unexpected value length %d", len(value))
	}
	return binary.BigEndian.Uint32(value), true, nil
}

// ClearValidatorInactiveSince removes any inactive-since marker for the given
// validator address. It is safe to call when no marker is set.
func (v *State) ClearValidatorInactiveSince(addr []byte) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	return v.tx.DeepSet(validatorInactiveSinceKey(addr), nil, StateTreeCfg(TreeExtra))
}
