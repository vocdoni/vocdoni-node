# StateDB

## Summary

Package statedb contains the implementation of StateDB, a database backed
structure that holds the state of the blockchain indexed by version (each
version corresponding to the state at each block).  The StateDB holds a dynamic
hierarcy of linked merkle trees starting with the mainTree on top, with the
property that the keys and values of all merkle trees can be cryptographically
represented by a single hash, the StateDB.Hash (which corresponds to the
mainTree.Root).

## Internals

Internally all subTrees of the StateDB use the same database (for views) and
the same transaction (for a block update).  Database prefixes are used to split
the storage of each subTree while avoiding collisions.  The structure of
prefixes is detailed here:
- subTree: `{KindID}{id}/`
	- arbo.Tree: `t/`
	- subTrees: `s/`
		- contains any number of subTree
	- nostate: `n/`
	- metadata: `m/`
		- versions: `v/` (only in mainTree)
			- currentVersion: `current` -> {currentVersion}
			- version to root: `{version}` -> `{root}`

Since the mainTree is unique and doesn't have a parent, the prefixes used in
the mainTree skip the first element of the path (`{KindID}{id}/`).
- Example:
	- mainTree arbo.Tree: `t/`
	- processTree arbo.Tree (a singleton under mainTree): `s/process/t/`
	- censusTree arbo.Tree (a non-singleton under processTree):
	`s/process/s/census{pID}/t/` (we are using pID, the processID as the id
	of the subTree)
	- voteTree arbo.Tree (a non-singleton under processTree):
	`s/process/s/vote{pID}/t/` (we are using pID, the processID as the id
	of the subTree)

Each tree has an associated database that can be accessed via the NoState
method.  These databases are auxiliary key-values that don't belong to the
blockchain state, and thus any value in the NoState databases won't be
reflected in the StateDB.Hash.  One of the usecases for the NoState database is
to store auxiliary mappings used to optimize the capacity usage of merkletrees
used for zkSNARKS, where the number of levels is of critical matter.  For
example, we may want to build a census tree of babyjubjub public keys that will
be used to prove ownership of the public key via a SNARK in order to vote.  If
we build the merkle tree using the public key as path, we will have an
unbalanced tree which requires more levels than strictly necessary.  On the
other hand, if we use a sequential index as path and set the value to the
public key, we achieve maximum balancing reducing the number of tree levels.
But then we can't easily query for the existence of a public key in the tree to
generate a proof, as it requires to know its index.  In such a case, we can
store the mapping of public key to index in the NoState database.

## Usage

By default the StateDB has a single tree that will be the parent of all
subTrees: the mainTree.

The StateDB is updated via transactions.  When starting a new transaction
(`TreeTx`), you will also get a pointer to the mainTree (via a `TreeUpdate`
type).  When commiting a transaction you must specify the version for the
update.

```go
mainTree, err := sdb.BeginTx()
qt.Assert(t, err, qt.IsNotNil)
defer mainTree.Discard()

mainTree.Add([]byte("key"), []byte("value"))

mainTree.Commit(1)
```

The `TreeTx` and all `TreeUpdate`s derived from it are not thread-safe.

You can get a read-only view of a commited version of the StateDB via the
`TreeView` types.  Passing `nil` to `StateDB.TreeView` will return the last
commited version.

```go
mainTree, err := sdb.TreeView(nil)
value, err := mainTree.Get([]byte("key"))
qt.Assert(t, err, qt.IsNotNil)
qt.Assert(t, value, qt.DeepEquals, []byte("value"))
```

Each subTree can contain other subTrees.  Singleton subTrees are those that are
unique, whereas NonSingleton subTrees can have many instances of the same type,
identified by a different key.  To use a subTree you must first define its configuration:

The following example is a Signleton subTree which stores its root as the
entire value of the parent leaf:
```go
func rootLeafGetRoot(value []byte) ([]byte, error) {
	if len(value) != defaultHashLen {
		return nil, fmt.Errorf("len(value) = %v != %v", len(value), defaultHashLen)
	}
	return value, nil
}

func rootLeafSetRoot(value []byte, root []byte) ([]byte, error) {
	if len(value) != defaultHashLen {
		return nil, fmt.Errorf("len(value) = %v != %v", len(value), defaultHashLen)
	}
	return root, nil
}

ProcessesCfg = statedb.NewTreeSingletonConfig(statedb.TreeParams{
    HashFunc:          arbo.HashFunctionSha256,
    KindID:            "procs",
    MaxLevels:         256,
    ParentLeafGetRoot: rootLeafGetRoot,
    ParentLeafSetRoot: rootLeafSetRoot,
})
```

Before accessing a subTree it must be created.  We do this by creating the
parent leaf that will contain its root, and by opening the SubTree once.
```go
mainTree, err := sdb.BeginTx()
qt.Assert(t, err, qt.IsNotNil)
defer mainTree.Discard()

err := update.Add(ProcessesCfg.Key(), make([]byte, ProcessesCfg.HashFunc().Len()))
qt.Assert(t, err, qt.IsNotNil)

_, err := update.SubTree(ProcessesCfg)
qt.Assert(t, err, qt.IsNotNil)

mainTree.Commit(0)
```

The subTrees are accessed directly with the `SubTree` function, available both in `TreeUpdate` and `TreeView`:
```go
mainTree, err := sdb.BeginTx()
qt.Assert(t, err, qt.IsNotNil)
defer mainTree.Discard()

processes, err := mainTree.SubTree(ProcessesCfg)
qt.Assert(t, err, qt.IsNotNil)

err := processes.Add([]byte("processID"), []byte("marshalled process"))
qt.Assert(t, err, qt.IsNotNil)

mainTree.Commit(3)
```

Using a NonSingleton subTree is very simmilar, but requires specifying a
subTree key in the tree configuration:
```go
census, err := processes.SubTree(CensusCfg.WithKey([]byte("processID")))
qt.Assert(t, err, qt.IsNotNil)

err := census.Add([]byte("nullifier"), []byte("censusKey"))
qt.Assert(t, err, qt.IsNotNil)
```

Apart from leafs identified by a path (key) and a value, each subTree has a
key-value storage that doesn't change the StateDB cryptographic integrity, the
`NoState`.  This storage should only be used for auxiliary data, like mappings
and indexes.

```go
noState := processes.NoState()
err := noState.Set([]byte("NumProcsAnon"), []byte("42"))
qt.Assert(t, err, qt.IsNotNil)

value, err := noState.Get([]byte("NumProcsAnon"))
qt.Assert(t, err, qt.IsNotNil)
qt.Assert(t, value, qt.DeepEquals, []byte("value"))
```

Both `TreeUpdate` and `TreeView` implement the `TreeViewer` interface, which
contains all the read-only methods.  This allows reusing code for reading from
the StateDB either from a commited state or from the current update.

Usually we will have several levels of subTrees that we need to traverse until
we reach the desired subTree.  For better ergonomics when we don't care about
the parent subTrees, we have the set of `Deep*` functions, which act the same
way as the regular `Get`, `Set`, `SubTree` functions, but allow passing a list
of tree configurations so that the operation is applied on the last one.  For
example, the example above where we added a key in the census tree could have
been done like this:

```go
err := mainTree.DeepAdd([]byte("nullifier"), []byte("censusKey"), 
  ProcessesCfg, CensusCfg.WithKey([]byte("processID")))
qt.Assert(t, err, qt.IsNotNil)
```
