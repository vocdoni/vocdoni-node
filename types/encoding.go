package types

import (
	"encoding/hex"
	"fmt"
)

// HexBytes is a []byte which encodes as hexadecimal in json, as opposed to the
// base64 default.
type HexBytes []byte

func (b *HexBytes) String() string {
	return hex.EncodeToString(*b)
}

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

// HexStringToHexBytes converts a hex string to a HexBytes.
// It strips a leading '0x' or '0X' if found, for backwards compatibility.
// Panics if the string is not a valid hex string.
func HexStringToHexBytes(hexString string) HexBytes {
	// Strip a leading "0x" prefix, for backwards compatibility.
	if len(hexString) >= 2 && hexString[0] == '0' && (hexString[1] == 'x' || hexString[1] == 'X') {
		hexString = hexString[2:]
	}
	b, err := hex.DecodeString(hexString)
	if err != nil {
		panic(err)
	}
	return b
}
