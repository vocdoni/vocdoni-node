package arbo

import (
	"bytes"
	"fmt"
	"math"
	"runtime"
	"sync"

	"go.vocdoni.io/dvote/db"
)

// AddBatch adds a batch of key-values to the Tree. Returns an array containing
// the indexes of the keys failed to add. Supports empty values as input
// parameters, which is equivalent to 0 valued byte array.
func (t *Tree) AddBatch(keys, values [][]byte) ([]Invalid, error) {
	wTx := t.db.WriteTx()
	defer wTx.Discard()

	invalids, err := t.AddBatchWithTx(wTx, keys, values)
	if err != nil {
		return invalids, err
	}
	return invalids, wTx.Commit()
}

// AddBatchWithTx does the same than the AddBatch method, but allowing to pass
// the db.WriteTx that is used. The db.WriteTx will not be committed inside
// this method.
func (t *Tree) AddBatchWithTx(wTx db.WriteTx, keys, values [][]byte) ([]Invalid, error) {
	t.Lock()
	defer t.Unlock()

	if !t.editable() {
		return nil, ErrSnapshotNotEditable
	}

	e := []byte{}
	// equal the number of keys & values
	if len(keys) > len(values) {
		// add missing values
		for i := len(values); i < len(keys); i++ {
			values = append(values, e)
		}
	} else if len(keys) < len(values) {
		// crop extra values
		values = values[:len(keys)]
	}

	nLeafs, err := t.GetNLeafsWithTx(wTx)
	if err != nil {
		return nil, err
	}
	if nLeafs > t.thresholdNLeafs {
		return t.addBatchInDisk(wTx, keys, values)
	}
	return t.addBatchInMemory(wTx, keys, values)
}

