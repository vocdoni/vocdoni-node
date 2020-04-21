// Package tree provides the functions for creating and managing an iden3 merkletree
package tree

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	common3 "github.com/iden3/go-iden3-core/common"
	"github.com/iden3/go-iden3-core/core/claims"
	iden3db "github.com/iden3/go-iden3-core/db"

	"github.com/iden3/go-iden3-core/merkletree"
	"golang.org/x/text/unicode/norm"

	"gitlab.com/vocdoni/go-dvote/db"
)

type Tree struct {
	StorageDir     string
	Tree           *merkletree.MerkleTree
	Storage        iden3db.Storage
	LastAccessTime time.Time
	timeMu         sync.RWMutex
}

const (
	MaxIndexSize = claims.IndexSlotLen
	MaxValueSize = claims.ValueSlotLen - 2 // -2 because the 2 first bytes are used to store the length of index and value
)

func (t *Tree) MaxClaimSize() int {
	return MaxIndexSize
}

// Init opens or creates a merkle tree indexed under the namespace name
func (t *Tree) Init(namespace string) error {
	if len(t.StorageDir) < 1 {
		if len(namespace) < 1 {
			return errors.New("namespace not valid")
		}
		home, err := os.UserHomeDir()
		if err == nil {
			t.StorageDir = home + "/.dvote/census"
		} else {
			t.StorageDir = "./dvoteTree"
		}
	}
	storage, err := db.NewIden3Storage(t.StorageDir + "/" + namespace)
	if err != nil {
		return err
	}
	mt, err := merkletree.NewMerkleTree(storage, 140)
	if err != nil {
		return err
	}
	t.Storage = storage
	t.Tree = mt
	t.updateAccessTime()
	return nil
}

// Close closes the storage of the merkle tree
func (t *Tree) Close() {
	t.Tree.Storage().Close()
}

//LastAccess returns the last time the Tree was accessed
func (t *Tree) LastAccess() time.Time {
	t.timeMu.Lock()
	defer t.timeMu.Unlock()
	return t.LastAccessTime
}

// TODO(mvdan): use sync/atomic instead to avoid introducing a bottleneck
func (t *Tree) updateAccessTime() {
	t.timeMu.Lock()
	defer t.timeMu.Unlock()
	t.LastAccessTime = time.Now()
}

func (t *Tree) entry(index, value []byte) (*merkletree.Entry, error) {
	claim, err := getClaimFromData(index, value)
	if err != nil {
		return nil, err
	}
	return claim.Entry(), nil
}

func getClaimFromData(index []byte, value []byte) (*claims.ClaimBasic, error) {
	if len(index) > claims.IndexSlotLen {
		return nil, fmt.Errorf("index len %v can not be bigger than %v", len(index), claims.IndexSlotLen)
	}
	if len(value) > claims.ValueSlotLen {
		return nil, fmt.Errorf("extra len %v can not be bigger than %v", len(value), claims.ValueSlotLen)
	}
	var indexSlot [claims.IndexSlotLen]byte
	var valueSlot [claims.ValueSlotLen]byte
	copy(indexSlot[:], index)
	valueSlot[0] = byte(len(index))
	valueSlot[1] = byte(len(value))
	copy(valueSlot[2:], value) // [2:] due the 2 first bytes used for saving the length of index & value
	return claims.NewClaimBasic(indexSlot, valueSlot), nil
}

func getDataFromClaim(c *claims.ClaimBasic) ([]byte, []byte) {
	indexSize := int(c.ValueSlot[0])
	valueSize := int(c.ValueSlot[1])
	return c.IndexSlot[:indexSize], c.ValueSlot[2 : valueSize+2]
}

// AddClaim adds a new claim to the merkle tree
// A claim is composed of two parts: index and value
//  1.index is mandatory, the data will be used for indexing the claim into to merkle tree
//  2.value is optional, the data will not affect the indexing
// Use value only if index is too small
func (t *Tree) AddClaim(index, value []byte) error {
	t.updateAccessTime()
	if len(index) < 4 {
		return fmt.Errorf("claim index too small (%d), minimum size is 4 bytes", len(index))
	}
	c, err := getClaimFromData(index, value)
	if err != nil {
		return err
	}
	return t.Tree.AddClaim(c)
}

// GenProof generates a merkle tree proof that can be later used on CheckProof() to validate it
func (t *Tree) GenProof(index, value []byte) (string, error) {
	t.updateAccessTime()
	e, err := t.entry(index, value)
	if err != nil {
		return "", err
	}
	hash, err := e.HIndex()
	if err != nil {
		return "", err
	}
	mp, err := t.Tree.GenerateProof(hash, nil)
	if err != nil {
		return "", err
	}
	if !mp.Existence {
		return "", nil
	}
	mpHex := common3.HexEncode(mp.Bytes())
	return mpHex, nil
}

