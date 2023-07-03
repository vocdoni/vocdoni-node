package state

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
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
	// mock invalid leaf value with a hysteresis height
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), encodeHeight(10), StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try to update the sik when the threshold is not reached
	c.Assert(s.SetSIK(address, sik), qt.ErrorIs, ErrSIKNotUpdateable)
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), encodeHeight(100), StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// increase the current height to reach hysteresis and try to update again
	err = s.SetSIK(address, sik)
	c.Assert(err, qt.IsNil)
}

func Test_DelSIK(t *testing.T) {
	c := qt.New(t)
	// create a tree for testing
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
