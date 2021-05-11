package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
)

// HexBytes is a []byte which encodes as hexadecimal in json, as opposed to the
// base64 default.
// This type mirrors types/encoding.go HexBytes for this package, to avoid cyclical imports
type HexBytes []byte

func (b HexBytes) MarshalJSON() ([]byte, error) {
	enc := make([]byte, hex.EncodedLen(len(b))+2)
	enc[0] = '"'
	hex.Encode(enc[1:], b)
	enc[len(enc)-1] = '"'
	return enc, nil
}

func (b *HexBytes) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string: %q", data)
	}
	data = data[1 : len(data)-1]

	// Strip a leading "0x" prefix, for backwards compatibility.
	if len(data) >= 2 && data[0] == '0' && (data[1] == 'x' || data[1] == 'X') {
		data = data[2:]
	}

	decLen := hex.DecodedLen(len(data))
	if cap(*b) < decLen {
		*b = make([]byte, decLen)
	}
	if _, err := hex.Decode(*b, data); err != nil {
		return err
	}
	return nil
}

// MessageRequest holds a decoded request but does not decode the body
type RequestMessage struct {
	MetaRequest json.RawMessage `json:"request"`

	ID        string         `json:"id"`
	Signature types.HexBytes `json:"signature"`
}

// MetaRequest contains all of the possible request fields.
// Fields must be in alphabetical order
type MetaRequest struct {
	CensusID     string         `json:"censusId,omitempty"`
	CensusURI    string         `json:"censusUri,omitempty"`
	CensusKey    []byte         `json:"censusKey,omitempty"`
	CensusKeys   [][]byte       `json:"censusKeys,omitempty"`
	CensusValue  []byte         `json:"censusValue,omitempty"`
	CensusValues [][]byte       `json:"censusValues,omitempty"`
	CensusDump   []byte         `json:"censusDump,omitempty"`
	Content      []byte         `json:"content,omitempty"`
	Digested     bool           `json:"digested,omitempty"`
	EntityId     types.HexBytes `json:"entityId,omitempty"`
	Hash         []byte         `json:"hash,omitempty"`
	Height       uint32         `json:"height,omitempty"`
	From         int            `json:"from,omitempty"`
	ListSize     int            `json:"listSize,omitempty"`
	Method       string         `json:"method"`
	Name         string         `json:"name,omitempty"`
	Namespace    uint32         `json:"namespace,omitempty"`
	Nullifier    types.HexBytes `json:"nullifier,omitempty"`
	Payload      []byte         `json:"payload,omitempty"`
	ProcessID    types.HexBytes `json:"processId,omitempty"`
	ProofData    types.HexBytes `json:"proofData,omitempty"`
	PubKeys      []string       `json:"pubKeys,omitempty"`
	RootHash     types.HexBytes `json:"rootHash,omitempty"`
	SearchTerm   string         `json:"searchTerm,omitempty"`
	Signature    types.HexBytes `json:"signature,omitempty"`
	Status       string         `json:"status,omitempty"`
	Timestamp    int32          `json:"timestamp"`
	TxIndex      int32          `json:"txIndex,omitempty"`
	Type         string         `json:"type,omitempty"`
	URI          string         `json:"uri,omitempty"`
	WithResults  bool           `json:"withResults,omitempty"`
}

func (r MetaRequest) String() string {
	v := reflect.ValueOf(r)
	t := v.Type()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s:{", r.Method))
	first := true
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		if fv.IsZero() {
			// omit zero values
			continue
		}
		ft := t.Field(i)
		if ft.Name == "Method" {
			continue
		}
		if first {
			first = false
		} else {
			b.WriteString(" ")
		}
		b.WriteString(ft.Name)
		b.WriteString(":")
		if ft.Type.Kind() == reflect.Slice && ft.Type.Elem().Kind() == reflect.Uint8 {
			// print []byte as hexadecimal
			fmt.Fprintf(&b, "%x", fv.Bytes())
		} else if ft.Type.Kind() == reflect.Slice && ft.Type.Elem().Kind() == reflect.Slice &&
			ft.Type.Elem().Elem().Kind() == reflect.Uint8 {
			for _, v := range fv.Interface().([][]byte) {
				fmt.Fprintf(&b, "%x ", v)
			}
		} else {
			fv = reflect.Indirect(fv) // print *T as T
			fmt.Fprintf(&b, "%v", fv.Interface())
		}
	}
	b.WriteString("}")
	return b.String()
}

// ResponseMessage wraps an api response
type ResponseMessage struct {
	MetaResponse json.RawMessage `json:"response"`

	ID        string         `json:"id"`
	Signature types.HexBytes `json:"signature"`
}

