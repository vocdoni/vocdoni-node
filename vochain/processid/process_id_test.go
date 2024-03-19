package processid

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	test_vbAddr = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
	test_chID   = "vocdoni-bizono"
)

func TestProcessID(t *testing.T) {
	pid := ProcessID{}
	pid.SetAddr(common.HexToAddress(test_vbAddr))
	pid.SetNonce(1)
	pid.SetChainID(test_chID)
	csOrg := models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED
	pid.SetCensusOrigin(csOrg)
	pid.SetNonce(1)
	pid.SetEnvelopeType(&models.EnvelopeType{Serial: true, CostFromWeight: true, EncryptedVotes: true})

	et := pid.EnvelopeType()
	qt.Assert(t, et.Serial, qt.Equals, true)
	qt.Assert(t, et.CostFromWeight, qt.Equals, true)
	qt.Assert(t, et.UniqueValues, qt.Equals, false)
	qt.Assert(t, et.EncryptedVotes, qt.Equals, true)
	qt.Assert(t, et.Anonymous, qt.Equals, false)

	pid2 := ProcessID{}
	b, err := hex.DecodeString("395b41cc9abcd8da6bf26964af9d7eed9e03e53415d37aa96045021300000002")
	qt.Assert(t, err, qt.IsNil)
	err = pid2.Unmarshal(b)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, pid2.Nonce(), qt.Equals, uint32(2))
	qt.Assert(t, pid2.CensusOrigin(), qt.Equals, models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED)
	qt.Assert(t, pid2.envType, qt.Equals, uint8(0x13))

	// 0x13 = 10011
	et = pid2.EnvelopeType()
	qt.Assert(t, et.Serial, qt.Equals, true)
	qt.Assert(t, et.CostFromWeight, qt.Equals, true)
	qt.Assert(t, et.UniqueValues, qt.Equals, false)
	qt.Assert(t, et.EncryptedVotes, qt.Equals, false)
	qt.Assert(t, et.Anonymous, qt.Equals, true)

	csOrg = models.CensusOrigin_ERC20
	err = pid.SetCensusOrigin(csOrg)
	qt.Assert(t, err, qt.IsNil)
	err = pid2.Unmarshal(pid.Marshal())
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, pid2.CensusOrigin(), qt.Equals, models.CensusOrigin_ERC20)
	qt.Assert(t, pid2.Addr().Hex(), qt.Equals, test_vbAddr)

	err = pid.SetCensusOrigin(777)
	qt.Assert(t, err, qt.IsNotNil)
}
