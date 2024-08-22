package state

import (
	"fmt"
	"sync"

	"go.vocdoni.io/dvote/censustree"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// StateDB Tree hierarchy
// - Main
//   - Extra (key: string, value: []byte)
//   - Validators (key: address, value: models.Validator)
//   - Accounts (key: address, value: models.Account)
//   - SIK: (key: address, value: []byte)
//   - Processes (key: ProcessId, value: models.StateDBProcess)
//     - CensusPoseidon (key: sequential index 64 bits little endian, value: zkCensusKey)
//     - Votes (key: VoteId, value: models.StateDBVote)
//   - FaucetNonce (key: hash(address + identifier), value: nil)

const (
	TreeMain       = "Main"
	TreeProcess    = "Processes"
	TreeExtra      = "Extra"
	TreeValidators = "Validators"
	TreeAccounts   = "Accounts"
	TreeFaucet     = "FaucetNonce"
	TreeSIK        = "CensusSIK"
	ChildTreeVotes = "Votes"
)

var ErrProcessChildLeafRootUnknown = fmt.Errorf("process child leaf root is unknown")

// treeTxWithMutex is a wrapper over TreeTx with a mutex for convenient
// RWLocking.
type treeTxWithMutex struct {
	*statedb.TreeTx
	sync.RWMutex
}

// MainTreeView is a thread-safe function to obtain a pointer to the last
// opened mainTree as a TreeView.
func (v *State) MainTreeView() *statedb.TreeView {
	v.tx.RLock()
	defer v.tx.RUnlock()
	return v.mainTreeViewValue.Load()
}

// setMainTreeView is a thread-safe function to store a pointer to the last
// opened mainTree as TreeView.
func (v *State) setMainTreeView(treeView *statedb.TreeView) {
	v.mainTreeViewValue.Store(treeView)
}

// mainTreeViewer returns the mainTree as a treeViewer.
// When committed is false, the mainTree returned is the not yet committed one
// from the currently open StateDB transaction.
// When committed is true, the mainTree returned is the last committed version.
func (v *State) mainTreeViewer(committed bool) statedb.TreeViewer {
	if committed {
		return v.MainTreeView()
	}
	return v.tx.AsTreeView()
}

var (
	// MainTrees contains the configuration for the singleton state trees
	MainTrees = map[string]statedb.TreeConfig{
		// TreeExtra is the Extra subTree configuration.
		TreeExtra: statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "xtra",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// TreeValidators is the Validators subTree configuration.
		TreeValidators: statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "valids",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// TreeProcess is the Processes subTree configuration.
		TreeProcess: statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "procs",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// TreeAccounts is the Accounts subTree configuration.
		TreeAccounts: statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "balan",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// TreeFaucet is the Accounts used Faucet Nonce subTree configuration
		TreeFaucet: statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "faucet",
			MaxLevels:         256,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),

		// TreeSIK is the Secret Identity Keys subTree configuration
		TreeSIK: statedb.NewTreeSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionPoseidon,
			KindID:            "sik",
			MaxLevels:         censustree.DefaultMaxLevels,
			ParentLeafGetRoot: rootLeafGetRoot,
			ParentLeafSetRoot: rootLeafSetRoot,
		}),
	}

	// ChildTrees contains the configuration for the state trees dependent on a main tree.
	ChildTrees = map[string]*statedb.TreeNonSingletonConfig{
		// ChildTreeVotes is the Votes subTree (found under a Process leaf) configuration.
		ChildTreeVotes: statedb.NewTreeNonSingletonConfig(statedb.TreeParams{
			HashFunc:          arbo.HashFunctionSha256,
			KindID:            "votes",
			MaxLevels:         256,
			ParentLeafGetRoot: processGetVotesRoot,
			ParentLeafSetRoot: processSetVotesRoot,
		}),
	}
)

// StateTreeCfg returns the state merkle tree with name.
func StateTreeCfg(name string) statedb.TreeConfig {
	if name == TreeMain {
		return statedb.MainTreeCfg
	}
	t, ok := MainTrees[name]
	if !ok {
		panic(fmt.Sprintf("state tree %s does not exist", name))
	}
	return t
}

// StateChildTreeCfg returns the state merkle child tree with name.
func StateChildTreeCfg(name string) *statedb.TreeNonSingletonConfig {
	t, ok := ChildTrees[name]
	if !ok {
		panic(fmt.Sprintf("state subtree %s does not exist", name))
	}
	return t
}

// StateParentChildTreeCfg returns the parent and its child tree under the key leaf.
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
