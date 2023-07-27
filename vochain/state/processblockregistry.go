package state

import (
	"encoding/binary"

	"go.vocdoni.io/dvote/statedb"
)

var pbrDBPrefix = []byte("pbr_")

// ProcessBlockRegistry struct allows to keep in track the block number when the
// current on going processes start. This information is important to know the
// on going election with the biggest endBlock, that is used to check if an
// account can change its SIK yet or not. Read more about this mechanism on
// vochain/state/sik.go
type ProcessBlockRegistry struct {
	startBlocksDB statedb.Updater
	dbLock        *treeTxWithMutex
	mainTree      statedb.TreeViewer
}

// SetStartBlock method creates a new record on the key-value database
// associated to the processes subtree for the process ID and the startBlock
// number provided.
func (pbr *ProcessBlockRegistry) SetStartBlock(pid []byte, startBlock uint32) error {
	pbr.dbLock.Lock()
	defer pbr.dbLock.Unlock()
	newStartBlock := make([]byte, 32)
	binary.LittleEndian.PutUint32(newStartBlock, startBlock)
	return pbr.startBlocksDB.Set(toPrefixKey(pbrDBPrefix, pid), newStartBlock)
}

// DeleteStartBlock method deletes the record on the key-value database
// associated to the processes subtree for the process ID provided.
func (pbr *ProcessBlockRegistry) DeleteStartBlock(pid []byte) error {
	pbr.dbLock.Lock()
	defer pbr.dbLock.Unlock()
	return pbr.startBlocksDB.Delete(toPrefixKey(pbrDBPrefix, pid))
}

// MinStartBlock returns the minimun start block of the current on going
// processes from the no-state db associated to the process sub tree.
func (pbr *ProcessBlockRegistry) MinStartBlock(fromBlock uint32, committed bool) (uint32, error) {
	minStartBlock := fromBlock
	if !committed {
		pbr.dbLock.RLock()
		defer pbr.dbLock.RUnlock()
	}
	processTree, err := pbr.mainTree.SubTree(StateTreeCfg(TreeProcess))
	if err != nil {
		return 0, err
	}
	if err := processTree.NoState().Iterate(pbrDBPrefix, func(_, value []byte) bool {
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
func (pbr *ProcessBlockRegistry) MaxEndBlock(fromBlock uint32, committed bool) (uint32, error) {
	maxEndBlock := fromBlock
	if !committed {
		pbr.dbLock.RLock()
		defer pbr.dbLock.RUnlock()
	}
	processTree, err := pbr.mainTree.SubTree(StateTreeCfg(TreeProcess))
	if err != nil {
		return 0, err
	}
	if err := processTree.NoState().Iterate(pbrDBPrefix, func(pid, _ []byte) bool {
		p, err := getProcess(pbr.mainTree, pid)
		if err != nil {
			return false
		}
		if endBlock := p.StartBlock + p.BlockCount; endBlock > maxEndBlock {
			maxEndBlock = endBlock
		}
		return true
	}); err != nil {
		return 0, err
	}
	return maxEndBlock, nil
}
