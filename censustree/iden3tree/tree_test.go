package iden3tree

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/iden3/go-iden3-core/core/claims"
)

func TestCheckProof(t *testing.T) {
	root := "0x9170d2c78b34ccd9ba51db7f2cf1cf0ee74c94721bf361448a2daee31183750a"
	proof := "0x000600000000000000000000000000000000000000000000000000000000003feba517158215284cea0012d627844f6fa98b8a347064d975dcfa35ddf54ae52c5c7f951324159feaf05f3532b1fd760add68f0f64b3d34c15aabac8822fd010246f1dd6a8cbe786e9d0a89d15a3701faa63e22c5267f74b483ede2231591d41c483b39dbfbb20c2b97dfb9deca41655f05d813bc9c30d6a88acb9dfd50465928ca072e891978c33f88b1470275498e2250cb5fce9f4e5ebd2fe6caf5cd485b05b34121d4bebfac5798ddd7dde629f0b3ce818c22cc0d8d9d191238ff614f7a21"
	claim := "HHDiCW3GJivvFHBqw2G+8jnL/tDtD4vNyhKjWokuLaY="
	data, err := base64.StdEncoding.DecodeString(claim)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := CheckProof(root, proof, data, []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Errorf("proof is invalid, but should be valid")
	} else {
		t.Log("proof valid")
	}
}

func TestClaimParsers(t *testing.T) {
	indexData := []byte("test")
	valueData := []byte("test")

	var indexSlot [claims.IndexSlotLen]byte
	var valueSlot [claims.ValueSlotLen]byte
	copy(indexSlot[:], indexData)
	valueSlot[0] = byte(len(indexData))
	valueSlot[1] = byte(len(valueData))
	copy(valueSlot[2:], valueData)

	claim := claims.NewClaimBasic(indexSlot, valueSlot)

	indexGetted, valueGetted := getDataFromClaim(claim)
	if !bytes.Equal(indexGetted, indexData) {
		t.Errorf("index %v not equal to expected %v", indexGetted, indexData)
	}
	if !bytes.Equal(valueGetted, valueData) {
		t.Errorf("value %v not equal to expected %v", valueGetted, valueData)
	}

	gettedClaim, err := getClaimFromData(indexData, valueData)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gettedClaim.Entry().Bytes(), claim.Entry().Bytes()) {
		t.Errorf("getClaimFromData returns an unexpected claim")
	}

	// check that with a index bigger than maximum allowed, the parser doesn't overflow when setting the len in the valueSlot[0] byte
	var indexOverflow [300]byte
	copy(indexOverflow[:], indexData)
	_, err = getClaimFromData(indexOverflow[:], valueData)
	if err == nil {
		t.Errorf("should return error to avoid overflow")
	}
	// check that with a value/extra bigger than maximum allowed, the parser doesn't overflow when setting the len in the valueSlot[1] byte
	var valueOverflow [300]byte
	copy(indexData, valueOverflow[:])
	_, err = getClaimFromData(indexOverflow[:], valueData)
	if err == nil {
		t.Errorf("should return error to avoid overflow")
	}
}
