package vochain

import (
	"fmt"
	"sync"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// StateDB Tree hierarchy
// - Main
//   - Extra (key: string, value: []byte)
//   - Oracles (key: address, value: []byte{1} if exists)
//   - Validators (key: address, value: models.Validator)
//   - Accounts (key: address, value: models.Account)
//   - Processes (key: ProcessId, value: models.StateDBProcess)
//     - CensusPoseidon (key: sequential index 64 bits little endian, value: zkCensusKey)
//     - Nullifiers (key: pre-census user nullifier, value: weight used)
//     - Votes (key: VoteId, value: models.StateDBVote)

const (
	TreeProcess                    = "Processes"
	TreeExtra                      = "Extra"
	TreeOracles                    = "Oracles"
	TreeValidators                 = "Validators"
	TreeAccounts                   = "Accounts"
	TreeFaucet                     = "FaucetNonce"
	ChildTreeCensus                = "Census"
	ChildTreeCensusPoseidon        = "CensusPoseidon"
	ChildTreePreRegisterNullifiers = "PreRegisterNullifiers"
	ChildTreeVotes                 = "Votes"
)

var (
	ErrProcessChildLeafRootUnknown = fmt.Errorf("process child leaf root is unkown")
)

// treeTxWithMutex is a wrapper over TreeTx with a mutex for convenient
// RWLocking.
type treeTxWithMutex struct {
	*statedb.TreeTx
	sync.RWMutex
}

// MainTreeView is a thread-safe function to obtain a pointer to the last
// opened mainTree as a TreeView.
func (v *State) MainTreeView() *statedb.TreeView {
	return v.mainTreeViewValue.Load().(*statedb.TreeView)
}

// setMainTreeView is a thread-safe function to store a pointer to the last
// opened mainTree as TreeView.
func (v *State) setMainTreeView(treeView *statedb.TreeView) {
	v.mainTreeViewValue.Store(treeView)
}

// mainTreeViewer returns the mainTree as a treeViewer.
// When committed is false, the mainTree returned is the not yet commited one
// from the currently open StateDB transaction.
// When committed is true, the mainTree returned is the last commited version.
func (v *State) mainTreeViewer(committed bool) statedb.TreeViewer {
	if committed {
		return v.MainTreeView()
	}
	return v.Tx.AsTreeView()
}

var (
	// MainTrees contains the configuration for the singleton state trees
	MainTrees = map[string]statedb.TreeConfig{
		// Extra is the Extra subTree configuration.
		"Extra": statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "xtra",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// Oracles is the Oracles subTree configuration.
		"Oracles": statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "oracs",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// Validators is the Validators subTree configuration.
		"Validators": statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "valids",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// Processes is the Processes subTree configuration.
		"Processes": statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "procs",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// Accounts is the Accounts subTree configuration.
		"Accounts": statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "balan",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// FaucetNonce is the Accounts used Faucet Nonce subTree configuration
		"FaucetNonce": statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "faucet",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),
	}

	// ChildTrees contains the configuration for the state trees dependent on a main tree.
	ChildTrees = map[string]*statedb.TreeNonSingletonConfig{
		// Census is the Rolling census subTree (found under a Process leaf)
		// configuration for a process that supports non-anonymous voting with
		// rolling census.
		"Census": statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "cen",
			MaxLevels:         256,
			ParentLeafGetRoot: processGetCensusRoot,
			ParentLeafSetRoot: processSetCensusRoot,
		}),

		// CensusPoseidon is the Rolling census subTree (found under a
		// Process leaf) configuration when the process supports anonymous
		// voting with rolling census.  This Census subTree uses the SNARK
		// friendly hash function Poseidon.
		"CensusPoseidon": statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionPoseidon,
			KindID:            "cenPos",
			MaxLevels:         64,
			ParentLeafGetRoot: processGetCensusRoot,
			ParentLeafSetRoot: processSetCensusRoot,
		}),

		// PreRegisterNullifiers is the Nullifiers subTree (found under a
		// Process leaf) configuration when the process supports anonymous
		// voting with rolling census.  This tree contains the pre-census
		// nullifiers that have pre-registered.
		"PreRegisterNullifiers": statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "prNul",
			MaxLevels:         256,
			ParentLeafGetRoot: processGetPreRegisterNullifiersRoot,
			ParentLeafSetRoot: processSetPreRegisterNullifiersRoot,
		}),

		// Votes is the Votes subTree (found under a Process leaf) configuration.
		"Votes": statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "votes",
			MaxLevels:         256,
			ParentLeafGetRoot: processGetVotesRoot,
			ParentLeafSetRoot: processSetVotesRoot,
		}),
	}
)

