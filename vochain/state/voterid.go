package state

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
)

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
	VoterIDTypeZkSnark   VoterIDType = 2
	VoterIDTypeEd25519   VoterIDType = 3
	VoterIDTypeFarcaster VoterIDType = 4
)

// Enum value map for VoterIDType.
var voterIDTypeName = map[VoterIDType]string{
	VoterIDTypeUndefined: "UNDEFINED",
	VoterIDTypeECDSA:     "ECDSA",
	VoterIDTypeZkSnark:   "ZKSNARK",
	VoterIDTypeEd25519:   "ED25519",
	VoterIDTypeFarcaster: "FARCASTER",
}

// NewVoterID creates a new VoterID from a VoterIDType and a key.
func NewVoterID(voterIDType VoterIDType, key []byte) VoterID {
	return append([]byte{voterIDType}, key...)
}

// NewFarcasterVoterID creates a new VoterID for Farcaster from a public key and a fid.
// The public key is hashed (keccak256) with the fid to create the VoterID.
func NewFarcasterVoterID(publicKey []byte, fid uint64) VoterID {
	fidBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(fidBytes, fid)
	hashedPubKey := ethereum.HashRaw(append(publicKey, fidBytes...))
	return NewVoterID(VoterIDTypeFarcaster, hashedPubKey)
}

// Type returns the VoterID type defined in VoterIDTypeName
func (v VoterID) Type() VoterIDType {
	return v[0]
}

// VoterIDTypeToString returns the string representation of the VoterIDType
func (v VoterID) VoterIDTypeToString() string {
	return voterIDTypeName[v[0]]
}

// Nil returns the default value for VoterID which is a non-nil slice
func (VoterID) Nil() []byte {
	return []byte{}
}

// IsNil returns true if the VoterID is empty.
func (v VoterID) IsNil() bool {
	return len(v) == 0
}

// Bytes returns the bytes of the VoterID without the first byte which indicates the type.
func (v VoterID) Bytes() []byte {
	if len(v) < 2 {
		return nil
	}
	return v[1:]
}

// Address returns the voterID Address depending on the VoterIDType.
// Returns nil if the VoterIDType is not supported or the address cannot be obtained.
func (v VoterID) Address() []byte {
	if len(v) < 2 {
		return nil
	}
	switch v[0] {
	case VoterIDTypeECDSA:
		ethAddr, err := ethereum.AddrFromPublicKey(v[1:])
		if err != nil {
			return nil
		}
		return ethAddr.Bytes()
	case VoterIDTypeZkSnark:
		return v[1:]
	case VoterIDTypeEd25519:
		return common.BytesToAddress(ethereum.HashRaw(v[1:])).Bytes()
	case VoterIDTypeFarcaster:
		return common.BytesToAddress(v[1:]).Bytes()
	default:
		return nil
	}
}
