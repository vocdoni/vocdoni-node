package state

import (
	"bytes"
	"encoding/binary"
)

var pbrDBPrefix = []byte("pbr/")

// ProcessBlockRegistry struct allows to keep in track the block number when the
// current on going processes start. This information is important to know the
// on going election with the biggest endBlock, that is used to check if an
// account can change its SIK yet or not. Read more about this mechanism on
// vochain/state/sik.go
type ProcessBlockRegistry struct {
	db    *NoState
	state *State
}

// SetStartBlock method creates a new record on the key-value database
// associated to the processes subtree for the process ID and the startBlock
// number provided.
func (pbr *ProcessBlockRegistry) SetStartBlock(pid []byte, startBlock uint32) error {
	newStartBlock := make([]byte, 32)
	binary.LittleEndian.PutUint32(newStartBlock, startBlock)
	return pbr.db.Set(toPrefixKey(pbrDBPrefix, pid), newStartBlock)
}

// DeleteStartBlock method deletes the record on the key-value database
// associated to the processes subtree for the process ID provided.
func (pbr *ProcessBlockRegistry) DeleteStartBlock(pid []byte) error {
	return pbr.db.Delete(toPrefixKey(pbrDBPrefix, pid))
}

// MinStartBlock returns the minimun start block of the current on going
// processes from the no-state db associated to the process sub tree.
func (pbr *ProcessBlockRegistry) MinStartBlock(fromBlock uint32) (uint32, error) {
	minStartBlock := fromBlock
	if err := pbr.db.Iterate(pbrDBPrefix, func(_, value []byte) bool {
		startBlock := binary.LittleEndian.Uint32(value)
		if startBlock < minStartBlock {
			minStartBlock = startBlock
		}
		return true
	}); err != nil {
		return 0, err
	}
	return minStartBlock, nil
}

// MaxEndBlock returns the maximun end block of the current on going
// processes from the no-state db associated to the process sub tree.
func (pbr *ProcessBlockRegistry) MaxEndBlock(fromBlock uint32) (uint32, error) {
	maxEndBlock := fromBlock
	if err := pbr.db.Iterate(pbrDBPrefix, func(pid, _ []byte) bool {
		p, err := pbr.state.Process(bytes.Clone(pid), false)
		if err != nil {
			return false
		}
		maxEndBlock = max(maxEndBlock, p.StartBlock+p.BlockCount)
		return true
	}); err != nil {
		return 0, err
	}
	return maxEndBlock, nil
}
