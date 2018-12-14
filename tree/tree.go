package tree

import (
  "github.com/iden3/go-iden3/db"
  "github.com/iden3/go-iden3/merkletree"
  mkcore "github.com/iden3/go-iden3/core"
  common3 "github.com/iden3/go-iden3/common"
  "os/user"
)

type Tree struct {
    Namespace string
    Storage string
    Tree *merkletree.MerkleTree
}

func (t *Tree) Init() error {
  if len(t.Storage) < 1 {
    usr, err := user.Current()
    if err == nil {
      t.Storage = usr.HomeDir + "/.dvote/Tree"
    } else { t.Storage = "./dvoteTree" }
  }
  mtdb, err := db.NewLevelDbStorage(t.Storage, false)
	if err != nil {
    return err
	}
	mt, err := merkletree.New(mtdb, 140)
	if err != nil {
	  return err
	}
  t.Tree = mt
  return nil
}

func (t *Tree) Close() {
	defer t.Tree.Storage().Close()
}

func (t *Tree) AddClaim(data []byte) error {
  claim := mkcore.NewGenericClaim(t.Namespace, "default", data, nil)
  return t.Tree.Add(claim)
}

func (t *Tree) GenProof(data []byte) (string, error) {
  claim := mkcore.NewGenericClaim(t.Namespace, "default", data, nil)
  mp, err := t.Tree.GenerateProof(claim.Hi())
  if err!=nil {
    return "", err
  }
  mpHex := common3.BytesToHex(mp)
  return mpHex, nil
}

func (t *Tree) CheckProof(data []byte, mpHex string) (bool, error) {
  mp, err := common3.HexToBytes(mpHex)
  if err != nil {
    return false, err
  }
  claim := mkcore.NewGenericClaim(t.Namespace, "default", data, nil)
  return merkletree.CheckProof(t.Tree.Root(), mp, claim.Hi(), claim.Ht(), t.Tree.NumLevels()), nil
}
