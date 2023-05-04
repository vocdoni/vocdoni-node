package state

import (
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// AddValidator adds a tendemint validator if it is not already added
func (v *State) AddValidator(validator *models.Validator) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	validatorBytes, err := proto.Marshal(validator)
	if err != nil {
		return err
	}
	return v.Tx.DeepSet(validator.Address, validatorBytes, StateTreeCfg(TreeValidators))
}
