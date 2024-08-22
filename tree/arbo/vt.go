// Package arbo > vt.go implements the Virtual Tree, which computes a tree
// without computing any hash. With the idea of once all the leafs are placed in
// their positions, the hashes can be computed, avoiding computing a node hash
// more than one time.
package arbo

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
)

//lint:file-ignore U1000 this code is for debugging

type node struct {
	l    *node
	r    *node
	k    []byte
	v    []byte
	path []bool
	h    []byte
}

type params struct {
	maxLevels    int
	hashFunction HashFunction
	emptyHash    []byte
	dbg          *dbgStats
}

type kv struct {
	pos     int // original position in the inputted array
	keyPath []byte
	k       []byte
	v       []byte
}

func keysValuesToKvs(maxLevels int, ks, vs [][]byte) ([]kv, []Invalid, error) {
	if len(ks) != len(vs) {
		return nil, nil, fmt.Errorf("len(keys)!=len(values) (%d!=%d)",
			len(ks), len(vs))
	}
	var invalids []Invalid
	var kvs []kv
	for i := 0; i < len(ks); i++ {
		keyPath, err := keyPathFromKey(maxLevels, ks[i])
		if err != nil {
			invalids = append(invalids, Invalid{i, err})
			continue
		}
		if err := checkKeyValueLen(ks[i], vs[i]); err != nil {
			invalids = append(invalids, Invalid{i, err})
			continue
		}

		var kvsI kv
		kvsI.pos = i
		kvsI.keyPath = keyPath
		kvsI.k = ks[i]
		kvsI.v = vs[i]
		kvs = append(kvs, kvsI)
	}

	return kvs, invalids, nil
}

// vt stands for virtual tree. It's a tree that does not have any computed hash
// while placing the leafs. Once all the leafs are placed, it computes all the
// hashes. In this way, each node hash is only computed one time (at the end)
// and the tree is computed in memory.
type vt struct {
	root   *node
	params *params
}

func newVT(maxLevels int, hash HashFunction) vt {
	return vt{
		root: nil,
		params: &params{
			maxLevels:    maxLevels,
			hashFunction: hash,
			emptyHash:    make([]byte, hash.Len()), // empty
		},
	}
}

