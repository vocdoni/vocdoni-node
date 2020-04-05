// Package tree provides the functions for creating and managing an iden3 merkletree
package tree

import (
	"encoding/base64"
	"encoding/binary"

	"errors"
	"fmt"
	"os"

	common3 "github.com/iden3/go-iden3-core/common"
	"github.com/iden3/go-iden3-core/core/claims"

	"github.com/iden3/go-iden3-core/db"
	"github.com/iden3/go-iden3-core/merkletree"
	"golang.org/x/text/unicode/norm"
)

type Tree struct {
	StorageDir string
	Tree       *merkletree.MerkleTree
	DbStorage  *db.LevelDbStorage
}

const MaxIndexSize = 84  // indexSlot = 112; 28 reserved bytes; 112-28 = 84
const MaxValueSize = 110 // valueSlot = 128; 4 reserver bytes; 14 for size; 128-18 = 110

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
	mtdb, err := db.NewLevelDbStorage(t.StorageDir+"/"+namespace, false)
	if err != nil {
		return err
	}
	mt, err := merkletree.NewMerkleTree(mtdb, 140)
	if err != nil {
		return err
	}
	t.DbStorage = mtdb
	t.Tree = mt
	return nil
}

// Close closes the storage of the merkle tree
func (t *Tree) Close() {
	t.Tree.Storage().Close()
}

func (t *Tree) entry(index, value []byte) (*merkletree.Entry, error) {
	if len(index) > MaxIndexSize {
		return nil, fmt.Errorf("claim index data too big (%d)", len(index))
	}
	if len(value) > MaxValueSize {
		return nil, fmt.Errorf("claim value data too big (%d)", len(value))
	}
	return getEntryFromData(index, value), nil
}

// claim goes to index slot between 32-64 bytes
// claim size goes to data slot between 0-32 bytes
// function assumes data has a correcth lenght
// indexSlot [ 46 bytes used by MT | 84 free ] = 128 bytes
// valueSlot  [ 4 bytes used by MT? | 7 for index size | 7 for value size | 110 free (not used) ] = 128 bytes
func getEntryFromData(claim []byte, extra []byte) *merkletree.Entry {
	var indexSlot [112]byte
	var valueSlot [120]byte

	// save sizes to valueSlot [ [index] [extra] ]
	bs := make([]byte, 32)
	binary.LittleEndian.PutUint32(bs, uint32(len(claim)))
	copy(valueSlot[:], bs[:7]) // save only first 7 bytes (2^7 = 128 is enough for the index size)
	binary.LittleEndian.PutUint32(bs, uint32(len(extra)))
	copy(valueSlot[7:], bs[:7]) // save only first 7 bytes (2^7 = 128 is enough for the value size)

	// copy claim and extra data
	copy(indexSlot[32:], claim[:]) // do not use first 32 bytes of index (reserver for internal operation)
	copy(valueSlot[:14], extra[:]) // copy extra data from byte 14

	//log.Warnf("adding: %x/%d [%08b]", indexSlot[32:], uint32(len(data)), dataSlot)
	return claims.NewClaimBasic(indexSlot, valueSlot).Entry()
}

func getDataFromEntry(e *merkletree.Entry) ([]byte, []byte) {
	var index, value []byte
	indexSizeBytes := e.Value()[0][4:11] // why it is moved to position 4?
	valueSizeBytes := e.Value()[0][11:18]

	indexSize := int(binary.LittleEndian.Uint32(indexSizeBytes[:]))
	index = make([]byte, MaxIndexSize)
	copy(index[:], e.Index()[1][13:]) // why it is moved 13 bytes? (index[0] + 13 = 44 bytes lost?)
	copy(index[18:], e.Index()[2][:])
	copy(index[50:], e.Index()[3][:])

	valueSize := int(binary.LittleEndian.Uint32(valueSizeBytes[:]))
	value = make([]byte, MaxValueSize)
	copy(value[:], e.Value()[0][18:])
	copy(value[14:], e.Value()[1][:])
	copy(value[46:], e.Value()[2][:])
	copy(value[78:], e.Value()[3][:])

	//log.Warnf("recovered: %x/%d", data[:size], size)
	return index[:indexSize], value[:valueSize]
}

// AddClaim adds a new claim to the merkle tree
// A claim is composed of two parts: index and value
//  1.index is mandatory, the data will be used for indexing the claim into to merkle tree
//  2.value is optional, the data will not affect the indexing
// Use value only if index is too small
func (t *Tree) AddClaim(index, value []byte) error {
	if len(index) > MaxIndexSize {
		return fmt.Errorf("claim index data too big (%d)", len(index))
	}
	if len(index) < 4 {
		return fmt.Errorf("claim index too small (%d)", len(index))
	}
	if len(value) > MaxValueSize {
		return fmt.Errorf("claim value data too big (%d)", len(value))
	}
	e := getEntryFromData(index, value)
	return t.Tree.AddEntry(e)
}

// GenProof generates a merkle tree proof that can be later used on CheckProof() to validate it
func (t *Tree) GenProof(index, value []byte) (string, error) {
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
	if len(index) > MaxIndexSize {
		return false, fmt.Errorf("claim index data too big (%d)", len(index))
	}
	if len(value) > MaxValueSize {
		return false, fmt.Errorf("claim value data too big (%d)", len(value))
	}
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
	e := getEntryFromData(index, value)
	hvalue, err := e.HValue()
	if err != nil {
		return false, err
	}
	hindex, err := e.HIndex()
	if err != nil {
		return false, err
	}
	return merkletree.VerifyProof(&rootHash, mp,
		hindex, hvalue), nil
}

// CheckProof validates a merkle proof and its data
func (t *Tree) CheckProof(index, value []byte, mpHex string) (bool, error) {
	return CheckProof(t.Root(), mpHex, index, value)
}

// Root returns the current root hash of the merkle tree
func (t *Tree) Root() string {
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
	if len(root) > 0 {
		rootHash, err = stringToHash(root)
		if err != nil {
			return indexes, values, err
		}
	}
	err = t.Tree.Walk(&rootHash, func(n *merkletree.Node) {
		if n.Type == merkletree.NodeTypeLeaf {
			index, value := getDataFromEntry(n.Entry)
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
