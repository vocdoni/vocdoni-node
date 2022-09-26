package types

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
)

func TestVoterID(t *testing.T) {
	// check nil specific case for voterID
	vID := VoterID{}
	qt.Assert(t, vID.IsNil(), qt.IsTrue)
	qt.Assert(t, vID.Nil(), qt.DeepEquals, []byte{})
	// generate random ethereum sign key
	signKey := &ethereum.SignKeys{}
	qt.Assert(t, signKey.Generate(), qt.IsNil)
	// create voterID with type ECDSA and append eth signkey public key
	var vID2 VoterID = VoterID{VoterIDTypeECDSA}
	vID2 = append(vID2, signKey.PublicKey()...)
	// check VoterIDType
	qt.Assert(t, vID2.Type(), qt.Equals, VoterIDTypeECDSA)
	// check VoterID address
	vID2Addr, err := vID2.Address()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, vID2Addr, qt.DeepEquals, signKey.Address().Bytes())
	// check VoterID type to string
	qt.Assert(t, vID2.VoterIDTypeToString(), qt.Equals, "ECDSA")
}
