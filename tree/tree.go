package tree

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os/user"

	common3 "github.com/iden3/go-iden3-core/common"
	mkcore "github.com/iden3/go-iden3-core/core"
	db "github.com/iden3/go-iden3-core/db"
	merkletree "github.com/iden3/go-iden3-core/merkletree"
	"golang.org/x/text/unicode/norm"
)

type Tree struct {
	Storage   string
	Tree      *merkletree.MerkleTree
	DbStorage *db.LevelDbStorage
}

const maxClaimSize = 62

func (t *Tree) Init(namespace string) error {
	if len(t.Storage) < 1 {
		if len(namespace) < 1 {
			return errors.New("namespace not valid")
		}
		usr, err := user.Current()
		if err == nil {
			t.Storage = usr.HomeDir + "/.dvote/census"
		} else {
			t.Storage = "./dvoteTree"
		}
	}
	mtdb, err := db.NewLevelDbStorage(t.Storage+"/"+namespace, false)
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
	defer t.Tree.Storage().Close()
}

func (t *Tree) GetClaim(data []byte) (*mkcore.ClaimBasic, error) {
	if len(data) > maxClaimSize {
		return nil, errors.New("claim data too large")
	}
	for i := len(data); i <= maxClaimSize; i++ {
		data = append(data, '\x00')
	}
	var indexSlot [400 / 8]byte
	var dataSlot [maxClaimSize]byte
	copy(indexSlot[:], data[:400/8])
	copy(dataSlot[:], data[:maxClaimSize])
	e := mkcore.NewClaimBasic(indexSlot, dataSlot)
	return e, nil
}

func (t *Tree) AddClaim(data []byte) error {
	e, err := t.GetClaim(data)
	if err != nil {
		return err
	}
	return t.Tree.Add(e.Entry())
}

func (t *Tree) GenProof(data []byte) (string, error) {
	e, err := t.GetClaim(data)
	if err != nil {
		return "", err
	}
	mp, err := t.Tree.GenerateProof(e.Entry().HIndex(), nil)
	if err != nil {
		return "", err
	}
	mpHex := common3.HexEncode(mp.Bytes())
	return mpHex, nil
}

func (t *Tree) CheckProof(data []byte, mpHex string) (bool, error) {
	mpBytes, err := common3.HexDecode(mpHex)
	if err != nil {
		return false, err
	}
	mp, err := merkletree.NewProofFromBytes(mpBytes)
	if err != nil {
		return false, err
	}
	e, err := t.GetClaim(data)
	if err != nil {
		return false, err
	}
	return merkletree.VerifyProof(t.Tree.RootKey(), mp,
		e.Entry().HIndex(), e.Entry().HValue()), nil
}

func (t *Tree) GetRoot() string {
	return common3.HexEncode(t.Tree.RootKey().Bytes())
}

func (t *Tree) GetIndex(data []byte) (string, error) {
	e, err := t.GetClaim(data)
	if err != nil {
		return "", err
	}
	index, err := t.Tree.GetDataByIndex(e.Entry().HIndex())
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
			data := bytes.Trim(n.Value()[65:], "\x00")
			data = bytes.Replace(data, []byte("\x00"), nil, -1)
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
