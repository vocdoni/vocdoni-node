package tree

import (
  "github.com/iden3/go-iden3/db"
  "github.com/iden3/go-iden3/merkletree"
  mkcore "github.com/iden3/go-iden3/core"
  common3 "github.com/iden3/go-iden3/common"
  "os/user"
)

type Tree struct {
    namespace string
    storage string
    tree *merkletree.MerkleTree
}

func (t *Tree) init() error {
  if len(t.storage) < 1 {
    usr, err := user.Current()
    if err == nil {
      t.storage = usr.HomeDir + "/.dvote/Tree"
    } else { t.storage = "./dvoteTree" }
  }
  mtdb, err := db.NewLevelDbStorage(t.storage, false)
	if err != nil {
    return err
	}
	mt, err := merkletree.New(mtdb, 140)
	if err != nil {
	  return err
	}
  t.tree = mt
  return nil
}

func (t *Tree) close() {
	defer t.tree.Storage().Close()
}

func (t *Tree) addClaim(data []byte) error {
  claim := mkcore.NewGenericClaim(t.namespace, "default", data, nil)
  return t.tree.Add(claim)
}

func (t *Tree) genProof(data []byte) (string, error) {
  claim := mkcore.NewGenericClaim(t.namespace, "default", data, nil)
  mp, err := t.tree.GenerateProof(claim.Hi())
  if err!=nil {
    return "", err
  }
  mpHex := common3.BytesToHex(mp)
  return mpHex, nil
}

func (t *Tree) checkProof(data []byte, mpHex string) (bool, error) {
  mp, err := common3.HexToBytes(mpHex)
  if err != nil {
    return false, err
  }
  claim := mkcore.NewGenericClaim(t.namespace, "default", data, nil)
  return merkletree.CheckProof(t.tree.Root(), mp, claim.Hi(), claim.Ht(), t.tree.NumLevels()), nil
}
