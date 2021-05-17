package arbo

import (
	"bytes"
	"fmt"
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
}

// vt stands for virtual tree. It's a tree that does not have any computed hash
// while placing the leafs. Once all the leafs are placed, it computes all the
// hashes. In this way, each node hash is only computed one time.
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

func (t *vt) add(k, v []byte) error {
	leaf := newLeafNode(t.params, k, v)
	if t.root == nil {
		t.root = leaf
		return nil
	}

	if err := t.root.add(t.params, 0, leaf); err != nil {
		return err
	}

	return nil
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
			}
			if err := n.r.add(p, currLvl+1, leaf); err != nil {
				return err
			}
		} else {
			if n.l == nil {
				// empty sub-node, add the leaf here
				n.l = leaf
			}
			if err := n.l.add(p, currLvl+1, leaf); err != nil {
				return err
			}
		}
	case vtLeaf:
		if bytes.Equal(n.k, leaf.k) {
			return fmt.Errorf("key already exists")
		}

		oldLeaf := &node{
			k:    n.k,
			v:    n.v,
			path: n.path,
		}
		// remove values from current node (converting it to mid node)
		n.k = nil
		n.v = nil
		n.path = nil
		if err := n.downUntilDivergence(p, currLvl, oldLeaf, leaf); err != nil {
			return err
		}
	default:
		return fmt.Errorf("ERR")
	}

	return nil
}

func (n *node) downUntilDivergence(p *params, currLvl int, oldLeaf, newLeaf *node) error {
	if currLvl > p.maxLevels-1 {
		return fmt.Errorf("max virtual level %d", currLvl)
	}

	// if oldLeaf.path[currLvl+1] != newLeaf.path[currLvl+1] {
	if oldLeaf.path[currLvl] != newLeaf.path[currLvl] {
		// reached divergence in next level
		// if newLeaf.path[currLvl+1] {
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

func (n *node) computeHashes() ([]kv, error) {
	return nil, nil
}