// addBatch adds a batch of key-values to the VirtualTree. Returns an array
// containing the indexes of the keys failed to add. Does not include the
// computation of hashes of the nodes neither the storage of the key-values of
// the tree into the db. After addBatch, vt.computeHashes should be called to
// compute the hashes of all the nodes of the tree.
func (t *vt) addBatch(ks, vs [][]byte) ([]Invalid, error) {
	nCPU := flp2(runtime.NumCPU())
	if nCPU == 1 || len(ks) < nCPU {
		var invalids []Invalid
		for i := 0; i < len(ks); i++ {
			if err := t.add(0, ks[i], vs[i]); err != nil {
				invalids = append(invalids, Invalid{i, err})
			}
		}
		return invalids, nil
	}

	l := int(math.Log2(float64(nCPU)))

	kvs, invalids, err := keysValuesToKvs(t.params.maxLevels, ks, vs)
	if err != nil {
		return invalids, err
	}

	buckets := splitInBuckets(kvs, nCPU)

	nodesAtL, err := t.getNodesAtLevel(l)
	if err != nil {
		return nil, err
	}
	if len(nodesAtL) != nCPU && t.root != nil {
		/*
			Already populated Tree but Unbalanced
			- Need to fill M1 and M2, and then will be able to continue with the flow
				- Search for M1 & M2 in the inputed Keys
				- Add M1 & M2 to the Tree
				- From here can continue with the flow

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
		*/

		// add one key at each bucket, and then continue with the flow
		for i := 0; i < len(buckets); i++ {
			// add one leaf of the bucket, if there is an error when
			// adding the k-v, try to add the next one of the bucket
			// (until one is added)
			inserted := -1
			for j := 0; j < len(buckets[i]); j++ {
				if err := t.add(0, buckets[i][j].k, buckets[i][j].v); err == nil {
					inserted = j
					break
				}
			}

			// remove the inserted element from buckets[i]
			if inserted != -1 {
				buckets[i] = append(buckets[i][:inserted], buckets[i][inserted+1:]...)
			}
		}
		nodesAtL, err = t.getNodesAtLevel(l)
		if err != nil {
			return nil, err
		}
	}
	if len(nodesAtL) != nCPU {
		return nil, fmt.Errorf("this error should not be reached."+
			" len(nodesAtL) != nCPU, len(nodesAtL)=%d, nCPU=%d."+
			" Please report it in a new issue:"+
			" https://go.vocdoni.io/dvote/tree/arbo/issues/new", len(nodesAtL), nCPU)
	}

	subRoots := make([]*node, nCPU)
	invalidsInBucket := make([][]Invalid, nCPU)

	var wg sync.WaitGroup
	wg.Add(nCPU)
	for i := 0; i < nCPU; i++ {
		go func(cpu int) {
			bucketVT := newVT(t.params.maxLevels, t.params.hashFunction)
			bucketVT.root = nodesAtL[cpu]
			for j := 0; j < len(buckets[cpu]); j++ {
				if err := bucketVT.add(l, buckets[cpu][j].k,
					buckets[cpu][j].v); err != nil {
					invalidsInBucket[cpu] = append(invalidsInBucket[cpu],
						Invalid{buckets[cpu][j].pos, err})
				}
			}
			subRoots[cpu] = bucketVT.root
			wg.Done()
		}(i)
	}
	wg.Wait()

	for i := 0; i < len(invalidsInBucket); i++ {
		invalids = append(invalids, invalidsInBucket[i]...)
	}

	newRootNode, err := upFromNodes(subRoots)
	if err != nil {
		return nil, err
	}
	t.root = newRootNode

	return invalids, nil
}

func (t *vt) getNodesAtLevel(l int) ([]*node, error) {
	if t.root == nil {
		var r []*node
		nChilds := int(math.Pow(2, float64(l)))
		for i := 0; i < nChilds; i++ {
			r = append(r, nil)
		}
		return r, nil
	}
	return t.root.getNodesAtLevel(0, l)
}

func (n *node) getNodesAtLevel(currLvl, l int) ([]*node, error) {
	if n == nil {
		var r []*node
		nChilds := int(math.Pow(2, float64(l-currLvl)))
		for i := 0; i < nChilds; i++ {
			r = append(r, nil)
		}
		return r, nil
	}

	typ := n.typ()
	if currLvl == l && typ != vtEmpty {
		return []*node{n}, nil
	}
	if currLvl >= l {
		return nil, fmt.Errorf("this error should not be reached."+
			" currLvl >= l, currLvl=%d, l=%d."+
			" Please report it in a new issue:"+
			" https://go.vocdoni.io/dvote/tree/arbo/issues/new", currLvl, l)
	}

	var nodes []*node

	nodesL, err := n.l.getNodesAtLevel(currLvl+1, l)
	if err != nil {
		return nil, err
	}
	nodes = append(nodes, nodesL...)

	nodesR, err := n.r.getNodesAtLevel(currLvl+1, l)
	if err != nil {
		return nil, err
	}
	nodes = append(nodes, nodesR...)

	return nodes, nil
}

// upFromNodes builds the tree from the bottom to up
func upFromNodes(ns []*node) (*node, error) {
	if len(ns) == 1 {
		return ns[0], nil
	}

	var res []*node
	for i := 0; i < len(ns); i += 2 {
		if (ns[i].typ() == vtEmpty && ns[i+1].typ() == vtEmpty) ||
			(ns[i].typ() == vtLeaf && ns[i+1].typ() == vtEmpty) {
			// when both sub nodes are empty, the parent is also empty
			// or
			// when 1st sub node is a leaf but the 2nd is empty, the
			// leaf is used as 'parent'
			res = append(res, ns[i])
			continue
		}
		if ns[i].typ() == vtEmpty && ns[i+1].typ() == vtLeaf {
			// when 2nd sub node is a leaf but the 1st is empty, the
			// leaf is used as 'parent'
			res = append(res, ns[i+1])
			continue
		}
		n := &node{
			l: ns[i],
			r: ns[i+1],
		}
		res = append(res, n)
	}
	return upFromNodes(res)
}

