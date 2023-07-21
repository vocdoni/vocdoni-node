package state

import (
	"encoding/binary"

	"go.vocdoni.io/dvote/statedb"
)

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
	return pbr.startBlocksDB.Set(pid, newStartBlock)
}

// DeleteStartBlock method deletes the record on the key-value database
// associated to the processes subtree for the process ID provided.
func (pbr *ProcessBlockRegistry) DeleteStartBlock(pid []byte) error {
	pbr.dbLock.Lock()
	defer pbr.dbLock.Unlock()
	return pbr.startBlocksDB.Delete(pid)
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
	if err := processTree.NoState().Iterate([]byte{}, func(key, value []byte) bool {
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
	if err := processTree.NoState().Iterate(nil, func(pid, _ []byte) bool {
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
