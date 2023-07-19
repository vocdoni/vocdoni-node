package state

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
)

func TestSetAddressSIK(t *testing.T) {
	c := qt.New(t)
	// create a tree for testing
	dir := t.TempDir()
	s, err := NewState(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	// create a valid leaf
	address := common.HexToAddress("0xF3668000B66c61aAa08aBC559a8C78Ae7E007C2e")
	sik, _ := hex.DecodeString("3a7806f4e0b5bda625d465abf5639ba42ac9b91bafea3b800a4a")
	c.Assert(s.SetAddressSIK(address, sik), qt.IsNil)
	// try to overwrite a valid sik with another
	c.Assert(s.SetAddressSIK(address, sik), qt.ErrorIs, ErrRegisteredValidSIK)
	// mock invalid leaf value with small encoded height
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), make(SIK, sikLeafValueLen).InvalidateAt(10), StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try to update the sik
	c.Assert(s.SetAddressSIK(address, sik), qt.IsNil)
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
	c.Assert(s.InvalidateSIK(address), qt.IsNotNil)
	// mock a deleted sik
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), make(SIK, sikLeafValueLen).InvalidateAt(5), StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try to delete a sik already deleted
	c.Assert(s.InvalidateSIK(address), qt.ErrorIs, ErrSIKAlreadyInvalid)
	// mock a valid sik
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), sik, StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try a success deletion
	s.SetHeight(2)
	c.Assert(s.InvalidateSIK(address), qt.IsNil)
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
	c.Assert(s.SetAddressSIK(address1, sik1), qt.IsNil)
	// check the results
	validSIKs, err := s.ValidSIKRoots()
	c.Assert(err, qt.IsNil)
	c.Assert(len(validSIKs), qt.Equals, 1)
	sikTree, err := s.Tx.DeepSubTree(StateTreeCfg(TreeSIK))
	c.Assert(err, qt.IsNil)
	firstRoot, err := sikTree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(firstRoot, qt.ContentEquals, validSIKs[0])
	// increase the height and include a new sik
	address2 := common.HexToAddress("0x5fb53c1f9b53fba0296f4e8306802d44235c1a11")
	sik2, _ := hex.DecodeString("5fb53c1f9b53fba0296f4e8306802d44235c1a11becc4e6853d0")
	s.SetHeight(33)
	c.Assert(s.SetAddressSIK(address2, sik2), qt.IsNil)
	// check the results
	validSIKs, err = s.ValidSIKRoots()
	c.Assert(err, qt.IsNil)
	c.Assert(len(validSIKs), qt.Equals, 2)
	secondRoot, err := sikTree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(firstRoot, qt.ContentEquals, validSIKs[0])
	c.Assert(secondRoot, qt.ContentEquals, validSIKs[1])
	// increase the height and include a new sik that will delete the rest of sikroots
	address3 := common.HexToAddress("0x2dd603151d817f829b03412f7378e1179b5b2b1c")
	sik3, _ := hex.DecodeString("7ccbc0da9e8d7e469ba60cd898a5b881c99a960c1e69990a3196")
	s.SetHeight(66)
	c.Assert(s.SetAddressSIK(address3, sik3), qt.IsNil)
	// check the results
	validSIKs, err = s.ValidSIKRoots()
	c.Assert(err, qt.IsNil)
	c.Assert(len(validSIKs), qt.Equals, 1)
	thirdRoot, err := sikTree.Root()
	c.Assert(err, qt.IsNil)
	c.Assert(thirdRoot, qt.ContentEquals, validSIKs[0])
}

func Test_heightEncoding(t *testing.T) {
	c := qt.New(t)
	height := uint32(3498223)
	encoded := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 239, 96, 53, 0}
	c.Assert(bytes.Equal(SIK{}.InvalidateAt(height), encoded), qt.IsTrue)
	c.Assert(SIK{}.InvalidateAt(height).DecodeInvalidatedHeight(), qt.Equals, height)

	height = uint32(0)
	encoded = make([]byte, sikLeafValueLen)
	c.Assert(bytes.Equal(SIK{}.InvalidateAt(height), encoded), qt.IsTrue)
	c.Assert(SIK{}.InvalidateAt(height).DecodeInvalidatedHeight(), qt.Equals, height)

	height = uint32(4294967294)
	encoded = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 254, 255, 255, 255}
	c.Assert(bytes.Equal(SIK{}.InvalidateAt(height), encoded), qt.IsTrue)
	c.Assert(SIK{}.InvalidateAt(height).DecodeInvalidatedHeight(), qt.Equals, height)

	height = uint32(16777472)
	encoded = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1}
	c.Assert(bytes.Equal(SIK{}.InvalidateAt(height), encoded), qt.IsTrue)
	c.Assert(SIK{}.InvalidateAt(height).DecodeInvalidatedHeight(), qt.Equals, height)
}

func Test_validSIK(t *testing.T) {
	input := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 239, 96, 53, 0}
	qt.Assert(t, SIK(input).Valid(), qt.IsFalse)

	input, _ = hex.DecodeString("F3668000B66c61aAa08aBC559a8C78Ae7E007C2e")
	qt.Assert(t, SIK(input).Valid(), qt.IsTrue)
}
