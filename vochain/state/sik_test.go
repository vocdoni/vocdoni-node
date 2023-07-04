package state

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/util"
)

func TestSetSIK(t *testing.T) {
	c := qt.New(t)
	// create a tree for testing
	dir := t.TempDir()
	s, err := NewState(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	// create a valid leaf
	address := common.HexToAddress("0xF3668000B66c61aAa08aBC559a8C78Ae7E007C2e")
	sik, _ := hex.DecodeString("3a7806f4e0b5bda625d465abf5639ba42ac9b91bafea3b800a4a")
	c.Assert(s.SetSIK(address, sik), qt.IsNil)
	// try to overwrite a valid sik with another
	c.Assert(s.SetSIK(address, sik), qt.ErrorIs, ErrRegisteredValidSIK)
	// mock invalid leaf value with small encoded height
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), encodeHeight(10), StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try to update the sik when the threshold is not reached
	c.Assert(s.SetSIK(address, sik), qt.ErrorIs, ErrSIKNotUpdateable)
	// retry to update the sik increasing the state height and registering new
	// startBlock
	s.SetHeight(50)
	err = s.RegisterStartBlock(common.BytesToAddress(util.RandomBytes(32)).Bytes(), 40)
	c.Assert(err, qt.IsNil)
	// try to update the sik when the threshold is not reached
	c.Assert(s.SetSIK(address, sik), qt.ErrorIs, ErrSIKNotUpdateable)
	// mock a valid encoded height as sik value
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), encodeHeight(100), StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try to update the sik when the threshold is reached
	err = s.SetSIK(address, sik)
	c.Assert(err, qt.IsNil)
}

func TestDelSIK(t *testing.T) {
	c := qt.New(t)
	// create a state for testing
	dir := t.TempDir()
	s, err := NewState(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	// create a valid leaf
	address := common.HexToAddress("0xF3668000B66c61aAa08aBC559a8C78Ae7E007C2e")
	sik, _ := hex.DecodeString("3a7806f4e0b5bda625d465abf5639ba42ac9b91bafea3b800a4a")
	// try to delete it when it not exists yet
	c.Assert(s.DelSIK(address), qt.IsNotNil)
	// mock a deleted sik
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), encodeHeight(5), StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try to delete a sik already deleted
	c.Assert(s.DelSIK(address), qt.ErrorIs, ErrSIKAlreadyInvalid)
	// mock a valid sik
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), sik, StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try a success deletion
	s.SetHeight(2)
	c.Assert(s.DelSIK(address), qt.IsNil)
}

func Test_sikRoots(t *testing.T) {
	c := qt.New(t)
	// create a state for testing
	dir := t.TempDir()
	s, err := NewState(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	// mock height and new sik and update the valid roots
	address1 := common.HexToAddress("0xF3668000B66c61aAa08aBC559a8C78Ae7E007C2e")
	sik1, _ := hex.DecodeString("3a7806f4e0b5bda625d465abf5639ba42ac9b91bafea3b800a4a")
	s.SetHeight(1)
	c.Assert(s.SetSIK(address1, sik1), qt.IsNil)
	// check the results
	validSIKs, err := s.ValidSIKRoots()
	c.Assert(err, qt.IsNil)
	c.Assert(len(validSIKs), qt.Equals, 1)
	processTree, err := s.Tx.DeepSubTree(StateTreeCfg(TreeProcess))
	c.Assert(err, qt.IsNil)
	firstRoot, err := processTree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(firstRoot, qt.ContentEquals, validSIKs[0])
	// increase the height and include a new sik
	address2 := common.HexToAddress("0x5fb53c1f9b53fba0296f4e8306802d44235c1a11")
	sik2, _ := hex.DecodeString("5fb53c1f9b53fba0296f4e8306802d44235c1a11becc4e6853d0")
	s.SetHeight(33)
	c.Assert(s.SetSIK(address2, sik2), qt.IsNil)
	// check the results
	validSIKs, err = s.ValidSIKRoots()
	c.Assert(err, qt.IsNil)
	c.Assert(len(validSIKs), qt.Equals, 2)
	secondRoot, err := processTree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(firstRoot, qt.ContentEquals, validSIKs[0])
	c.Assert(secondRoot, qt.ContentEquals, validSIKs[1])
	// increase the height and include a new sik that will delete the first one
	address3 := common.HexToAddress("0x2dd603151d817f829b03412f7378e1179b5b2b1c")
	sik3, _ := hex.DecodeString("7ccbc0da9e8d7e469ba60cd898a5b881c99a960c1e69990a3196")
	s.SetHeight(66)
	c.Assert(s.SetSIK(address3, sik3), qt.IsNil)
	// check the results
	validSIKs, err = s.ValidSIKRoots()
	c.Assert(err, qt.IsNil)
	c.Assert(len(validSIKs), qt.Equals, 2)
	thirdRoot, err := processTree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(secondRoot, qt.ContentEquals, validSIKs[0])
	c.Assert(thirdRoot, qt.ContentEquals, validSIKs[1])
}

func Test_heightEncoding(t *testing.T) {
	c := qt.New(t)
	height := uint32(3498223)
	encoded := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 239, 96, 53, 0}
	c.Assert(encodeHeight(height), qt.ContentEquals, encoded)
	c.Assert(decodeHeight(encodeHeight(height)), qt.Equals, height)

	height = uint32(0)
	encoded = make([]byte, sikLeafValueLen)
	c.Assert(encodeHeight(height), qt.ContentEquals, encoded)
	c.Assert(decodeHeight(encodeHeight(height)), qt.Equals, height)

	height = uint32(4294967294)
	encoded = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 254, 255, 255, 255}
	c.Assert(encodeHeight(height), qt.ContentEquals, encoded)
	c.Assert(decodeHeight(encodeHeight(height)), qt.Equals, height)

	height = uint32(16777472)
	encoded = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1}
	c.Assert(encodeHeight(height), qt.ContentEquals, encoded)
	c.Assert(decodeHeight(encodeHeight(height)), qt.Equals, height)
}

func Test_validSIK(t *testing.T) {
	input := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 239, 96, 53, 0}
	qt.Assert(t, validSIK(input), qt.IsFalse)

	input, _ = hex.DecodeString("F3668000B66c61aAa08aBC559a8C78Ae7E007C2e")
	qt.Assert(t, validSIK(input), qt.IsTrue)
}
