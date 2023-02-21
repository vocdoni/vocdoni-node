package zk

import (
	"encoding/hex"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
)

func TestAddressFromBytes(t *testing.T) {
	c := qt.New(t)

	_, err := AddressFromBytes(nil)
	c.Assert(err, qt.IsNotNil)
	_, err = AddressFromBytes([]byte("ZZZZ"))
	c.Assert(err, qt.IsNotNil)
	input := []byte("6430ab787ad5130942369901498a118fade013ebab5450efbfb6acac66d8fb88")
	zkAddr, err := AddressFromBytes(input)
	c.Assert(err, qt.IsNil)

	expectedPrivKey := "6735248701457559886126785742277482466576784161746903995071090348762482970571"
	c.Assert(zkAddr.PrivKey.String(), qt.Equals, expectedPrivKey)
	expectedPubKey := "12019150563308728469741609856876966791119787897175240651244842581859372505224"
	c.Assert(zkAddr.PubKey.String(), qt.Equals, expectedPubKey)
	expectedAddr := "8818461a07d5b7394446342a319aea2f5f0efe98"
	c.Assert(zkAddr.String(), qt.Equals, expectedAddr)
	c.Assert(zkAddr.Bytes(), qt.HasLen, defaultZkAddrLen)
}

func TestAddressFromString(t *testing.T) {
	c := qt.New(t)

	input := "6430ab787ad5130942369901498a118fade013ebab5450efbfb6acac66d8fb88"
	zkAddr, err := AddressFromString(input)
	c.Assert(err, qt.IsNil)

	expectedPrivKey := "6735248701457559886126785742277482466576784161746903995071090348762482970571"
	c.Assert(zkAddr.PrivKey.String(), qt.Equals, expectedPrivKey)
	expectedPubKey := "12019150563308728469741609856876966791119787897175240651244842581859372505224"
	c.Assert(zkAddr.PubKey.String(), qt.Equals, expectedPubKey)
	expectedAddr := "8818461a07d5b7394446342a319aea2f5f0efe98"
	c.Assert(zkAddr.String(), qt.Equals, expectedAddr)
	c.Assert(zkAddr.Bytes(), qt.HasLen, defaultZkAddrLen)
}

func TestAddressFromSignKeys(t *testing.T) {
	c := qt.New(t)

	input := ethereum.NewSignKeys()
	input.AddHexKey("6430ab787ad5130942369901498a118fade013ebab5450efbfb6acac66d8fb88")
	zkAddr, err := AddressFromSignKeys(input)
	c.Assert(err, qt.IsNil)

	expectedPrivKey := "6735248701457559886126785742277482466576784161746903995071090348762482970571"
	c.Assert(zkAddr.PrivKey.String(), qt.Equals, expectedPrivKey)
	expectedPubKey := "12019150563308728469741609856876966791119787897175240651244842581859372505224"
	c.Assert(zkAddr.PubKey.String(), qt.Equals, expectedPubKey)
	expectedAddr := "8818461a07d5b7394446342a319aea2f5f0efe98"
	c.Assert(zkAddr.String(), qt.Equals, expectedAddr)
	c.Assert(zkAddr.Bytes(), qt.HasLen, defaultZkAddrLen)
}

func TestNewRandAddress(t *testing.T) {
	c := qt.New(t)

	zkAddr, err := NewRandAddress()
	c.Assert(err, qt.IsNil)
	c.Assert(zkAddr.Bytes(), qt.HasLen, defaultZkAddrLen)
}

func TestZkAddressNullifier(t *testing.T) {
	c := qt.New(t)

	zkAddr, err := AddressFromBytes([]byte("6430ab787ad5130942369901498a118fade013ebab5450efbfb6acac66d8fb88"))
	c.Assert(err, qt.IsNil)

	electionId, _ := hex.DecodeString("c5d2460186f77673dcc1cac53a626a10e28f4de7a70a1ac6961a020200000000")
	expected := "12175702186054423772707071181312602181618718988777502075780940568723235915174"
	nullifier, err := zkAddr.Nullifier(electionId)
	c.Assert(err, qt.IsNil)
	c.Assert(nullifier.String(), qt.Equals, expected)

	zkAddr, err = AddressFromBytes([]byte("37c4e1c61da8de4d9d608e6eee41e08319a0cadd6173fc7d17e5b9e016c55231"))
	c.Assert(err, qt.IsNil)
	expected = "1300937744706495110000879872006800567355722156584025848258702214563294421229"
	nullifier, err = zkAddr.Nullifier(electionId)
	c.Assert(err, qt.IsNil)
	c.Assert(nullifier.String(), qt.Equals, expected)
}