// StateTree returns the state merkle tree with name.
func StateTreeCfg(name string) statedb.TreeConfig {
	t, ok := MainTrees[name]
	if !ok {
		panic(fmt.Sprintf("state tree %s does not exist", name))
	}
	return t
}

// StateChildTree returns the state merkle child tree with name.
func StateChildTreeCfg(name string) *statedb.TreeNonSingletonConfig {
	t, ok := ChildTrees[name]
	if !ok {
		panic(fmt.Sprintf("state subtree %s does not exist", name))
	}
	return t
}

// StateParentChildTree returns the parent and its child tree under the key leaf.
func StateParentChildTreeCfg(parent, child string, key []byte) (statedb.TreeConfig, statedb.TreeConfig) {
	parentTree := StateTreeCfg(parent)
	childTree := StateChildTreeCfg(child)
	return parentTree, childTree.WithKey(key)
}

// rootLeafGetRoot is the GetRootFn function for a leaf that is the root
// itself.
func rootLeafGetRoot(value []byte) ([]byte, error) {
	if len(value) != defaultHashLen {
		return nil, fmt.Errorf("len(value) = %v != %v: %w",
			len(value),
			defaultHashLen,
			ErrProcessChildLeafRootUnknown)
	}
	return value, nil
}

// rootLeafSetRoot is the SetRootFn function for a leaf that is the root
// itself.
func rootLeafSetRoot(value []byte, root []byte) ([]byte, error) {
	if len(value) != defaultHashLen {
		return nil, fmt.Errorf("len(value) = %v != %v", len(value), defaultHashLen)
	}
	return root, nil
}

// processGetCensusRoot is the GetRootFn function to get the rolling census
// root of a process leaf.
func processGetCensusRoot(value []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	if len(sdbProc.Process.RollingCensusRoot) != defaultHashLen {
		return nil, fmt.Errorf("len(sdbProc.Process.RollingCensusRoot) != %v: %w",
			defaultHashLen,
			ErrProcessChildLeafRootUnknown)
	}
	return sdbProc.Process.RollingCensusRoot, nil
}

// processSetCensusRoot is the SetRootFn function to set the rolling census
// root of a process leaf.
func processSetCensusRoot(value []byte, root []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	sdbProc.Process.RollingCensusRoot = root
	newValue, err := proto.Marshal(&sdbProc)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal StateDBProcess: %w", err)
	}
	return newValue, nil
}

// processGetPreRegisterNullifiersRoot is the GetRootFn function to get the nullifiers
// root of a process leaf.
func processGetPreRegisterNullifiersRoot(value []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	if len(sdbProc.Process.NullifiersRoot) != defaultHashLen {
		return nil, fmt.Errorf("len(sdbProc.Process.NullifiersRoot) != %v: %w",
			defaultHashLen,
			ErrProcessChildLeafRootUnknown,
		)
	}
	return sdbProc.Process.NullifiersRoot, nil
}

// processSetPreRegisterNullifiersRoot is the SetRootFn function to set the nullifiers
// root of a process leaf.
func processSetPreRegisterNullifiersRoot(value []byte, root []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	sdbProc.Process.NullifiersRoot = root
	newValue, err := proto.Marshal(&sdbProc)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal StateDBProcess: %w", err)
	}
	return newValue, nil
}

// processGetVotesRoot is the GetRootFn function to get the votes root of a
// process leaf.
func processGetVotesRoot(value []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	if len(sdbProc.VotesRoot) != defaultHashLen {
		return nil, fmt.Errorf(
			"len(sdbProc.VotesRoot) != %v: %w",
			defaultHashLen,
			ErrProcessChildLeafRootUnknown)
	}
	return sdbProc.VotesRoot, nil
}

// processSetVotesRoot is the SetRootFn function to set the votes root of a
// process leaf.
func processSetVotesRoot(value []byte, root []byte) ([]byte, error) {
	var sdbProc models.StateDBProcess
	if err := proto.Unmarshal(value, &sdbProc); err != nil {
		return nil, fmt.Errorf("cannot unmarshal StateDBProcess: %w", err)
	}
	sdbProc.VotesRoot = root
	newValue, err := proto.Marshal(&sdbProc)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal StateDBProcess: %w", err)
	}
	return newValue, nil
}
