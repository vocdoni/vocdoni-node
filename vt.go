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
)

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

// WIP
// func (t *vt) addBatch(fromLvl int, k, v []byte) error {
//         // parallelize adding leafs in the virtual tree
//         nCPU := flp2(runtime.NumCPU())
//         l := int(math.Log2(float64(nCPU)))
//
//         return nil
// }

func (t *vt) add(fromLvl int, k, v []byte) error {
	leaf := newLeafNode(t.params, k, v)
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
// leafs are in the tree
func (t *vt) computeHashes() ([][2][]byte, error) {
	var pairs [][2][]byte
	var err error
	pairs, err = t.root.computeHashes(t.params, pairs)
	if err != nil {
		return pairs, err
	}
	return pairs, nil
}

func newLeafNode(p *params, k, v []byte) *node {
	keyPath := make([]byte, p.hashFunction.Len())
	copy(keyPath[:], k)
	path := getPath(p.maxLevels, keyPath)
	n := &node{
		k:    k,
		v:    v,
		path: path,
	}
	return n
}

type virtualNodeType int

const (
	vtEmpty = 0 // for convenience uses same value that PrefixValueEmpty
	vtLeaf  = 1 // for convenience uses same value that PrefixValueLeaf
	vtMid   = 2 // for convenience uses same value that PrefixValueIntermediate
)

func (n *node) typ() virtualNodeType {
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
		return fmt.Errorf("max virtual level %d", currLvl)
	}

	if n == nil {
		// n = leaf // TMP!
		return nil
	}

	t := n.typ()
	switch t {
	case vtMid:
		if leaf.path[currLvl] {
			//right
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
			return fmt.Errorf("key already exists. Existing node: %s, trying to add node: %s",
				hex.EncodeToString(n.k), hex.EncodeToString(leaf.k))
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
		panic(fmt.Errorf("EMPTY %v", n)) // TODO TMP
	default:
		return fmt.Errorf("ERR")
	}

	return nil
}

func (n *node) downUntilDivergence(p *params, currLvl int, oldLeaf, newLeaf *node) error {
	if currLvl > p.maxLevels-1 {
		return fmt.Errorf("max virtual level %d", currLvl)
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

// returns an array of key-values to store in the db
func (n *node) computeHashes(p *params, pairs [][2][]byte) ([][2][]byte, error) {
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
			pairs, err = n.l.computeHashes(p, pairs)
			if err != nil {
				return pairs, err
			}
		} else {
			n.l = &node{
				h: p.emptyHash,
			}
		}
		if n.r != nil {
			pairs, err = n.r.computeHashes(p, pairs)
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
	default:
		return nil, fmt.Errorf("ERR TMP") // TODO
	}

	return pairs, nil
}

//nolint:unused
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

//nolint:unused
func (n *node) graphviz(w io.Writer, p *params, nEmpties int) (int, error) {
	nChars := 4 // TODO move to global constant
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

		fmt.Fprintf(w, "\"%p\" -> {\"k:%v\\nv:%v\"}\n", n,
			hex.EncodeToString(n.k[:nChars]),
			hex.EncodeToString(n.v[:nChars]))
		fmt.Fprintf(w, "\"k:%v\\nv:%v\" [style=dashed]\n",
			hex.EncodeToString(n.k[:nChars]),
			hex.EncodeToString(n.v[:nChars]))
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

//nolint:unused
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