// add adds a key&value as a leaf in the VirtualTree
func (t *vt) add(fromLvl int, k, v []byte) error {
	leaf, err := newLeafNode(t.params, k, v)
	if err != nil {
		return err
	}
	if t.root == nil {
		t.root = leaf
		return nil
	}

	if err := t.root.add(t.params, fromLvl, leaf); err != nil {
		return err
	}

	return nil
}

// computeHashes should be called after all the vt.add is used, once all the
// leafs are in the tree. Computes the hashes of the tree, parallelizing in the
// available CPUs.
func (t *vt) computeHashes() ([][2][]byte, error) {
	var err error

	nCPU := flp2(runtime.NumCPU())
	l := int(math.Log2(float64(nCPU)))
	nodesAtL, err := t.getNodesAtLevel(l)
	if err != nil {
		return nil, err
	}
	subRoots := make([]*node, nCPU)
	bucketPairs := make([][][2][]byte, nCPU)
	dbgStatsPerBucket := make([]*dbgStats, nCPU)
	errs := make([]error, nCPU)

	var wg sync.WaitGroup
	wg.Add(nCPU)
	for i := 0; i < nCPU; i++ {
		go func(cpu int) {
			bucketVT := newVT(t.params.maxLevels, t.params.hashFunction)
			bucketVT.params.dbg = newDbgStats()
			bucketVT.root = nodesAtL[cpu]
			var err error
			bucketPairs[cpu], err = bucketVT.root.computeHashes(l-1,
				t.params.maxLevels, bucketVT.params, bucketPairs[cpu])
			if err != nil {
				errs[cpu] = err
			}

			subRoots[cpu] = bucketVT.root
			dbgStatsPerBucket[cpu] = bucketVT.params.dbg
			wg.Done()
		}(i)
	}
	wg.Wait()

	for i := 0; i < len(errs); i++ {
		if errs[i] != nil {
			return nil, errs[i]
		}
	}

	for i := 0; i < len(dbgStatsPerBucket); i++ {
		t.params.dbg.add(dbgStatsPerBucket[i])
	}

	var pairs [][2][]byte
	for i := 0; i < len(bucketPairs); i++ {
		pairs = append(pairs, bucketPairs[i]...)
	}

	nodesAtL, err = t.getNodesAtLevel(l)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(nodesAtL); i++ {
		nodesAtL = subRoots
	}

	pairs, err = t.root.computeHashes(0, l, t.params, pairs)
	if err != nil {
		return nil, err
	}

	return pairs, nil
}

func newLeafNode(p *params, k, v []byte) (*node, error) {
	if err := checkKeyValueLen(k, v); err != nil {
		return nil, err
	}

	keyPath, err := keyPathFromKey(p.maxLevels, k)
	if err != nil {
		return nil, err
	}
	path := getPath(p.maxLevels, keyPath)
	n := &node{
		k:    k,
		v:    v,
		path: path,
	}
	return n, nil
}

type virtualNodeType int

const (
	vtEmpty = 0 // for convenience uses same value that PrefixValueEmpty
	vtLeaf  = 1 // for convenience uses same value that PrefixValueLeaf
	vtMid   = 2 // for convenience uses same value that PrefixValueIntermediate
)

func (n *node) typ() virtualNodeType {
	if n == nil {
		return vtEmpty
	}
	if n.l == nil && n.r == nil && n.k != nil {
		return vtLeaf
	}
	if n.l != nil || n.r != nil {
		return vtMid
	}
	return vtEmpty
}

