// Package blocktypes defines the block and transaction types stored by the
// single-node block store and served through the API.
package blocktypes

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// HashSize is the size in bytes of block and transaction hashes.
const HashSize = sha256.Size

// HexBytes is a byte slice that is encoded in JSON as an uppercase hex string.
type HexBytes []byte

// Bytes returns the underlying byte slice.
func (b HexBytes) Bytes() []byte {
	return b
}

// String returns the uppercase hex representation of the bytes.
func (b HexBytes) String() string {
	return strings.ToUpper(hex.EncodeToString(b))
}

// MarshalJSON encodes the bytes as an uppercase hex string.
func (b HexBytes) MarshalJSON() ([]byte, error) {
	return []byte(`"` + b.String() + `"`), nil
}

// UnmarshalJSON decodes a hex string, in any case.
func (b *HexBytes) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid hex string %s", data)
	}
	decoded, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*b = decoded
	return nil
}

// Tx is a raw transaction as stored in a block.
type Tx []byte

// Hash returns the SHA-256 hash of the transaction bytes.
func (tx Tx) Hash() []byte {
	h := sha256.Sum256(tx)
	return h[:]
}

// Key returns the SHA-256 hash of the transaction bytes as a fixed size array.
func (tx Tx) Key() [HashSize]byte {
	return sha256.Sum256(tx)
}

// BlockID identifies a block by its hash.
type BlockID struct {
	Hash HexBytes `json:"hash"`
}

// Header holds the block metadata.
type Header struct {
	ChainID         string    `json:"chain_id"`
	Height          int64     `json:"height"`
	Time            time.Time `json:"time"`
	LastBlockID     BlockID   `json:"last_block_id"`
	DataHash        HexBytes  `json:"data_hash"`
	AppHash         HexBytes  `json:"app_hash"`
	ProposerAddress HexBytes  `json:"proposer_address"`
}

// Hash returns the SHA-256 hash of the deterministic encoding of the header fields.
func (h *Header) Hash() []byte {
	hasher := sha256.New()
	writeField := func(b []byte) {
		var l [8]byte
		binary.BigEndian.PutUint64(l[:], uint64(len(b)))
		hasher.Write(l[:])
		hasher.Write(b)
	}
	var num [8]byte
	writeField([]byte(h.ChainID))
	binary.BigEndian.PutUint64(num[:], uint64(h.Height))
	writeField(num[:])
	binary.BigEndian.PutUint64(num[:], uint64(h.Time.UnixNano()))
	writeField(num[:])
	writeField(h.LastBlockID.Hash)
	writeField(h.DataHash)
	writeField(h.AppHash)
	writeField(h.ProposerAddress)
	return hasher.Sum(nil)
}

// Data holds the transactions of a block.
type Data struct {
	Txs []Tx `json:"txs"`
}

// Hash returns the SHA-256 hash of the concatenated transaction hashes.
func (d *Data) Hash() []byte {
	hasher := sha256.New()
	for _, tx := range d.Txs {
		hasher.Write(tx.Hash())
	}
	return hasher.Sum(nil)
}

// Block is a block of transactions.
type Block struct {
	Header `json:"header"`
	Data   `json:"data"`
}

// Hash returns the block hash, which is the hash of its header.
func (b *Block) Hash() HexBytes {
	if b == nil {
		return nil
	}
	return b.Header.Hash()
}
