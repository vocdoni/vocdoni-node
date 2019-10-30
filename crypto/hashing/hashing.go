// Package hashing allows iOS and Android apps to interact with the DVote ecosystem
package hashing

import (
	"encoding/base64"
	"encoding/hex"

	poseidon "github.com/iden3/go-iden3-crypto/poseidon"
)

// PoseidonHash computes the base64 Poseidon hash of the given hex string
func PoseidonHash(hexPayload string) (b64Hash string, err error) {
	if hexPayload[0:2] == "0x" {
		hexPayload = hexPayload[2:]
	}
	if len(hexPayload)%2 != 0 {
		hexPayload = "0" + hexPayload
	}
	hexPayloadBytes, err := hex.DecodeString(hexPayload)
	if err != nil {
		return "", err
	}

	hashNum, err := poseidon.HashBytes(hexPayloadBytes)
	if err != nil {
		return "", err
	}
	hashNumBytes := hashNum.Bytes()
	return base64.StdEncoding.EncodeToString(hashNumBytes), nil
}