func (t *Tree) addBatchInDisk(wTx db.WriteTx, keys, values [][]byte) ([]Invalid, error) {
	nCPU := flp2(runtime.NumCPU())
	if nCPU == 1 || len(keys) < nCPU {
		var invalids []Invalid
		for i := 0; i < len(keys); i++ {
			if err := t.addWithTx(wTx, keys[i], values[i]); err != nil {
				invalids = append(invalids, Invalid{i, err})
			}
		}
		return invalids, nil
	}

	kvs, invalids, err := keysValuesToKvs(t.maxLevels, keys, values)
	if err != nil {
		return nil, err
	}

	buckets := splitInBuckets(kvs, nCPU)

	root, err := t.RootWithTx(wTx)
	if err != nil {
		return nil, err
	}

	l := int(math.Log2(float64(nCPU)))
	subRoots, err := t.getSubRootsAtLevel(wTx, root, l+1)
	if err != nil {
		return nil, err
	}
	if len(subRoots) != nCPU {
		// Already populated Tree but Unbalanced.

		// add one key at each bucket, and then continue with the flow
		for i := 0; i < len(buckets); i++ {
			// add one leaf of the bucket, if there is an error when
			// adding the k-v, try to add the next one of the bucket
			// (until one is added)
			inserted := -1
			for j := 0; j < len(buckets[i]); j++ {
				if newRoot, err := t.add(wTx, root, 0,
					buckets[i][j].k, buckets[i][j].v); err == nil {
					inserted = j
					root = newRoot
					break
				}
			}

			// remove the inserted element from buckets[i]
			if inserted != -1 {
				buckets[i] = append(buckets[i][:inserted], buckets[i][inserted+1:]...)
			}
		}
		subRoots, err = t.getSubRootsAtLevel(wTx, root, l+1)
		if err != nil {
			return nil, err
		}
	}

	if len(subRoots) != nCPU {
		return nil, fmt.Errorf("this error should not be reached."+
			" len(subRoots) != nCPU, len(subRoots)=%d, nCPU=%d."+
			" Please report it in a new issue:"+
			" https://go.vocdoni.io/dvote/tree/arbo/issues/new", len(subRoots), nCPU)
	}

	invalidsInBucket := make([][]Invalid, nCPU)
	txs := make([]db.WriteTx, nCPU)
	for i := 0; i < nCPU; i++ {
		txs[i] = t.db.WriteTx()
		err := txs[i].Apply(wTx)
		if err != nil {
			return nil, err
		}
	}

	var wg sync.WaitGroup
	wg.Add(nCPU)
	for i := 0; i < nCPU; i++ {
		go func(cpu int) {
			// use different wTx for each cpu, after once all
			// are done, iter over the cpuWTxs and copy their
			// content into the main wTx
			for j := 0; j < len(buckets[cpu]); j++ {
				newSubRoot, err := t.add(txs[cpu], subRoots[cpu],
					l, buckets[cpu][j].k, buckets[cpu][j].v)
				if err != nil {
					invalidsInBucket[cpu] = append(invalidsInBucket[cpu],
						Invalid{buckets[cpu][j].pos, err})
					continue
				}
				// if there has not been errors, set the new subRoots[cpu]
				subRoots[cpu] = newSubRoot
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	for i := 0; i < nCPU; i++ {
		if err := wTx.Apply(txs[i]); err != nil {
			return nil, err
		}
		txs[i].Discard()
	}

	for i := 0; i < len(invalidsInBucket); i++ {
		invalids = append(invalids, invalidsInBucket[i]...)
	}

	newRoot, err := t.upFromSubRoots(wTx, subRoots)
	if err != nil {
		return nil, err
	}

	// update dbKeyNLeafs
	if err := t.SetRootWithTx(wTx, newRoot); err != nil {
		return nil, err
	}

	// update nLeafs
	if err := t.incNLeafs(wTx, len(keys)-len(invalids)); err != nil {
		return nil, err
	}

	return invalids, nil
}

func (t *Tree) upFromSubRoots(wTx db.WriteTx, subRoots [][]byte) ([]byte, error) {
	// is a method of Tree just to get access to t.hashFunction and
	// t.emptyHash.

	// go up from subRoots to up, storing nodes in the given WriteTx
	// once up at the root, store it in the WriteTx using the dbKeyRoot
	if len(subRoots) == 1 {
		return subRoots[0], nil
	}
	// get the subRoots values to know the node types of each subRoot
	nodeTypes := make([]byte, len(subRoots))
	for i := 0; i < len(subRoots); i++ {
		if bytes.Equal(subRoots[i], t.emptyHash) {
			nodeTypes[i] = PrefixValueEmpty
			continue
		}
		v, err := wTx.Get(subRoots[i])
		if err != nil {
			return nil, err
		}
		nodeTypes[i] = v[0]
	}

	var newSubRoots [][]byte
	for i := 0; i < len(subRoots); i += 2 {
		if (bytes.Equal(subRoots[i], t.emptyHash) && bytes.Equal(subRoots[i+1], t.emptyHash)) ||
			(nodeTypes[i] == PrefixValueLeaf && bytes.Equal(subRoots[i+1], t.emptyHash)) {
			// when both sub nodes are empty, the parent is also empty
			// or
			// when 1st sub node is a leaf but the 2nd is empty, the
			// leaf is used as 'parent'

			newSubRoots = append(newSubRoots, subRoots[i])
			continue
		}
		if bytes.Equal(subRoots[i], t.emptyHash) && nodeTypes[i+1] == PrefixValueLeaf {
			// when 2nd sub node is a leaf but the 1st is empty,
			// the leaf is used as 'parent'
			newSubRoots = append(newSubRoots, subRoots[i+1])
			continue
		}

		k, v, err := t.newIntermediate(subRoots[i], subRoots[i+1])
		if err != nil {
			return nil, err
		}
		// store k-v to db
		if err = wTx.Set(k, v); err != nil {
			return nil, err
		}
		newSubRoots = append(newSubRoots, k)
	}

	return t.upFromSubRoots(wTx, newSubRoots)
}

func (t *Tree) getSubRootsAtLevel(rTx db.Reader, root []byte, l int) ([][]byte, error) {
	// go at level l and return each node key, where each node key is the
	// subRoot of the subTree that starts there

	var subRoots [][]byte
	err := t.iterWithStop(rTx, root, 0, func(currLvl int, k, v []byte) bool {
		if currLvl == l && !bytes.Equal(k, t.emptyHash) {
			subRoots = append(subRoots, k)
		}
		if currLvl >= l {
			return true // to stop the iter from going down
		}
		return false
	})

	return subRoots, err
}

func (t *Tree) addBatchInMemory(wTx db.WriteTx, keys, values [][]byte) ([]Invalid, error) {
	vt, err := t.loadVT(wTx)
	if err != nil {
		return nil, err
	}

	invalids, err := vt.addBatch(keys, values)
	if err != nil {
		return nil, err
	}

	// once the VirtualTree is build, compute the hashes
	pairs, err := vt.computeHashes()
	if err != nil {
		// currently invalids in computeHashes are not counted,
		// but should not be needed, as if there is an error there is
		// nothing stored in the db and the error is returned
		return nil, err
	}

	// store pairs in db
	for i := 0; i < len(pairs); i++ {
		if err := wTx.Set(pairs[i][0], pairs[i][1]); err != nil {
			return nil, err
		}
	}

	// store root (from the vt) to db
	if vt.root != nil {
		if err := wTx.Set(dbKeyRoot, vt.root.h); err != nil {
			return nil, err
		}
	}

	// update nLeafs
	if err := t.incNLeafs(wTx, len(keys)-len(invalids)); err != nil {
		return nil, err
	}

	return invalids, nil
}

// loadVT loads a new virtual tree (vt) from the current Tree, which contains
// the same leafs.
func (t *Tree) loadVT(rTx db.Reader) (vt, error) {
	vt := newVT(t.maxLevels, t.hashFunction)
	vt.params.dbg = t.dbg
	var callbackErr error
	err := t.IterateWithStopWithTx(rTx, nil, func(_ int, k, v []byte) bool {
		if v[0] != PrefixValueLeaf {
			return false
		}
		leafK, leafV := ReadLeafValue(v)
		if err := vt.add(0, leafK, leafV); err != nil {
			callbackErr = err
			return true
		}
		return false
	})
	if callbackErr != nil {
		return vt, callbackErr
	}

	return vt, err
}
