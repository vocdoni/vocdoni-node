package arbo

import (
	"bytes"
	"fmt"
	"sort"
)

/*

AddBatch design
===============


CASE A: Empty Tree --> if tree is empty (root==0)
=================================================
- Build the full tree from bottom to top (from all the leaf to the root)


CASE B: ALMOST CASE A, Almost empty Tree --> if Tree has numLeafs < minLeafsThreshold
==============================================================================
- Get the Leafs (key & value) (iterate the tree from the current root getting
the leafs)
- Create a new empty Tree
- Do CASE A for the new Tree, giving the already existing key&values (leafs)
from the original Tree + the new key&values to be added from the AddBatch call

      R
     / \
    A   *
       / \
      B   C


CASE C: ALMOST CASE B --> if Tree has few Leafs (but numLeafs>=minLeafsThreshold)
==============================================================================
- Use A, B, G, F as Roots of subtrees
- Do CASE B for each subtree
- Then go from L to the Root

              R
             /  \
            /    \
           /      \
          *        *
         / |      / \
        /  |     /   \
       /   |    /     \
L:    A    B   G       D
              / \
             /   \
            /     \
           C      *
                 / \
                /   \
               /     \
              D      E



CASE D: Already populated Tree
==============================
- Use A, B, C, D as subtree
- Sort the Keys in Buckets that share the initial part of the path
- For each subtree add there the new leafs

              R
             /  \
            /    \
           /      \
          *        *
         / |      / \
        /  |     /   \
       /   |    /     \
L:    A    B   C       D
     /\   /\  / \     / \
    ...  ... ... ... ... ...


CASE E: Already populated Tree Unbalanced
=========================================
- Need to fill M1 and M2, and then will be able to use CASE D
	- Search for M1 & M2 in the inputed Keys
	- Add M1 & M2 to the Tree
	- From here can use CASE D

              R
             /  \
            /    \
           /      \
          *        *
           |        \
           |         \
           |          \
L:    M1   *   M2      *        (where M1 and M2 are empty)
          / |         /
         /  |        /
        /   |       /
       A    *      *
           / \     | \
          /   \    |  \
         /     \   |   \
        B      *   *   C
              / \  |\
           ... ... | \
                   |  \
                   D  E



Algorithm decision
==================
- if nLeafs==0 (root==0): CASE A
- if nLeafs<minLeafsThreshold: CASE B
- if nLeafs>=minLeafsThreshold && (nLeafs/nBuckets) < minLeafsThreshold: CASE C
- else: CASE D & CASE E


- Multiple tree.Add calls: O(n log n)
	- Used in: cases A, B, C
- Tree from bottom to top: O(log n)
	- Used in: cases D, E

*/

// AddBatchOpt is the WIP implementation of the AddBatch method in a more
// optimized approach.
func (t *Tree) AddBatchOpt(keys, values [][]byte) ([]int, error) {
	t.updateAccessTime()
	t.Lock()
	defer t.Unlock()

	// TODO if len(keys) is not a power of 2, add padding of empty
	// keys&values. Maybe when len(keyvalues) is not a power of 2, cut at
	// the biggest power of 2 under the len(keys), add those 2**n key-values
	// using the AddBatch approach, and then add the remaining key-values
	// using tree.Add.

	kvs, err := t.keysValuesToKvs(keys, values)
	if err != nil {
		return nil, err
	}

	t.tx, err = t.db.NewTx()
	if err != nil {
		return nil, err
	}

	// if nLeafs==0 (root==0): CASE A
	if bytes.Equal(t.root, t.emptyHash) {
		// sort keys & values by path
		sortKvs(kvs)
		return t.buildTreeBottomUp(kvs)
	}

	// if nLeafs<nBuckets: CASE B
	nLeafs, err := t.GetNLeafs()
	if err != nil {
		return nil, err
	}
	minLeafsThreshold := uint64(100) // nolint:gomnd // TMP WIP
	if nLeafs < minLeafsThreshold {
		// get already existing keys
		aKs, aVs, err := t.getLeafs()
		if err != nil {
			return nil, err
		}
		aKvs, err := t.keysValuesToKvs(aKs, aVs)
		if err != nil {
			return nil, err
		}
		// add already existing key-values to the inputted key-values
		kvs = append(kvs, aKvs...)
		// proceed with CASE A
		sortKvs(kvs)
		return t.buildTreeBottomUp(kvs)
	}

	return nil, fmt.Errorf("UNIMPLEMENTED")
}

