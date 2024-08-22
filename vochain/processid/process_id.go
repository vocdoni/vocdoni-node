package processid

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

// These consts are sugar for calling BuildProcessID
const (
	BuildNextProcessID = 0
	BuildLastProcessID = -1
)

// ProcessID is a 32 bytes identifier that holds the following information about the voting process:
//
// - chainID: a 6 bytes trunked hash of the blockchain identifier
//
// - organization: hex address of the wallet creating the process
//
// - census origin: kind of census used for the process (from protobuf models)
//
// - envelope type: ballot properties configuration
//
// - nonce: an incremental uint32 number
//
// processID: 395b41cc9abcd8da6bf26964af9d7eed9e03e53415d37aa96045000600000001 represents
// chID:vocdoni-bizono addr:0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045 pt:arbo nc:1
type ProcessID struct {
	chainID          string
	organizationAddr common.Address
	censusOrigin     uint8
	envType          uint8
	nonce            uint32
}

// Marshal encodes to bytes:
// processID = chainID[6bytes] + organizationAddress[20bytes] + proofType[1byte] + envelopeType[1byte] + nonce[4bytes]
func (p *ProcessID) Marshal() []byte {
	chainID := ethereum.HashRaw([]byte(p.chainID))

	nonce := make([]byte, 4)
	binary.BigEndian.PutUint32(nonce, p.nonce)

	proofType := make([]byte, 2)
	binary.BigEndian.PutUint16(proofType, uint16(p.censusOrigin))

	envType := make([]byte, 2)
	binary.BigEndian.PutUint16(envType, uint16(p.envType))

	var id bytes.Buffer
	id.Write(chainID[:6])
	id.Write(p.organizationAddr.Bytes()[:20])
	id.Write(proofType[1:])
	id.Write(envType[1:])
	id.Write(nonce[:4])
	return id.Bytes()
}

// MarshalBinary implements the BinaryMarshaler interface
func (p *ProcessID) MarshalBinary() (data []byte, err error) {
	return p.Marshal(), nil
}

// UnmarshalBinary implements the BinaryMarshaler interface
func (p *ProcessID) UnmarshalBinary(data []byte) error {
	return p.Unmarshal(data)
}

// Unmarshal decodes a 32 byte payload into the process ID fields
func (p *ProcessID) Unmarshal(pid []byte) error {
	if len(pid) != 32 {
		return fmt.Errorf("processID length not correct")
	}

	p.chainID = fmt.Sprintf("%x", pid[:4])

	baddr := make([]byte, 20)
	copy(baddr, pid[6:26])
	p.organizationAddr = common.BytesToAddress(baddr)

	p.censusOrigin = uint8(binary.BigEndian.Uint16(append([]byte{0x00}, pid[26:27]...)))
	if p.censusOrigin == 0 {
		return fmt.Errorf("cannot unmarshal processID: census origin is invalid")
	}

	p.envType = uint8(binary.BigEndian.Uint16(append([]byte{0x00}, pid[27:28]...)))
	if types.ProcessesContractMaxEnvelopeType < p.envType {
		return fmt.Errorf("cannot unmarshal processID: overflow on envelope type %d", p.envType)
	}

	p.nonce = binary.BigEndian.Uint32(pid[28:32])
	return nil
}

// String returns a human readable representation of process ID
func (p *ProcessID) String() string {
	return fmt.Sprintf("[chID:%s/addr:%s/co:%d/et:%d/nc:%d]",
		p.chainID, p.organizationAddr.Hex(), p.censusOrigin, p.envType, p.nonce)
}

// SetChainID sets the process blockchain identifier
func (p *ProcessID) SetChainID(chID string) {
	p.chainID = chID
}

// ChainID returns the process blockchain identifier
func (p *ProcessID) ChainID() string {
	return p.chainID
}

// SetAddr sets the organizer address (20 bytes)
func (p *ProcessID) SetAddr(addr common.Address) {
	p.organizationAddr = common.BytesToAddress(addr.Bytes())
}

// Addr returns the organizer address
func (p *ProcessID) Addr() common.Address {
	return common.BytesToAddress(p.organizationAddr.Bytes())
}

// SetNonce sets the nonce number that identifies the process
func (p *ProcessID) SetNonce(nonce uint32) {
	p.nonce = nonce
}

// Nonce returns the process ID identifier number
func (p *ProcessID) Nonce() uint32 {
	return p.nonce
}

// SetCensusOrigin sets the census origin type
func (p *ProcessID) SetCensusOrigin(csOrg models.CensusOrigin) error {
	if csOrg == 0 || csOrg > 255 {
		return fmt.Errorf("cannot set census origin for processID: invalid value")
	}
	p.censusOrigin = uint8(csOrg)
	return nil
}

// CensusOrigin returns the encoded census origin
func (p *ProcessID) CensusOrigin() models.CensusOrigin {
	return models.CensusOrigin(p.censusOrigin)
}

// SetEnvelopeType sets the envelope type for the process
func (p *ProcessID) SetEnvelopeType(et *models.EnvelopeType) error {
	if et == nil {
		return fmt.Errorf("invalid envelope type: (%s)", log.FormatProto(et))
	}
	p.envType = 0
	if et.Serial {
		p.envType = p.envType | byte(0b00000001)
	}
	if et.Anonymous {
		p.envType = p.envType | byte(0b00000010)
	}
	if et.EncryptedVotes {
		p.envType = p.envType | byte(0b00000100)
	}
	if et.UniqueValues {
		p.envType = p.envType | byte(0b00001000)
	}
	if et.CostFromWeight {
		p.envType = p.envType | byte(0b00010000)
	}
	return nil
}

// EnvelopeType returns the envelope type encoded into the process ID
func (p *ProcessID) EnvelopeType() *models.EnvelopeType {
	return &models.EnvelopeType{
		Serial:         p.envType&byte(0b00000001) > 0,
		Anonymous:      p.envType&byte(0b00000010) > 0,
		EncryptedVotes: p.envType&byte(0b00000100) > 0,
		UniqueValues:   p.envType&byte(0b00001000) > 0,
		CostFromWeight: p.envType&byte(0b00010000) > 0,
	}
}

// BuildProcessID returns a ProcessID constructed in a deterministic way.
//   - with delta == 0, it will return the next process id
//   - with delta != 0, it will return the (next + delta) process id (past or future)
//   - for example, with delta == -1 it will return the last process id
func BuildProcessID(proc *models.Process, state *state.State, delta int32) (*ProcessID, error) {
	pid := new(ProcessID)
	pid.SetChainID(state.ChainID())
	if err := pid.SetEnvelopeType(proc.EnvelopeType); err != nil {
		return nil, err
	}
	if err := pid.SetCensusOrigin(proc.CensusOrigin); err != nil {
		return nil, err
	}
	addr := common.BytesToAddress(proc.EntityId)
	acc, err := state.GetAccount(addr, false)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, fmt.Errorf("account not found %s", addr.Hex())
	}
	pid.SetAddr(addr)
	if int32(acc.GetProcessIndex())+delta < 0 {
		return nil, fmt.Errorf("invalid delta")
	}
	pid.SetNonce(uint32(int32(acc.GetProcessIndex()) + delta))
	return pid, nil
}