func (n *node) add(p *params, currLvl int, leaf *node) error {
	if currLvl > p.maxLevels-1 {
		return ErrMaxVirtualLevel
	}

	if n == nil {
		// n = leaf // TMP!
		return nil
	}

	t := n.typ()
	switch t {
	case vtMid:
		if leaf.path[currLvl] {
			// right
			if n.r == nil {
				// empty sub-node, add the leaf here
				n.r = leaf
				return nil
			}
			if err := n.r.add(p, currLvl+1, leaf); err != nil {
				return err
			}
		} else {
			if n.l == nil {
				// empty sub-node, add the leaf here
				n.l = leaf
				return nil
			}
			if err := n.l.add(p, currLvl+1, leaf); err != nil {
				return err
			}
		}
	case vtLeaf:
		if bytes.Equal(n.k, leaf.k) {
			return fmt.Errorf("%s. Existing node: %s, trying to add node: %s",
				ErrKeyAlreadyExists, hex.EncodeToString(n.k),
				hex.EncodeToString(leaf.k))
		}

		oldLeaf := &node{
			k:    n.k,
			v:    n.v,
			path: n.path,
		}
		// remove values from current node (converting it to mid node)
		n.k = nil
		n.v = nil
		n.h = nil
		n.path = nil
		if err := n.downUntilDivergence(p, currLvl, oldLeaf, leaf); err != nil {
			return err
		}
	case vtEmpty:
		return fmt.Errorf("virtual tree node.add() with empty node %v", n)
	default:
		return fmt.Errorf("virtual tree node.add() with unknown node type %v", n)
	}

	return nil
}

func (n *node) downUntilDivergence(p *params, currLvl int, oldLeaf, newLeaf *node) error {
	if currLvl > p.maxLevels-1 {
		return ErrMaxVirtualLevel
	}

	if oldLeaf.path[currLvl] != newLeaf.path[currLvl] {
		// reached divergence in next level
		if newLeaf.path[currLvl] {
			n.l = oldLeaf
			n.r = newLeaf
		} else {
			n.l = newLeaf
			n.r = oldLeaf
		}
		return nil
	}
	// no divergence yet, continue going down
	if newLeaf.path[currLvl] {
		// right
		n.r = &node{}
		if err := n.r.downUntilDivergence(p, currLvl+1, oldLeaf, newLeaf); err != nil {
			return err
		}
	} else {
		// left
		n.l = &node{}
		if err := n.l.downUntilDivergence(p, currLvl+1, oldLeaf, newLeaf); err != nil {
			return err
		}
	}

	return nil
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
	mid := len(r) / 2
	for i := 0; i < nLevels; i++ {
		if int(k[i/8]&(1<<(i%8))) != 0 {
			r = r[mid:]
			mid = len(r) / 2
		} else {
			r = r[:mid]
			mid = len(r) / 2
		}
	}
	return r[0]
}

// flp2 computes the floor power of 2, the highest power of 2 under the given
// value.
func flp2(n int) int {
	res := 0
	for i := n; i >= 1; i-- {
		if (i & (i - 1)) == 0 {
			res = i
			break
		}
	}
	return res
}