// MetaResponse contains all of the possible request fields.
// Fields must be in alphabetical order
// Those fields with valid zero-values (such as bool) must be pointers
type MetaResponse struct {
	APIList              []string                         `json:"apiList,omitempty"`
	Block                *indexertypes.BlockMetadata      `json:"block,omitempty"`
	BlockList            []*indexertypes.BlockMetadata    `json:"blockList,omitempty"`
	BlockTime            *[5]int32                        `json:"blockTime,omitempty"`
	BlockTimestamp       int32                            `json:"blockTimestamp,omitempty"`
	CensusID             string                           `json:"censusId,omitempty"`
	CensusList           []string                         `json:"censusList,omitempty"`
	CensusKeys           [][]byte                         `json:"censusKeys,omitempty"`
	CensusValues         []types.HexBytes                 `json:"censusValues,omitempty"`
	CensusDump           []byte                           `json:"censusDump,omitempty"`
	CommitmentKeys       []Key                            `json:"commitmentKeys,omitempty"`
	Content              []byte                           `json:"content,omitempty"`
	CreationTime         int64                            `json:"creationTime,omitempty"`
	EncryptionPrivKeys   []Key                            `json:"encryptionPrivKeys,omitempty"`
	EncryptionPublicKeys []Key                            `json:"encryptionPubKeys,omitempty"`
	EntityID             string                           `json:"entityId,omitempty"`
	EntityIDs            []string                         `json:"entityIds,omitempty"`
	Envelope             *indexertypes.EnvelopePackage    `json:"envelope,omitempty"`
	Envelopes            []*indexertypes.EnvelopeMetadata `json:"envelopes,omitempty"`
	Files                []byte                           `json:"files,omitempty"`
	Final                *bool                            `json:"final,omitempty"`
	Finished             *bool                            `json:"finished,omitempty"`
	Health               int32                            `json:"health,omitempty"`
	Height               *uint32                          `json:"height,omitempty"`
	InvalidClaims        []int                            `json:"invalidClaims,omitempty"`
	Message              string                           `json:"message,omitempty"`
	Nullifier            string                           `json:"nullifier,omitempty"`
	Nullifiers           *[]string                        `json:"nullifiers,omitempty"`
	Ok                   bool                             `json:"ok"`
	Paused               *bool                            `json:"paused,omitempty"`
	Payload              string                           `json:"payload,omitempty"`
	ProcessID            types.HexBytes                   `json:"processId,omitempty"`
	ProcessIDs           []string                         `json:"processIds,omitempty"`
	Process              *indexertypes.Process            `json:"process,omitempty"`
	ProcessList          []string                         `json:"processList,omitempty"`
	Registered           *bool                            `json:"registered,omitempty"`
	Request              string                           `json:"request"`
	Results              [][]string                       `json:"results,omitempty"`
	RevealKeys           []Key                            `json:"revealKeys,omitempty"`
	Root                 types.HexBytes                   `json:"root,omitempty"`
	Siblings             types.HexBytes                   `json:"siblings,omitempty"`
	Size                 *int64                           `json:"size,omitempty"`
	State                string                           `json:"state,omitempty"`
	Stats                *VochainStats                    `json:"stats,omitempty"`
	Timestamp            int32                            `json:"timestamp"`
	Type                 string                           `json:"type,omitempty"`
	Tx                   *indexertypes.TxPackage          `json:"tx,omitempty"`
	TxList               []*indexertypes.TxMetadata       `json:"txList,omitempty"`
	URI                  string                           `json:"uri,omitempty"`
	ValidatorList        []*models.Validator              `json:"validatorlist,omitempty"`
	ValidProof           *bool                            `json:"validProof,omitempty"`
	Weight               string                           `json:"weight,omitempty"`
}

func (r MetaResponse) String() string {
	v := reflect.ValueOf(r)
	t := v.Type()
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		if fv.IsZero() {
			// omit zero values
			continue
		}
		if b.Len() > 1 {
			b.WriteString(" ")
		}
		ft := t.Field(i)
		b.WriteString(ft.Name)
		b.WriteString(":")
		if ft.Type.Kind() == reflect.Slice && ft.Type.Elem().Kind() == reflect.Uint8 {
			// print []byte as hexadecimal
			fmt.Fprintf(&b, "%x", fv.Bytes())
		} else {
			fv = reflect.Indirect(fv) // print *T as T
			fmt.Fprintf(&b, "%v", fv.Interface())
		}
	}
	b.WriteString("}")
	return b.String()
}

// SetError sets the MetaResponse's Ok field to false, and Message to a string
// representation of v. Usually, v's type will be error or string.
func (r *MetaResponse) SetError(v interface{}) {
	r.Ok = false
	r.Message = fmt.Sprintf("%s", v)
}

// Key associates a key string with an index, so clients can check
// the index of each process key.
type Key struct {
	Idx int    `json:"idx"`
	Key string `json:"key"`
}

// VochainStats contains information about the current Vochain statistics and state
type VochainStats struct {
	BlockHeight      uint32    `json:"block_height"`
	EntityCount      int64     `json:"entity_count"`
	EnvelopeCount    uint64    `json:"envelope_count"`
	ProcessCount     int64     `json:"process_count"`
	ValidatorCount   int       `json:"validator_count"`
	BlockTime        [5]int32  `json:"block_time"`
	BlockTimeStamp   int32     `json:"block_time_stamp"`
	ChainID          string    `json:"chain_id"`
	GenesisTimeStamp time.Time `json:"genesis_time_stamp"`
	Syncing          bool      `json:"syncing"`
}
