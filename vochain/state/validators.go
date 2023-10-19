package state

import (
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
