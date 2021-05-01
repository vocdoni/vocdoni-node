package arbo

import (
	"bytes"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/iden3/go-merkletree/db"
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


       R                 R
      / \               /  \
     A   *             /    \
        / \           /      \
       B   C         *        *
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
                         ...     ... (nLeafs < minLeafsThreshold)


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
              ...    ... (nLeafs >= minLeafsThreshold)



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

const (
	minLeafsThreshold = 100 // nolint:gomnd // TMP WIP this will be autocalculated
)

// AddBatchOpt is the WIP implementation of the AddBatch method in a more
// optimized approach.
func (t *Tree) AddBatchOpt(keys, values [][]byte) ([]int, error) {
	t.updateAccessTime()
	t.Lock()
	defer t.Unlock()

	// when len(keyvalues) is not a power of 2, cut at the biggest power of
	// 2 under the len(keys), add those 2**n key-values using the AddBatch
	// approach, and then add the remaining key-values using tree.Add.

	kvs, err := t.keysValuesToKvs(keys, values)
	if err != nil {
		return nil, err
	}

	t.tx, err = t.db.NewTx()
	if err != nil {
		return nil, err
	}

	// if nCPU is not a power of two, cut at the highest power of two under
	// nCPU
	nCPU := highestPowerOfTwo(runtime.NumCPU())
	l := int(math.Log2(float64(nCPU)))
	var invalids []int

	// CASE A: if nLeafs==0 (root==0)
	if bytes.Equal(t.root, t.emptyHash) {
		invalids, err = t.caseA(nCPU, kvs)
		if err != nil {
			return nil, err
		}

		if err = t.finalizeAddBatch(); err != nil {
			return nil, err
		}
		return invalids, nil
	}

	// CASE B: if nLeafs<nBuckets
	nLeafs, err := t.GetNLeafs()
	if err != nil {
		return nil, err
	}
	if nLeafs < minLeafsThreshold { // CASE B
		var excedents []kv
		invalids, excedents, err = t.caseB(nCPU, 0, kvs)
		if err != nil {
			return nil, err
		}
		// add the excedents
		for i := 0; i < len(excedents); i++ {
			err = t.add(0, excedents[i].k, excedents[i].v)
			if err != nil {
				invalids = append(invalids, excedents[i].pos)
			}
		}

		if err = t.finalizeAddBatch(); err != nil {
			return nil, err
		}
		return invalids, nil
	}

	keysAtL, err := t.getKeysAtLevel(l + 1)
	if err != nil {
		return nil, err
	}

	// CASE C: if nLeafs>=minLeafsThreshold && (nLeafs/nBuckets) < minLeafsThreshold
	// available parallelization, will need to be a power of 2 (2**n)
	if nLeafs >= minLeafsThreshold &&
		(nLeafs/nCPU) < minLeafsThreshold &&
		len(keysAtL) == nCPU {
		invalids, err = t.caseC(nCPU, l, keysAtL, kvs)
		if err != nil {
			return nil, err
		}

		if err = t.finalizeAddBatch(); err != nil {
			return nil, err
		}
		return invalids, nil
	}

	// CASE E
	if len(keysAtL) != nCPU {
		// CASE E: add one key at each bucket, and then do CASE D
		buckets := splitInBuckets(kvs, nCPU)
		kvs = []kv{}
		for i := 0; i < len(buckets); i++ {
			// add one leaf of the bucket, if there is an error when
			// adding the k-v, try to add the next one of the bucket
			// (until one is added)
			var inserted int
			for j := 0; j < len(buckets[i]); j++ {
				if err := t.add(0, buckets[i][j].k, buckets[i][j].v); err == nil {
					inserted = j
					break
				}
			}

			// put the buckets elements except the inserted one
			kvs = append(kvs, buckets[i][:inserted]...)
			kvs = append(kvs, buckets[i][inserted+1:]...)
		}
		keysAtL, err = t.getKeysAtLevel(l + 1)
		if err != nil {
			return nil, err
		}
	}

	// CASE D
	if len(keysAtL) == nCPU { // enter in CASE D if len(keysAtL)=nCPU, if not, CASE E
		invalidsCaseD, err := t.caseD(nCPU, l, keysAtL, kvs)
		if err != nil {
			return nil, err
		}
		invalids = append(invalids, invalidsCaseD...)

		if err = t.finalizeAddBatch(); err != nil {
			return nil, err
		}
		return invalids, nil
	}

	// TODO update NLeafs from DB

	return nil, fmt.Errorf("UNIMPLEMENTED")
}

func (t *Tree) finalizeAddBatch() error {
	// store root to db
	if err := t.tx.Put(dbKeyRoot, t.root); err != nil {
		return err
	}
	// commit db tx
	if err := t.tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (t *Tree) caseA(nCPU int, kvs []kv) ([]int, error) {
	// if len(kvs) is not a power of 2, cut at the bigger power
	// of two under len(kvs), build the tree with that, and add
	// later the excedents
	kvsP2, kvsNonP2 := cutPowerOfTwo(kvs)
	invalids, err := t.buildTreeBottomUp(nCPU, kvsP2)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(kvsNonP2); i++ {
		if err = t.add(0, kvsNonP2[i].k, kvsNonP2[i].v); err != nil {
			invalids = append(invalids, kvsNonP2[i].pos)
		}
	}
	return invalids, nil
}

func (t *Tree) caseB(nCPU, l int, kvs []kv) ([]int, []kv, error) {
	// get already existing keys
	aKs, aVs, err := t.getLeafs(t.root)
	if err != nil {
		return nil, nil, err
	}
	aKvs, err := t.keysValuesToKvs(aKs, aVs)
	if err != nil {
		return nil, nil, err
	}
	// add already existing key-values to the inputted key-values
	kvs = append(kvs, aKvs...)

	// proceed with CASE A
	sortKvs(kvs)

	// cutPowerOfTwo, the excedent add it as normal Tree.Add
	kvsP2, kvsNonP2 := cutPowerOfTwo(kvs)
	var invalids []int
	if nCPU > 1 {
		invalids, err = t.buildTreeBottomUp(nCPU, kvsP2)
		if err != nil {
			return nil, nil, err
		}
	} else {
		invalids, err = t.buildTreeBottomUpSingleThread(kvsP2)
		if err != nil {
			return nil, nil, err
		}
	}
	// return the excedents which will be added at the full tree at the end
	return invalids, kvsNonP2, nil
}

func (t *Tree) caseC(nCPU, l int, keysAtL [][]byte, kvs []kv) ([]int, error) {
	// 1. go down until level L (L=log2(nBuckets)): keysAtL

	var excedents []kv
	buckets := splitInBuckets(kvs, nCPU)

	// 2. use keys at level L as roots of the subtrees under each one
	excedentsInBucket := make([][]kv, nCPU)
	subRoots := make([][]byte, nCPU)
	txs := make([]db.Tx, nCPU)
	var wg sync.WaitGroup
	wg.Add(nCPU)
	for i := 0; i < nCPU; i++ {
		go func(cpu int) {
			var err error
			txs[cpu], err = t.db.NewTx()
			if err != nil {
				panic(err) // TODO WIP
			}
			bucketTree := Tree{tx: txs[cpu], db: t.db, maxLevels: t.maxLevels,
				hashFunction: t.hashFunction, root: keysAtL[cpu]}

			// 3. do CASE B (with 1 cpu) for each key at level L
			_, bucketExcedents, err := bucketTree.caseB(1, l, buckets[cpu])
			if err != nil {
				panic(err)
				// return nil, err
			}
			excedentsInBucket[cpu] = bucketExcedents
			subRoots[cpu] = bucketTree.root
			wg.Done()
		}(i)
	}
	wg.Wait()

	// merge buckets txs into Tree.tx
	for i := 0; i < len(txs); i++ {
		if err := t.tx.Add(txs[i]); err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(excedentsInBucket); i++ {
		excedents = append(excedents, excedentsInBucket[i]...)
	}

	// 4. go upFromKeys from the new roots of the subtrees
	newRoot, err := t.upFromKeys(subRoots)
	if err != nil {
		return nil, err
	}
	t.root = newRoot

	// add the key-values that have not been used yet
	var invalids []int
	for i := 0; i < len(excedents); i++ {
		// Add until the level L
		if err = t.add(0, excedents[i].k, excedents[i].v); err != nil {
			invalids = append(invalids, excedents[i].pos) // TODO WIP
		}
	}
	return invalids, nil
}

func (t *Tree) caseD(nCPU, l int, keysAtL [][]byte, kvs []kv) ([]int, error) {
	if nCPU == 1 { // CASE D, but with 1 cpu
		var invalids []int
		for i := 0; i < len(kvs); i++ {
			if err := t.add(0, kvs[i].k, kvs[i].v); err != nil {
				invalids = append(invalids, kvs[i].pos)
			}
		}
		return invalids, nil
	}

	buckets := splitInBuckets(kvs, nCPU)

	subRoots := make([][]byte, nCPU)
	invalidsInBucket := make([][]int, nCPU)
	txs := make([]db.Tx, nCPU)

	var wg sync.WaitGroup
	wg.Add(nCPU)
	for i := 0; i < nCPU; i++ {
		go func(cpu int) {
			var err error
			txs[cpu], err = t.db.NewTx()
			if err != nil {
				panic(err) // TODO WIP
			}
			// put already existing tx into txs[cpu], as txs[cpu]
			// needs the pending key-values that are not in tree.db,
			// but are in tree.tx
			if err := txs[cpu].Add(t.tx); err != nil {
				panic(err) // TODO WIP
			}

			bucketTree := Tree{tx: txs[cpu], db: t.db, maxLevels: t.maxLevels - l,
				hashFunction: t.hashFunction, root: keysAtL[cpu]}

			for j := 0; j < len(buckets[cpu]); j++ {
				if err = bucketTree.add(l, buckets[cpu][j].k, buckets[cpu][j].v); err != nil {
					invalidsInBucket[cpu] = append(invalidsInBucket[cpu], buckets[cpu][j].pos)
				}
			}
			subRoots[cpu] = bucketTree.root
			wg.Done()
		}(i)
	}
	wg.Wait()

	// merge buckets txs into Tree.tx
	for i := 0; i < len(txs); i++ {
		if err := t.tx.Add(txs[i]); err != nil {
			return nil, err
		}
	}

	newRoot, err := t.upFromKeys(subRoots)
	if err != nil {
		return nil, err
	}
	t.root = newRoot

	var invalids []int
	for i := 0; i < len(invalidsInBucket); i++ {
		invalids = append(invalids, invalidsInBucket[i]...)
	}

	return invalids, nil
}

func splitInBuckets(kvs []kv, nBuckets int) [][]kv {
	buckets := make([][]kv, nBuckets)
	// 1. classify the keyvalues into buckets
	for i := 0; i < len(kvs); i++ {
		pair := kvs[i]

		// bucketnum := keyToBucket(pair.k, nBuckets)
		bucketnum := keyToBucket(pair.keyPath, nBuckets)
		buckets[bucketnum] = append(buckets[bucketnum], pair)
	}
	return buckets
}

// TODO rename in a more 'real' name (calculate bucket from/for key)
func keyToBucket(k []byte, nBuckets int) int {
	nLevels := int(math.Log2(float64(nBuckets)))
	b := make([]int, nBuckets)
	for i := 0; i < nBuckets; i++ {
		b[i] = i
	}
	r := b
	mid := len(r) / 2 //nolint:gomnd
	for i := 0; i < nLevels; i++ {
		if int(k[i/8]&(1<<(i%8))) != 0 {
			r = r[mid:]
			mid = len(r) / 2 //nolint:gomnd
		} else {
			r = r[:mid]
			mid = len(r) / 2 //nolint:gomnd
		}
	}
	return r[0]
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

// buildTreeBottomUp splits the key-values into n Buckets (where n is the number
// of CPUs), in parallel builds a subtree for each bucket, once all the subtrees
// are built, uses the subtrees roots as keys for a new tree, which as result
// will have the complete Tree build from bottom to up, where until the
// log2(nCPU) level it has been computed in parallel.
func (t *Tree) buildTreeBottomUp(nCPU int, kvs []kv) ([]int, error) {
	buckets := splitInBuckets(kvs, nCPU)

	subRoots := make([][]byte, nCPU)
	invalidsInBucket := make([][]int, nCPU)
	txs := make([]db.Tx, nCPU)

	var wg sync.WaitGroup
	wg.Add(nCPU)
	for i := 0; i < nCPU; i++ {
		go func(cpu int) {
			sortKvs(buckets[cpu])

			var err error
			txs[cpu], err = t.db.NewTx()
			if err != nil {
				panic(err) // TODO
			}
			bucketTree := Tree{tx: txs[cpu], db: t.db, maxLevels: t.maxLevels,
				hashFunction: t.hashFunction, root: t.emptyHash}

			currInvalids, err := bucketTree.buildTreeBottomUpSingleThread(buckets[cpu])
			if err != nil {
				panic(err) // TODO
			}
			invalidsInBucket[cpu] = currInvalids
			subRoots[cpu] = bucketTree.root
			wg.Done()
		}(i)
	}
	wg.Wait()

	// merge buckets txs into Tree.tx
	for i := 0; i < len(txs); i++ {
		if err := t.tx.Add(txs[i]); err != nil {
			return nil, err
		}
	}

	newRoot, err := t.upFromKeys(subRoots)
	if err != nil {
		return nil, err
	}
	t.root = newRoot

	var invalids []int
	for i := 0; i < len(invalidsInBucket); i++ {
		invalids = append(invalids, invalidsInBucket[i]...)
	}
	return invalids, err
}

// buildTreeBottomUpSingleThread builds the tree with the given []kv from bottom
// to the root. keys & values must be sorted by path, and the array ks must be
// length multiple of 2
func (t *Tree) buildTreeBottomUpSingleThread(kvs []kv) ([]int, error) {
	// TODO check that log2(len(leafs)) < t.maxLevels, if not, maxLevels
	// would be reached and should return error

	var invalids []int
	// build the leafs
	leafKeys := make([][]byte, len(kvs))
	for i := 0; i < len(kvs); i++ {
		// TODO handle the case where Key&Value == 0
		leafKey, leafValue, err := newLeafValue(t.hashFunction, kvs[i].k, kvs[i].v)
		if err != nil {
			// return nil, err
			invalids = append(invalids, kvs[i].pos)
		}
		// store leafKey & leafValue to db
		if err := t.tx.Put(leafKey, leafValue); err != nil {
			// return nil, err
			invalids = append(invalids, kvs[i].pos)
		}
		leafKeys[i] = leafKey
	}
	r, err := t.upFromKeys(leafKeys)
	if err != nil {
		return invalids, err
	}
	t.root = r
	return invalids, nil
}

// keys & values must be sorted by path, and the array ks must be length
// multiple of 2
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

func (t *Tree) getLeafs(root []byte) ([][]byte, [][]byte, error) {
	var ks, vs [][]byte
	err := t.iter(root, func(k, v []byte) {
		if v[0] != PrefixValueLeaf {
			return
		}
		leafK, leafV := readLeafValue(v)
		ks = append(ks, leafK)
		vs = append(vs, leafV)
	})
	return ks, vs, err
}

func (t *Tree) getKeysAtLevel(l int) ([][]byte, error) {
	var keys [][]byte
	err := t.iterWithStop(t.root, 0, func(currLvl int, k, v []byte) bool {
		if currLvl == l && !bytes.Equal(k, t.emptyHash) {
			keys = append(keys, k)
		}
		if currLvl >= l {
			return true // to stop the iter from going down
		}
		return false
	})

	return keys, err
}

// cutPowerOfTwo returns []kv of length that is a power of 2, and a second []kv
// with the extra elements that don't fit in a power of 2 length
func cutPowerOfTwo(kvs []kv) ([]kv, []kv) {
	x := len(kvs)
	if (x & (x - 1)) != 0 {
		p2 := highestPowerOfTwo(x)
		return kvs[:p2], kvs[p2:]
	}
	return kvs, nil
}

func highestPowerOfTwo(n int) int {
	res := 0
	for i := n; i >= 1; i-- {
		if (i & (i - 1)) == 0 {
			res = i
			break
		}
	}
	return res
}

// func computeSimpleAddCost(nLeafs int) int {
//         // nLvls 2^nLvls
//         nLvls := int(math.Log2(float64(nLeafs)))
//         return nLvls * int(math.Pow(2, float64(nLvls)))
// }
//
// func computeBottomUpAddCost(nLeafs int) int {
//         // 2^nLvls * 2 - 1
//         nLvls := int(math.Log2(float64(nLeafs)))
//         return (int(math.Pow(2, float64(nLvls))) * 2) - 1
// }