// computeHashes computes the hashes under the node from which is called the
// method. Returns an array of key-values to store in the db
func (n *node) computeHashes(currLvl, maxLvl int, p *params, pairs [][2][]byte) (
	[][2][]byte, error,
) {
	if n == nil || currLvl >= maxLvl {
		// no need to compute any hash
		return pairs, nil
	}
	if pairs == nil {
		pairs = [][2][]byte{}
	}
	var err error
	t := n.typ()
	switch t {
	case vtLeaf:
		p.dbg.incHash()
		leafKey, leafValue, err := newLeafValue(p.hashFunction, n.k, n.v)
		if err != nil {
			return pairs, err
		}
		n.h = leafKey
		kv := [2][]byte{leafKey, leafValue}
		pairs = append(pairs, kv)
	case vtMid:
		if n.l != nil {
			pairs, err = n.l.computeHashes(currLvl+1, maxLvl, p, pairs)
			if err != nil {
				return pairs, err
			}
		} else {
			n.l = &node{
				h: p.emptyHash,
			}
		}
		if n.r != nil {
			pairs, err = n.r.computeHashes(currLvl+1, maxLvl, p, pairs)
			if err != nil {
				return pairs, err
			}
		} else {
			n.r = &node{
				h: p.emptyHash,
			}
		}
		// once the sub nodes are computed, can compute the current node
		// hash
		p.dbg.incHash()
		k, v, err := newIntermediate(p.hashFunction, n.l.h, n.r.h)
		if err != nil {
			return nil, err
		}
		n.h = k
		kv := [2][]byte{k, v}
		pairs = append(pairs, kv)
	case vtEmpty:
	default:
		return nil, fmt.Errorf("error: n.computeHashes type (%d) no match", t)
	}

	return pairs, nil
}

func (t *vt) graphviz(w io.Writer) error {
	fmt.Fprintf(w, `digraph hierarchy {
node [fontname=Monospace,fontsize=10,shape=box]
`)
	if _, err := t.root.graphviz(w, t.params, 0); err != nil {
		return err
	}
	fmt.Fprintf(w, "}\n")
	return nil
}

func (n *node) graphviz(w io.Writer, p *params, nEmpties int) (int, error) {
	if n == nil {
		return nEmpties, nil
	}

	t := n.typ()
	switch t {
	case vtLeaf:
		leafKey, _, err := newLeafValue(p.hashFunction, n.k, n.v)
		if err != nil {
			return nEmpties, err
		}
		fmt.Fprintf(w, "\"%p\" [style=filled,label=\"%v\"];\n", n, hex.EncodeToString(leafKey[:nChars]))

		k := n.k
		v := n.v
		if len(n.k) >= nChars {
			k = n.k[:nChars]
		}
		if len(n.v) >= nChars {
			v = n.v[:nChars]
		}

		fmt.Fprintf(w, "\"%p\" -> {\"k:%v\\nv:%v\"}\n", n,
			hex.EncodeToString(k),
			hex.EncodeToString(v))
		fmt.Fprintf(w, "\"k:%v\\nv:%v\" [style=dashed]\n",
			hex.EncodeToString(k),
			hex.EncodeToString(v))
	case vtMid:
		fmt.Fprintf(w, "\"%p\" [label=\"\"];\n", n)

		lStr := fmt.Sprintf("%p", n.l)
		rStr := fmt.Sprintf("%p", n.r)
		eStr := ""
		if n.l == nil {
			lStr = fmt.Sprintf("empty%v", nEmpties)
			eStr += fmt.Sprintf("\"%v\" [style=dashed,label=0];\n",
				lStr)
			nEmpties++
		}
		if n.r == nil {
			rStr = fmt.Sprintf("empty%v", nEmpties)
			eStr += fmt.Sprintf("\"%v\" [style=dashed,label=0];\n",
				rStr)
			nEmpties++
		}
		fmt.Fprintf(w, "\"%p\" -> {\"%v\" \"%v\"}\n", n, lStr, rStr)
		fmt.Fprint(w, eStr)
		nEmpties, err := n.l.graphviz(w, p, nEmpties)
		if err != nil {
			return nEmpties, err
		}
		nEmpties, err = n.r.graphviz(w, p, nEmpties)
		if err != nil {
			return nEmpties, err
		}

	case vtEmpty:
	default:
		return nEmpties, fmt.Errorf("ERR")
	}

	return nEmpties, nil
}

func (t *vt) printGraphviz() error {
	w := bytes.NewBufferString("")
	fmt.Fprintf(w,
		"--------\nGraphviz:\n")
	err := t.graphviz(w)
	if err != nil {
		fmt.Println(w)
		return err
	}
	fmt.Fprintf(w,
		"End of Graphviz --------\n")
	fmt.Println(w)
	return nil
}
