package types

import (
	"errors"

	"go.vocdoni.io/dvote/crypto/ethereum"
)

type DataStore struct {
	Datadir string
}

// TODO: use an array, and possibly declare methods to encode/decode as hex.

type ProcessID = []byte

type EntityID = []byte

type CensusRoot = []byte

// VoterID is the indentifier of a voter.
// The first byte of the slice indicates one of the supported identifiers
// For example for an Ethereum public key the VoterID is [1, pubkb0, pubkb1, ...]
// where pubkb0 is the first byte of the Ethereum public key
type VoterID []byte

// VoterIDType represents the type of a voterID
type VoterIDType = uint8

const (
	VoterIDTypeUndefined VoterIDType = 0
	VoterIDTypeECDSA     VoterIDType = 1
)

// Enum value map for VoterIDType.
var voterIDTypeName = map[VoterIDType]string{
	VoterIDTypeUndefined: "UNDEFINED",
	VoterIDTypeECDSA:     "ECDSA",
}

var errUnsupportedVoterIDType error = errors.New("voterID type not supported")

// Type returns the VoterID type defined in VoterIDTypeName
func (v VoterID) Type() VoterIDType {
	return VoterIDType(v[0])
}

// VoterIDTypeToString returns the string representation of the VoterIDType
func (v VoterID) VoterIDTypeToString() string {
	return voterIDTypeName[v[0]]
}

// Nil returns the default value for VoterID which is a non-nil slice
func (v VoterID) Nil() []byte {
	return []byte{}
}

// IsNil returns true if the VoterID is empty
func (v VoterID) IsNil() bool {
	return len(v) == 0
}

// Address returns the voterID Address depending on the VoterIDType
func (v VoterID) Address() ([]byte, error) {
	switch v[0] {
	case VoterIDTypeECDSA:
		ethAddr, err := ethereum.AddrFromPublicKey(v[1:])
		if err != nil {
			return nil, err
		}
		return ethAddr.Bytes(), nil
	default:
		return nil, errUnsupportedVoterIDType
	}
}

// TODO: consider using a database/sql interface instead?

type EncodedProtoBuf = []byte

type Nullifier = []byte
