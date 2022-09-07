package vochain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
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

// Snapshot performs a snapshot of the last commited state for all trees.
func (v *State) Snapshot() error {
	t := v.MainTreeView()
	height, err := v.LastHeight()
	if err != nil {
		return err
	}
	root, err := t.Root()
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(
		v.dataDir,
		storageDirectory,
		snapshotsDirectory,
		fmt.Sprintf("%d_%x", height, root)),
		0750)
	if err != nil {
		return err
	}

	dumpTree := func(name string, tr statedb.TreeViewer) error {
		log.Debugf("dumping subtree %s", name)

		file, err := os.Create(filepath.Join(
			v.dataDir,
			storageDirectory,
			snapshotsDirectory,
			fmt.Sprintf("%d_%x", height, root),
			fmt.Sprintf("%s.arbo", name),
		))
		if err != nil {
			return err
		}
		if err := tr.Dump(file); err != nil {
			return fmt.Errorf("cannot dump tree: %w", err)
		}
		return file.Close()
	}

	// dump main tree
	if err := dumpTree("Main", v.mainTreeViewer(true)); err != nil {
		return err
	}

	// dump main subtrees
	for k := range MainTrees {
		t, err := v.mainTreeViewer(true).SubTree(StateTreeCfg(k))
		if err != nil {
			return err
		}
		if err := dumpTree(k, t); err != nil {
			return err
		}
	}

	// dump child trees that depend on process
	pids, err := v.ListProcessIDs(true)
	if err != nil {
		return err
	}
	log.Debugf("found %d processes", len(pids))
	for name := range ChildTrees {
		for _, p := range pids {
			childTreeCfg := StateChildTreeCfg(name)
			processTree, err := v.mainTreeViewer(true).SubTree(StateTreeCfg(TreeProcess))
			if err != nil {
				return fmt.Errorf("cannot load process tree: %w", err)
			}
			childTree, err := processTree.SubTree(childTreeCfg.WithKey(p))
			if err != nil {
				// key might not exist (i.e process does not have census)
				if !errors.Is(err, arbo.ErrKeyNotFound) &&
					!errors.Is(err, ErrProcessChildLeafRootUnknown) &&
					!errors.Is(err, statedb.ErrEmptyTree) {
					return fmt.Errorf("child tree (%s) cannot be loaded with key %x: %w", name, p, err)
				}
				continue
			}
			if err := dumpTree(fmt.Sprintf("%s.%x", name, p), childTree); err != nil {
				return err
			}
		}
	}

	return nil
}
