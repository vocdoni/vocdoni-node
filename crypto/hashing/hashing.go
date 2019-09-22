// Package hashing allows iOS and Android apps to interact with the DVote ecosystem
package hashing

import (
	"encoding/base64"
	"encoding/hex"

	poseidon "github.com/iden3/go-iden3-crypto/poseidon"
)

// PoseidonHash computes the Poseidon hash of the given hex string
func PoseidonHash(hexPayload string) (hexHash string, err error) {
	if hexPayload[0:2] == "0x" {
		hexPayload = hexPayload[2:]
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