type kv struct {
	pos     int // original position in the array
	keyPath []byte
	k       []byte
	v       []byte
}

// compareBytes compares byte slices where the bytes are compared from left to
// right and each byte is compared by bit from right to left
func compareBytes(a, b []byte) bool {
	// WIP
	for i := 0; i < len(a); i++ {
		for j := 0; j < 8; j++ {
			aBit := a[i] & (1 << j)
			bBit := b[i] & (1 << j)
			if aBit > bBit {
				return false
			} else if aBit < bBit {
				return true
			}
		}
	}
	return false
}

// sortKvs sorts the kv by path
func sortKvs(kvs []kv) {
	sort.Slice(kvs, func(i, j int) bool {
		return compareBytes(kvs[i].keyPath, kvs[j].keyPath)
	})
}

func (t *Tree) keysValuesToKvs(ks, vs [][]byte) ([]kv, error) {
	if len(ks) != len(vs) {
		return nil, fmt.Errorf("len(keys)!=len(values) (%d!=%d)",
			len(ks), len(vs))
	}
	kvs := make([]kv, len(ks))
	for i := 0; i < len(ks); i++ {
		keyPath := make([]byte, t.hashFunction.Len())
		copy(keyPath[:], ks[i])
		kvs[i].pos = i
		kvs[i].keyPath = ks[i]
		kvs[i].k = ks[i]
		kvs[i].v = vs[i]
	}

	return kvs, nil
}

/*
func (t *Tree) kvsToKeysValues(kvs []kv) ([][]byte, [][]byte) {
	ks := make([][]byte, len(kvs))
	vs := make([][]byte, len(kvs))
	for i := 0; i < len(kvs); i++ {
		ks[i] = kvs[i].k
		vs[i] = kvs[i].v
	}
	return ks, vs
}
*/

// keys & values must be sorted by path, and must be length multiple of 2
// TODO return index of failed keyvaules
func (t *Tree) buildTreeBottomUp(kvs []kv) ([]int, error) {
	// build the leafs
	leafKeys := make([][]byte, len(kvs))
	for i := 0; i < len(kvs); i++ {
		// TODO handle the case where Key&Value == 0
		leafKey, leafValue, err := newLeafValue(t.hashFunction, kvs[i].k, kvs[i].v)
		if err != nil {
			return nil, err
		}
		// store leafKey & leafValue to db
		if err := t.tx.Put(leafKey, leafValue); err != nil {
			return nil, err
		}
		leafKeys[i] = leafKey
	}
	r, err := t.upFromKeys(leafKeys)
	if err != nil {
		return nil, err
	}
	t.root = r
	return nil, nil
}

func (t *Tree) upFromKeys(ks [][]byte) ([]byte, error) {
	if len(ks) == 1 {
		return ks[0], nil
	}

	var rKs [][]byte
	for i := 0; i < len(ks); i += 2 {
		// TODO handle the case where Key&Value == 0
		k, v, err := newIntermediate(t.hashFunction, ks[i], ks[i+1])
		if err != nil {
			return nil, err
		}
		// store k-v to db
		if err = t.tx.Put(k, v); err != nil {
			return nil, err
		}
		rKs = append(rKs, k)
	}
	return t.upFromKeys(rKs)
}

func (t *Tree) getLeafs() ([][]byte, [][]byte, error) {
	var ks, vs [][]byte
	err := t.Iterate(func(k, v []byte) {
		if v[0] != PrefixValueLeaf {
			return
		}
		leafK, leafV := readLeafValue(v)
		ks = append(ks, leafK)
		vs = append(vs, leafV)
	})
	return ks, vs, err
}
