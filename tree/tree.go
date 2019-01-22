package tree

import (
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"

	common3 "github.com/vocdoni/go-iden3/common"
	mkcore "github.com/vocdoni/go-iden3/core"
	"github.com/vocdoni/go-iden3/db"
	"github.com/vocdoni/go-iden3/merkletree"
)

type Tree struct {
	Namespace string
	Storage   string
	Tree      *merkletree.MerkleTree
	DbStorage *db.LevelDbStorage
}

func (t *Tree) Init() error {
	if len(t.Storage) < 1 {
		usr, err := user.Current()
		if err == nil {
			t.Storage = usr.HomeDir + "/.dvote/Tree"
		} else {
			t.Storage = "./dvoteTree"
		}
	}
	mtdb, err := db.NewLevelDbStorage(t.Storage, false)
	if err != nil {
		return err
	}
	mt, err := merkletree.New(mtdb, 140)
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

func (t *Tree) AddClaim(data []byte) error {
	isSnapshot := strings.Contains(t.Namespace, "snapshot.")
	if isSnapshot {
		return errors.New("No new claims can be added to a Snapshot")
	}
	claim := mkcore.NewGenericClaim(t.Namespace, "default", data, nil)
	return t.Tree.Add(claim)
}

func (t *Tree) GenProof(data []byte) (string, error) {
	claim := mkcore.NewGenericClaim(t.Namespace, "default", data, nil)
	mp, err := t.Tree.GenerateProof(claim.Hi())
	if err != nil {
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

func (t *Tree) GetRoot() string {
	return t.Tree.Root().Hex()
}

/* Dump, Export and Snapshot functions are a bit tricky.
   Since go-iden3 does not provide the necessary tools. Low level operations must be performed.
   Once go-iden3 API is mature enough, these functions must be adapted.

   To explore: Values are stored twice in the BD?
*/
func (t *Tree) Dump() ([]string, error) {
	var response []string
	substorage := t.DbStorage.WithPrefix([]byte(t.Namespace))
	nsHash := merkletree.HashBytes([]byte(t.Namespace))
	substorage.Iterate(func(key, value []byte) {
		nsValue := value[5:37]
		if fmt.Sprint(nsHash) == fmt.Sprint(nsValue) {
			response = append(response, string(value[69:]))
		}
	})
	return response, nil
}

func (t *Tree) Snapshot() (string, error) {
	substorage := t.DbStorage.WithPrefix([]byte(t.Namespace))
	nsHash := merkletree.HashBytes([]byte(t.Namespace))
	currentTime := int64(time.Now().Unix())
	snapshotNamespace := fmt.Sprintf("snapshot.%s.%d", t.Namespace, currentTime)
	substorage.Iterate(func(key, value []byte) {
		nsValue := value[5:37]
		if fmt.Sprint(nsHash) == fmt.Sprint(nsValue) {
			data := value[69:]
			//fmt.Printf(" Adding value: %s\n", data)
			claim := mkcore.NewGenericClaim(snapshotNamespace, "default", data, nil)
			err := t.Tree.Add(claim)
			if err != nil {
				fmt.Println(err)
			}
		}
	})
	return snapshotNamespace, nil
}
