package state

import (
	"encoding/binary"

	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type ProcessBlockRegistry struct {
	treeRW   statedb.Updater
	treeRead statedb.TreeViewer
	treeLock *treeTxWithMutex
}

// RegisterStartBlock method creates a new record on the key-value database
// associated to the processes subtree for the process ID and the startBlock
// number provided.
func (pbr *ProcessBlockRegistry) SetStartBlock(pid []byte, startBlock uint32) error {
	pbr.treeLock.Lock()
	defer pbr.treeLock.Unlock()
	newStartBlock := make([]byte, 32)
	binary.LittleEndian.PutUint32(newStartBlock, startBlock)
	return pbr.treeRW.Set(pid, newStartBlock)
}

// DeleteStartBlock method deletes the record on the key-value database
// associated to the processes subtree for the process ID provided.
func (pbr *ProcessBlockRegistry) DeleteStartBlock(pid []byte) error {
	pbr.treeLock.Lock()
	defer pbr.treeLock.Unlock()
	return pbr.treeRW.Delete(pid)
}

// MinReadyProcessStartBlock returns the minimun start block of the current on going
// processes from the no-state db associated to the process sub tree.
func (pbr *ProcessBlockRegistry) MinStartBlock(minStartBlock uint32, committed bool) (uint32, error) {
	if !committed {
		pbr.treeLock.RLock()
		defer pbr.treeLock.RUnlock()
	}
	if err := pbr.treeRead.NoState().Iterate([]byte{}, func(key, value []byte) bool {
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

// MaxReadyProcessEndBlock returns the maximun end block of the current on going
// processes from the no-state db associated to the process sub tree.
func (pbr *ProcessBlockRegistry) MaxEndBlock(maxEndBlock uint32, committed bool) (uint32, error) {
	if !committed {
		pbr.treeLock.RLock()
		defer pbr.treeLock.RUnlock()
	}
	if err := pbr.treeRead.NoState().Iterate([]byte{}, func(pid, _ []byte) bool {
		processBytes, err := pbr.treeRead.DeepGet(pid)
		if err != nil {
			return false
		}
		var process models.StateDBProcess
		err = proto.Unmarshal(processBytes, &process)
		if err != nil {
			return false
		}
		endBlock := process.GetProcess().StartBlock + process.GetProcess().BlockCount
		if endBlock > maxEndBlock {
			maxEndBlock = endBlock
		}
		return true
	}); err != nil {
		return 0, err
	}
	return maxEndBlock, nil
}
