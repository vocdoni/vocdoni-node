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
}
