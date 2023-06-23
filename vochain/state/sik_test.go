package state

import (
	"encoding/hex"
	"fmt"
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
	leafValue := encodeHysteresis(10)
	s.Tx.Lock()
	err = s.Tx.DeepSet(address.Bytes(), leafValue, StateTreeCfg(TreeSIK))
	s.Tx.Unlock()
	c.Assert(err, qt.IsNil)
	// try to update the sik without reach hysteresis
	s.SetHeight(5)
	c.Assert(s.SetSIK(address, sik), qt.ErrorIs, ErrHysteresisNotReached)
	// increase the current height to reach hysteresis and try to update again
	s.SetHeight(10)
	err = s.SetSIK(address, sik)
	fmt.Println(err)
	c.Assert(err, qt.IsNil)
}

func Test_hysteresis(t *testing.T) {
	c := qt.New(t)
	height := uint32(3498223)
	encoded := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 239, 96, 53, 0}
	c.Assert(encodeHysteresis(height), qt.ContentEquals, encoded)
	c.Assert(decodeHysteresis(encodeHysteresis(height)), qt.Equals, height)

	height = uint32(0)
	encoded = make([]byte, hysteresisLen)
	c.Assert(encodeHysteresis(height), qt.ContentEquals, encoded)
	c.Assert(decodeHysteresis(encodeHysteresis(height)), qt.Equals, height)

	height = uint32(4294967294)
	encoded = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 254, 255, 255, 255}
	c.Assert(encodeHysteresis(height), qt.ContentEquals, encoded)
	c.Assert(decodeHysteresis(encodeHysteresis(height)), qt.Equals, height)

	height = uint32(16777472)
	encoded = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1}
	c.Assert(encodeHysteresis(height), qt.ContentEquals, encoded)
	c.Assert(decodeHysteresis(encodeHysteresis(height)), qt.Equals, height)

}

func Test_validSIK(t *testing.T) {
	input := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 239, 96, 53, 0}
	qt.Assert(t, validSIK(input), qt.IsFalse)

	input, _ = hex.DecodeString("F3668000B66c61aAa08aBC559a8C78Ae7E007C2e")
	qt.Assert(t, validSIK(input), qt.IsTrue)
}