// CheckProof standalone function for checking a merkle proof
func CheckProof(root, mpHex string, index, value []byte) (bool, error) {
	mpBytes, err := common3.HexDecode(mpHex)
	if err != nil {
		return false, err
	}
	mp, err := merkletree.NewProofFromBytes(mpBytes)
	if err != nil {
		return false, err
	}
	rootHash, err := stringToHash(root)
	if err != nil {
		return false, err
	}
	c, err := getClaimFromData(index, value)
	if err != nil {
		return false, err
	}
	hvalue, err := c.Entry().HValue()
	if err != nil {
		return false, err
	}
	hindex, err := c.Entry().HIndex()
	if err != nil {
		return false, err
	}
	return merkletree.VerifyProof(&rootHash, mp,
		hindex, hvalue), nil
}

// CheckProof validates a merkle proof and its data
func (t *Tree) CheckProof(index, value []byte, mpHex string) (bool, error) {
	t.updateAccessTime()
	return CheckProof(t.Root(), mpHex, index, value)
}

// Root returns the current root hash of the merkle tree
func (t *Tree) Root() string {
	t.updateAccessTime()
	return common3.HexEncode(t.Tree.RootKey().Bytes())
}

func stringToHash(hash string) (merkletree.Hash, error) {
	var rootHash merkletree.Hash
	rootBytes, err := common3.HexDecode(hash)
	if err != nil {
		return rootHash, err
	}
	copy(rootHash[:32], rootBytes)
	return rootHash, err
}

// Dump returns the whole merkle tree serialized in a format that can be used on Import
func (t *Tree) Dump(root string) (claims []string, err error) {
	var rootHash merkletree.Hash
	t.updateAccessTime()
	if len(root) > 0 {
		rootHash, err = stringToHash(root)
		if err != nil {
			return
		}
	}
	claims, err = t.Tree.DumpClaims(&rootHash)
	return
}

// Size returns the number of leaf nodes on the merkle tree
func (t *Tree) Size(root string) (int64, error) {
	var err error
	var rootHash merkletree.Hash
	var size int64
	t.updateAccessTime()
	if len(root) > 0 {
		rootHash, err = stringToHash(root)
		if err != nil {
			return size, err
		}
	}
	err = t.Tree.Walk(&rootHash, func(n *merkletree.Node) {
		if n.Type == merkletree.NodeTypeLeaf {
			size++
		}
	})
	return size, err
}

// DumpPlain returns the entire list of added claims for a specific root hash
// If root is not specified, the current one is used
// If responseBase64 is true, the list will be returned base64 encoded
func (t *Tree) DumpPlain(root string, responseBase64 bool) ([]string, []string, error) {
	var indexes, values []string
	var err error
	var rootHash merkletree.Hash
	t.updateAccessTime()
	if len(root) > 0 {
		rootHash, err = stringToHash(root)
		if err != nil {
			return indexes, values, err
		}
	}
	var index, value []byte
	err = t.Tree.Walk(&rootHash, func(n *merkletree.Node) {
		if n.Type == merkletree.NodeTypeLeaf {
			c := claims.NewClaimBasicFromEntry(n.Entry)

			index, value = getDataFromClaim(c)
			if responseBase64 {
				datab64 := base64.StdEncoding.EncodeToString(index)
				indexes = append(indexes, datab64)
				datab64 = base64.StdEncoding.EncodeToString(value)
				values = append(values, datab64)
			} else {
				indexes = append(indexes, string(norm.NFC.Bytes(index)))
				values = append(values, string(norm.NFC.Bytes(value)))
			}
		}
	})
	return indexes, values, err
}

// ImportDump imports a partial or whole tree previously exported with Dump()
func (t *Tree) ImportDump(claims []string) error {
	t.updateAccessTime()
	return t.Tree.ImportDumpedClaims(claims)
}

// Snapshot returns a Tree instance of a exiting merkle root
func (t *Tree) Snapshot(root string) (*Tree, error) {
	rootHash := new(merkletree.Hash)
	snapshotTree := new(Tree)
	var err error
	if len(root) > 0 {
		*rootHash, err = stringToHash(root)
		if err != nil || rootHash == nil {
			return snapshotTree, err
		}
	}
	mt, err := t.Tree.Snapshot(rootHash)
	snapshotTree.Tree = mt
	return snapshotTree, err
}

// HashExist checks if a hash exists as a node in the merkle tree
func (t *Tree) HashExist(hash string) (bool, error) {
	t.updateAccessTime()
	h, err := stringToHash(hash)
	if err != nil {
		return false, err
	}
	n, err := t.Tree.GetNode(&h)
	if err != nil || n == nil {
		return false, nil
	}
	return true, nil
}
