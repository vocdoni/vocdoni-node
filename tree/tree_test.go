package tree

import (
	"encoding/base64"
	"testing"

	"gitlab.com/vocdoni/go-dvote/crypto/hashing"
	"gitlab.com/vocdoni/go-dvote/log"
)

func TestCensus(t *testing.T) {
	log.InitLogger("debug", "stdout")
	root := "0x08ef67b514239063dc655317132ddaa8e5dc8ccb773089d19999c2bde18d8dd5"
	proof := "0x00010000000000000000000000000000000000000000000000000000000000010ebcdf8e6d1c254ff4884616e93d7da69d755c56d0b1cda1e414592cf875acb5"
	pubk := "04c0b80d53d7ee47d6f1b8c8fdc90f44716194f3117be0e133e3a9f61c1444b8d2e210449ad64a72230b25b5b261fb0bb753a86589eb226a4595251ff97f1f672d"
	dataB64, err := hashing.PoseidonHash(pubk)

	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		t.Error(err)
	}
	valid, err := CheckProof(root, proof, data)
	if err != nil {
		t.Error(err)
	}
	if !valid {
		t.Errorf("proof invalid, but should be valid")
	} else {
		t.Log("proof valid")
	}

	root = "283df98838da5ed4d501dbe4135115fd99a36a6282ae5cd468fac09f5694b2ba"
	proof = "0x000b0000000000000000000000000000000000000000000000000000000007ff19759c3273cb9cf3ac819840c2ae34785a6bd5415d3e96e9a80bd37839fd658f20d355e1241f645b6976232a4d3de342d8ecacdebe05dcf0c557e2f852e5ec5811e8b56c3e301170366cd6da8837c04d659ccf41da06e3eeccc6787d4617dce9124d7c13e848577fb6b2b225e310ba51ac15aee2beec7e1d5395d110fe287f811b1b330b33680e2db4b02bfbd1e02961035ee2c02c26086d3ae2778d5837495310741bc6e3401d2563403c4716f3d97dd08ff305ee814bcf46722a48ed219a1b08da46ba43133f1baaec80cc59c05e1715f85188d1df9f3141b49abf35c2031326a51033a7307b1f19e10b60d79100afca69af2ceba32ba8a0914cf8db346cbc1d1c6d9cde5c9ccebc50c446344418b28d6023d652906f09d04be9caf626cca21181a833d2e4e8880ea4f7522d7adecf68d787dcb8ea5eabaa8e4bfe49c6750a1ece4253d32603985b2a880f10adcdeb10d25ecb13239ad48c6cf2df8128a95f"
	claim := "AUdRdmr45pKte7gCUWNsuvCVf6DKvUmsaSVDmudl9lM="
	data, err = base64.StdEncoding.DecodeString(claim)
	//	data, err = hex.DecodeString(claim)
	if err != nil {
		t.Fatal(err)
	}
	valid, err = CheckProof(root, proof, data)
	if !valid {
		t.Errorf("proof is invalid, but should be valid")
	} else {
		t.Log("proof valid")
	}
}
