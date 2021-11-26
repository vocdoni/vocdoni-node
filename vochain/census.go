package vochain

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	models "go.vocdoni.io/proto/build/go/models"
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
	census, err := v.Tx.DeepSubTree(ProcessesCfg, CensusPoseidonCfg.WithKey(pid))
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
	if censusLen >= *process.MaxCensusSize {
		return fmt.Errorf("maxCensusSize already reached")
	}
	// Add key to census
	index := [8]byte{}
	binary.LittleEndian.PutUint64(index[:], censusLen)
	if err := census.Add(index[:], key); err != nil {
		return fmt.Errorf("cannot add (%x) to rolling census: %w", key, err)
	}
	log.Debugf("added key %x with index %d to rolling census", key, censusLen)
	// // Store mapping between key -> key index
	// if err := noState.Set(keyCensusKeyIndex(key), censusLenLE); err != nil {
	// 	return err
	// }
	// Update census size
	if err := statedb.SetUint64(noState, keyCensusLen, censusLen+1); err != nil {
		return err
	}
	return nil
}

func getRollingCensusSize(mainTreeView statedb.TreeViewer, pid []byte) (uint64, error) {
	census, err := mainTreeView.DeepSubTree(ProcessesCfg, CensusPoseidonCfg.WithKey(pid))
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

func (v *State) GetRollingCensusSize(pid []byte, isQuery bool) (uint64, error) {
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	return getRollingCensusSize(v.mainTreeViewer(isQuery), pid)
}

// PurgeRollingCensus removes a rolling census from the permanent store
// If the census does not exist, it does nothing.
func (s *State) PurgeRollingCensus(pid []byte) error {
	return fmt.Errorf("TODO")
}

// GetRollingCensusRoot returns the last rolling census root for a process id
func (v *State) GetRollingCensusRoot(pid []byte, isQuery bool) ([]byte, error) {
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	census, err := v.mainTreeViewer(isQuery).DeepSubTree(ProcessesCfg, CensusPoseidonCfg.WithKey(pid))
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
	census, err := v.MainTreeView().DeepSubTree(ProcessesCfg,
		CensusPoseidonCfg.WithKey(pid))
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
	dumpData, err := census.Dump()
	if err != nil {
		return nil, fmt.Errorf("cannot dump census with pid %x: %w", pid, err)
	}
	censusID := hex.EncodeToString(dumpRoot)
	return &RollingCensus{
		CensusID: censusID,
		DumpData: dumpData,
		DumpRoot: dumpRoot,
		// IndexKeys: indexKeys,
		Type: models.Census_ARBO_POSEIDON,
	}, nil
}

// RegisterKeyTxCheck validates a registerKeyTx transaction against the state
func (v *State) RegisterKeyTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State,
	forCommit bool) error {
	tx := vtx.GetRegisterKey()

	// Sanity checks
	if tx == nil {
		return fmt.Errorf("register key transaction is nil")
	}
	process, err := state.Process(tx.ProcessId, false)
	if err != nil {
		return fmt.Errorf("cannot fetch processId: %w", err)
	}
	if process == nil || process.EnvelopeType == nil || process.Mode == nil {
		return fmt.Errorf("process %x malformed", tx.ProcessId)
	}
	if state.CurrentHeight() >= process.StartBlock {
		return fmt.Errorf("process %x already started", tx.ProcessId)
	}
	if !(process.Mode.PreRegister && process.EnvelopeType.Anonymous) {
		return fmt.Errorf("RegisterKeyTx only supported with " +
			"Mode.PreRegister and EnvelopeType.Anonymous")
	}
	if process.Status != models.ProcessStatus_READY {
		return fmt.Errorf("process %x not in READY state", tx.ProcessId)
	}
	if tx.Proof == nil {
		return fmt.Errorf("proof missing on registerKeyTx")
	}
	if signature == nil {
		return fmt.Errorf("signature missing on voteTx")
	}
	if len(tx.NewKey) != 32 {
		return fmt.Errorf("newKey wrong size")
	}
	// Verify that we are not over maxCensusSize
	censusSize, err := v.GetRollingCensusSize(tx.ProcessId, false)
	if err != nil {
		return err
	}
	if censusSize >= *process.MaxCensusSize {
		return fmt.Errorf("maxCensusSize already reached")
	}

	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	var addr common.Address
	addr, err = ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}

	var valid bool
	var weight *big.Int
	valid, weight, err = VerifyProof(process, tx.Proof,
		process.CensusOrigin,
		process.CensusRoot,
		process.ProcessId,
		pubKey,
		addr,
	)
	if err != nil {
		return fmt.Errorf("proof not valid: %w", err)
	}
	if !valid {
		return fmt.Errorf("proof not valid")
	}

	// Validate that this user is not registering more keys than possible
	// with the users weight.
	usedWeight, err := v.GetPreRegisterAddrUsedWeight(process.ProcessId, addr)
	if err != nil {
		return fmt.Errorf("cannot get nullifeir used weight: %w", err)
	}
	txWeight, ok := new(big.Int).SetString(tx.Weight, 10)
	if !ok {
		return fmt.Errorf("cannot parse tx weight %s", txWeight)
	}
	usedWeight.Add(usedWeight, txWeight)

	// TODO: In order to support tx.Weight != 1 for anonymous voting, we
	// need to add the weight to the leaf in the CensusPoseidon Tree, and
	// also add the weight as a public input in the circuit to verify it anonymously.
	// The following check ensures that weight != 1 is not used, once the above is
	// implemented we can remove it
	if usedWeight.Cmp(bigOne) != 0 {
		return fmt.Errorf("weight != 1 is not yet supported, received %s", tx.Weight)
	}

	if usedWeight.Cmp(weight) > 0 {
		return fmt.Errorf("cannot register more keys: "+
			"usedWeight + RegisterKey.Weight (%v) > proof weight (%v)",
			usedWeight, weight)
	}
	if forCommit {
		v.setPreRegisterAddrUsedWeight(process.ProcessId, addr, usedWeight)
	}

	// TODO: Add cache like in VoteEnvelopeCheck for the registered key so that:
	// A. We can skip the proof verification when commiting
	// B. We can detect invalid key registration (due to no more weight
	//    available) at mempool tx insertion

	return nil
}

func (v *State) setPreRegisterAddrUsedWeight(pid []byte, addr common.Address, weight *big.Int) error {
	return v.Tx.DeepSet(GenerateNullifier(addr, pid), weight.Bytes(), ProcessesCfg, PreRegisterNullifiersCfg.WithKey(pid))
}

// GetPreRegisterAddrUsedWeight returns the weight used by the address for a process ID on pre-register census
func (v *State) GetPreRegisterAddrUsedWeight(pid []byte, addr common.Address) (*big.Int, error) {
	weightBytes, err := v.Tx.DeepGet(GenerateNullifier(addr, pid), ProcessesCfg, PreRegisterNullifiersCfg.WithKey(pid))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return big.NewInt(0), nil
	} else if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(weightBytes), nil
}
