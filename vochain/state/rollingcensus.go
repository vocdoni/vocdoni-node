package state

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/proto/build/go/models"
)

// keyCensusLen is the census.NoState key used to store the census size.
var keyCensusLen = []byte("len")

// // pathCensusKeyIndex is the census.NoState path used to store key index
// // indexed by key.
// const pathCensusKeyIndex = "key"
//
// // keyCensusKeyIndex returns the census.Nostate key where key index is stored.
// func keyCensusKeyIndex(key []byte) []byte {
// 	return []byte(path.Join(pathCensusKeyIndex, string(key)))
// }

// AddToRollingCensus adds a new key to an existing rolling census.
// NOTE: weight value is not used.
func (v *State) AddToRollingCensus(pid []byte, key []byte, weight *big.Int) error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	process, err := getProcess(v.mainTreeViewer(false), pid)
	if err != nil {
		return fmt.Errorf("cannot open process with pid %x: %w", pid, err)
	}
	census, err := v.Tx.DeepSubTree(
		StateTreeCfg(TreeProcess),
		StateChildTreeCfg(ChildTreeCensusPoseidon).WithKey(pid),
	)
	if err != nil {
		return fmt.Errorf("cannot open rolling census with pid %x: %w", pid, err)
	}
	// TODO: Replace storage of CensusLen in census.NoState by usage of
	// census.Size once Tree.Size is implemented in Arbo.
	noState := census.NoState()
	censusLen, err := statedb.GetUint64(noState, keyCensusLen)
	if err != nil {
		return fmt.Errorf("cannot get ceneusLen: %w", err)
	}
	if censusLen >= process.MaxCensusSize {
		return fmt.Errorf("maxCensusSize already reached")
	}
	// Add key to census
	index := [8]byte{}
	binary.LittleEndian.PutUint64(index[:], censusLen)
	if err := census.Add(index[:], key); err != nil {
		return fmt.Errorf("cannot add (%x) to rolling census: %w", key, err)
	}
	log.Debugf("added key %x with index %d to rolling census", key, censusLen)
	// Update census size
	if err := statedb.SetUint64(noState, keyCensusLen, censusLen+1); err != nil {
		return err
	}
	return nil
}

func getRollingCensusSize(mainTreeView statedb.TreeViewer, pid []byte) (uint64, error) {
	census, err := mainTreeView.DeepSubTree(
		StateTreeCfg(TreeProcess),
		StateChildTreeCfg(ChildTreeCensusPoseidon).WithKey(pid),
	)
	if err != nil {
		return 0, fmt.Errorf("cannot open rolling census with pid %x: %w", pid, err)
	}
	noState := census.NoState()
	censusLen, err := statedb.GetUint64(noState, keyCensusLen)
	if err != nil {
		return 0, fmt.Errorf("cannot get ceneusLen: %w", err)
	}
	return censusLen, nil
}

func (v *State) GetRollingCensusSize(pid []byte, committed bool) (uint64, error) {
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	return getRollingCensusSize(v.mainTreeViewer(committed), pid)
}

// PurgeRollingCensus removes a rolling census from the permanent store
// If the census does not exist, it does nothing.
func (s *State) PurgeRollingCensus(pid []byte) error {
	return fmt.Errorf("TODO")
}

// GetRollingCensusRoot returns the last rolling census root for a process id
func (v *State) GetRollingCensusRoot(pid []byte, committed bool) ([]byte, error) {
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	census, err := v.mainTreeViewer(committed).DeepSubTree(
		StateParentChildTreeCfg(TreeProcess, ChildTreeCensusPoseidon, pid))
	if err != nil {
		return nil, fmt.Errorf("cannot open rolling census with pid %x: %w", pid, err)
	}
	return census.Root()
}

type RollingCensus struct {
	CensusID string
	DumpData []byte
	DumpRoot []byte
	// IndexKeys [][]byte
	Type models.Census_Type
}

func (v *State) DumpRollingCensus(pid []byte) (*RollingCensus, error) {
	census, err := v.MainTreeView().DeepSubTree(
		StateParentChildTreeCfg(TreeProcess, ChildTreeCensusPoseidon, pid))
	if err != nil {
		return nil, fmt.Errorf("cannot access rolling census with pid %x: %w", pid, err)
	}
	// noState := census.NoState()
	// censusLenLE, err := noState.Get(keyCensusLen)
	// if err != nil {
	// 	return nil, fmt.Errorf("cannot get censusLen for census with pid %x: %w", pid, err)
	// }
	// censusLen := binary.LittleEndian.Uint64(censusLenLE)
	// indexKeys := make([][]byte, censusLen)
	// census.Iterate(func(indexLE, key []byte) bool {
	// 	indexKeys[binary.LittleEndian.Uint64(indexLE)] = key
	// 	return false
	// })
	dumpRoot, err := census.Root()
	if err != nil {
		return nil, fmt.Errorf("cannot get census with pid %x root: %w", pid, err)
	}

	var buf bytes.Buffer
	if err := census.Dump(&buf); err != nil {
		return nil, fmt.Errorf("cannot dump census with pid %x: %w", pid, err)
	}

	censusID := hex.EncodeToString(dumpRoot)
	return &RollingCensus{
		CensusID: censusID,
		DumpData: buf.Bytes(),
		DumpRoot: dumpRoot,
		// IndexKeys: indexKeys,
		Type: models.Census_ARBO_POSEIDON,
	}, nil
}

// SetPreRegisterAddrUsedWeight sets the used weight for a pre-register address.
func (v *State) SetPreRegisterAddrUsedWeight(pid []byte, addr common.Address, weight *big.Int) error {
	return v.Tx.DeepSet(
		GenerateNullifier(addr, pid),
		weight.Bytes(),
		StateTreeCfg(TreeProcess),
		StateChildTreeCfg(ChildTreePreRegisterNullifiers).WithKey(pid),
	)
}

// GetPreRegisterAddrUsedWeight returns the weight used by the address for a process ID on pre-register census
func (v *State) GetPreRegisterAddrUsedWeight(pid []byte, addr common.Address) (*big.Int, error) {
	weightBytes, err := v.Tx.DeepGet(
		GenerateNullifier(addr, pid),
		StateTreeCfg(TreeProcess),
		StateChildTreeCfg(ChildTreePreRegisterNullifiers).WithKey(pid))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return big.NewInt(0), nil
	} else if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(weightBytes), nil
}
