package state

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/tree/arbo"
)

// TreeDescription has the details needed for a tree to be backed up
type TreeDescription struct {
	Name   string
	Parent string
	Key    []byte
	Tree   statedb.TreeViewer
}

// DeepListStateTrees returns a flat list of all trees in statedb
func (v *State) DeepListStateTrees() ([]TreeDescription, error) {
	list := []TreeDescription{}

	// main tree
	list = append(list, TreeDescription{
		Name:   TreeMain,
		Parent: "",
		Key:    nil,
		Tree:   v.mainTreeViewer(true),
	})

	// main subtrees
	for name := range MainTrees {
		tv, err := v.mainTreeViewer(true).SubTree(StateTreeCfg(name))
		if err != nil {
			return nil, err
		}
		list = append(list, TreeDescription{
			Name:   name,
			Parent: "",
			Key:    nil,
			Tree:   tv,
		})
	}

	// dump child trees that depend on process
	pids, err := v.ListProcessIDs(true)
	if err != nil {
		return nil, err
	}
	log.Debugf("found %d processes", len(pids))

	for name := range ChildTrees {
		for _, pid := range pids {
			childTree, err := v.mainTreeViewer(true).DeepSubTree(StateTreeCfg(TreeProcess), StateChildTreeCfg(name).WithKey(pid))
			if err != nil {
				// key might not exist (i.e process does not have votes)
				if !errors.Is(err, arbo.ErrKeyNotFound) &&
					!errors.Is(err, ErrProcessChildLeafRootUnknown) &&
					!errors.Is(err, statedb.ErrEmptyTree) {
					return nil, fmt.Errorf("child tree (%s) cannot be loaded with key %x: %w", name, pid, err)
				}
				continue
			}
			// according to statedb docs, voteTree arbo.Tree (a non-singleton under processTree) uses prefix
			// `s/procs/s/votes{pID}/t/` (we are using pID, the processID as the id of the subTree)
			// so we need to pass pid here, in order to import the subtree WithKey(pid) during restore
			list = append(list, TreeDescription{
				Name:   name,
				Parent: TreeProcess,
				Key:    pid,
				Tree:   childTree,
			})

		}
	}
	return list, nil
}

func (v *State) RestoreStateTree(r io.Reader, h TreeDescription) error {
	if h.Name == TreeMain {
		log.Debug("found main tree, but it will be simply be rebuilt at the end, skipping...")
		_, _ = io.ReadAll(r)
		return nil
	}

	var treeCfg, parent statedb.TreeConfig
	switch {
	case h.Parent != "":
		parent = StateTreeCfg(h.Parent)
		treeCfg = StateChildTreeCfg(h.Name).WithKey(h.Key)
	default:
		treeCfg = StateTreeCfg(h.Name)
	}

	if err := v.store.Import(treeCfg, &parent, r); err != nil {
		log.Errorw(err, "import tree failed:")
		return err
	}

	var st *statedb.TreeUpdate
	var err error
	switch {
	case h.Parent != "":
		st, err = v.tx.TreeUpdate.DeepSubTree(parent, treeCfg)
	default:
		st, err = v.tx.TreeUpdate.SubTree(treeCfg)
	}
	if err != nil {
		return err
	}
	// TODO: v.store.Import() does not mark the tree as dirty,
	// but it should, to indicate that the root needs to be updated and propagated,
	// so we do it here
	st.MarkDirty()

	return nil
}

// DBPair is a key value pair for the no state db.
type DBPair struct {
	Key   []byte
	Value []byte
}

// ExportNoStateDB exports the no state db to a gob encoder and writes it to the given writer.
// The resulting stream of bytes is deterministic.
func (v *State) ExportNoStateDB(w io.Writer) error {
	pairs := []DBPair{}
	// Iterate traverses the db in order, lexicographically by key
	err := v.NoState(true).Iterate(nil, func(key []byte, value []byte) bool {
		pairs = append(pairs, DBPair{Key: bytes.Clone(key), Value: bytes.Clone(value)})
		return true
	})
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(w)
	return enc.Encode(pairs)
}

// ImportNoStateDB imports the no state db from a gob decoder and writes it to the given db updater.
func (v *State) ImportNoStateDB(r io.Reader) error {
	pairs := []DBPair{}
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&pairs); err != nil {
		return err
	}
	ns := v.NoState(true)
	for _, pair := range pairs {
		if err := ns.Set(pair.Key, pair.Value); err != nil {
			return err
		}
	}
	return nil
}

// Commit calls v.tx.Commit(height)
func (v *State) Commit(height uint32) error {
	return v.tx.Commit(height)
}
