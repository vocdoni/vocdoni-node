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
	// Metrics runs on a background goroutine; the caller frequently mutates the
	// *models.Validator immediately after this call (e.g. IST Pass 3 setting
	// Power=0 for tombstoning), so hand the goroutine a fresh copy to avoid a
	// data race on the shared pointer.
	go metricsUpdateValidator(proto.Clone(validator).(*models.Validator))
	return nil
}

// RemoveValidator removes a tendermint validator identified by its
// validator.Address. It also clears the per-validator IST markers (vldIS/,
// vldSW/) so any subsequent re-add of the same address starts with a clean
// slate — the two are logically part of the same "this address is no longer
// a validator" operation, and letting a caller forget one and remember the
// other has produced silent bugs before.
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
	// Inline the marker clears rather than delegating to ClearValidator*
	// helpers because those take v.tx.Lock() themselves — sync.Mutex is not
	// reentrant, so the delegated call would deadlock under our defer.
	if err := v.tx.DeepSet(validatorInactiveSinceKey(validator.GetAddress()), nil, StateTreeCfg(TreeExtra)); err != nil {
		return err
	}
	if err := v.tx.DeepSet(validatorScoreWindowKey(validator.GetAddress()), nil, StateTreeCfg(TreeExtra)); err != nil {
		return err
	}
	go metricsDeleteValidator(proto.Clone(validator).(*models.Validator))
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

// validatorScoreWindowKeyPrefix namespaces the TreeExtra entry that records
// the score-window boundary for each validator: the block height at which the
// last score computation ran and the validator's lifetime Votes count at that
// moment. IST derives the next window's score from the delta between the
// current Votes and this recorded value, so the public models.Validator.Height
// and Votes fields can stay as canonical join-height / lifetime-votes values.
const validatorScoreWindowKeyPrefix = "vldSW/"

// validatorScoreWindow is the encoded value stored under vldSW/<address>:
// 4-byte big-endian uint32 window-start height, followed by 8-byte big-endian
// uint64 lifetime Votes count at that height. 12 bytes total.
const validatorScoreWindowValueLen = 12

func validatorScoreWindowKey(addr []byte) []byte {
	k := make([]byte, 0, len(validatorScoreWindowKeyPrefix)+len(addr))
	k = append(k, validatorScoreWindowKeyPrefix...)
	return append(k, addr...)
}

// SetValidatorScoreWindow records the score-window boundary (block height and
// the lifetime Votes count observed at that height) for the given validator.
func (v *State) SetValidatorScoreWindow(addr []byte, height uint32, votes uint64) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	var b [validatorScoreWindowValueLen]byte
	binary.BigEndian.PutUint32(b[0:4], height)
	binary.BigEndian.PutUint64(b[4:12], votes)
	return v.tx.DeepSet(validatorScoreWindowKey(addr), b[:], StateTreeCfg(TreeExtra))
}

// ValidatorScoreWindow returns the score-window boundary recorded by
// SetValidatorScoreWindow. The third result is false when no entry exists
// (the validator has never had a window close yet); in that case the caller
// should fall back to (models.Validator.Height, 0).
func (v *State) ValidatorScoreWindow(addr []byte, committed bool) (uint32, uint64, bool, error) {
	if !committed {
		v.tx.RLock()
		defer v.tx.RUnlock()
	}
	extra, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeExtra))
	if err != nil {
		return 0, 0, false, err
	}
	value, err := extra.Get(validatorScoreWindowKey(addr))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	// A cleared entry is written as an empty leaf.
	if len(value) == 0 {
		return 0, 0, false, nil
	}
	if len(value) != validatorScoreWindowValueLen {
		return 0, 0, false, fmt.Errorf("validator score-window: unexpected value length %d", len(value))
	}
	return binary.BigEndian.Uint32(value[0:4]), binary.BigEndian.Uint64(value[4:12]), true, nil
}

// ClearValidatorScoreWindow removes any score-window entry for the given
// validator address. It is safe to call when no entry is set.
func (v *State) ClearValidatorScoreWindow(addr []byte) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	return v.tx.DeepSet(validatorScoreWindowKey(addr), nil, StateTreeCfg(TreeExtra))
}
