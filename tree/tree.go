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

const MaxClaimSize = 84 // indexSlot = 112; 28 reserved bytes ; 112-28=84

func (t *Tree) MaxClaimSize() int {
	return MaxClaimSize
}

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

func (t *Tree) Close() {
	t.Tree.Storage().Close()
}

func (t *Tree) Entry(data []byte) (*merkletree.Entry, error) {
	if len(data) > MaxClaimSize {
		return nil, errors.New("claim data too large")
	}
	return getEntryFromData(data), nil
}

// claim goes to index slot between 32-64 bytes
// claim size goes to data slot between 0-32 bytes
// function assumes data has a correcth lenght
func getEntryFromData(data []byte) *merkletree.Entry {
	var indexSlot [112]byte
	var dataSlot [120]byte
	copy(indexSlot[32:], data[:])
	bs := make([]byte, 32)
	binary.LittleEndian.PutUint32(bs, uint32(len(data)))
	copy(dataSlot[:], bs[:])
	//log.Warnf("adding: %x/%d [%08b]", indexSlot[32:], uint32(len(data)), dataSlot)
	return claims.NewClaimBasic(indexSlot, dataSlot).Entry()
}

func getDataFromEntry(e *merkletree.Entry) []byte {
	sizeBytes := e.Value()[0][4:] // why it is moved to position 4?
	size := int(binary.LittleEndian.Uint32(sizeBytes[:]))
	data := make([]byte, MaxClaimSize)
	copy(data[:], e.Index()[1][13:]) // why it is moved 14 bytes? (index[0] + 14 = 46 bytes lost?)
	copy(data[18:], e.Index()[2][:])
	copy(data[50:], e.Index()[3][:])
	//log.Warnf("recovered: %x/%d", data[:size], size)
	return data[:size]
}

// Obsolete
func doPadding(data *[]byte) {
	for i := len(*data); i < MaxClaimSize; i++ {
		*data = append(*data, byte('\x00'))
	}
}

// AddClaim adds a new claim to the merkle tree
// If len(data) is bigger than tree.MaxClaimSize, an error is returned
func (t *Tree) AddClaim(data []byte) error {
	if len(data) > MaxClaimSize {
		return fmt.Errorf("claim is too big (%d)", len(data))
	}
	e := getEntryFromData(data)
	return t.Tree.AddEntry(e)
}

// GenProof generates a merkle tree proof that can be later used on CheckProof() to validate it
func (t *Tree) GenProof(data []byte) (string, error) {
	if len(data) > MaxClaimSize {
		return "", fmt.Errorf("claim data too big (%d)", len(data))
	}
	e, err := t.Entry(data)
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
func CheckProof(root, mpHex string, data []byte) (bool, error) {
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
	if len(data) > MaxClaimSize {
		return false, fmt.Errorf("claim too big (%d)", len(data))
	}
	e := getEntryFromData(data)
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
func (t *Tree) CheckProof(data []byte, mpHex string) (bool, error) {
	return CheckProof(t.Root(), mpHex, data)
}

// Root returns the current root hash of the merkle tree
func (t *Tree) Root() string {
	return common3.HexEncode(t.Tree.RootKey().Bytes())
}

func (t *Tree) Index(data []byte) (string, error) {
	e, err := t.Entry(data)
	if err != nil {
		return "", err
	}
	hash, err := e.HIndex()
	if err != nil {
		return "", err
	}
	index, err := t.Tree.GetDataByIndex(hash)
	return index.String(), err
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
func (t *Tree) DumpPlain(root string, responseBase64 bool) ([]string, error) {
	var response []string
	var err error
	var rootHash merkletree.Hash
	if len(root) > 0 {
		rootHash, err = stringToHash(root)
		if err != nil {
			return response, err
		}
	}
	err = t.Tree.Walk(&rootHash, func(n *merkletree.Node) {
		if n.Type == merkletree.NodeTypeLeaf {
			data := getDataFromEntry(n.Entry)
			if responseBase64 {
				datab64 := base64.StdEncoding.EncodeToString(data)
				response = append(response, datab64)
			} else {
				response = append(response, string(norm.NFC.Bytes(data)))
			}
		}
	})
	return response, err
}

func (t *Tree) ImportDump(claims []string) error {
	return t.Tree.ImportDumpedClaims(claims)
}

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
