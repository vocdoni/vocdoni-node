package tree

import (
	"encoding/base64"
	"testing"

	"gitlab.com/vocdoni/go-dvote/crypto/hashing"
	"gitlab.com/vocdoni/go-dvote/log"
)

func TestCensus(t *testing.T) {
	log.InitLogger("error", "stdout")
	root := "0xc993b7a04e27ccc6c0d04c65a9ca1a273b2bc44b1f360f16c1e310bba0d4f321"
	proof := "0x000d00000000000000000000000000000000000000000000000000000000131f5e18a3f218b709e81acad773bed88dd08f95648b743c04f00e02b4b53f296c08e7877cb7252e5babb4d4e711296b0c606e2cca671548c98b520799217770362e12680cab9f6ff784d7ad95d95bdd1d1e563ce4677e3f38b18a7dd1ec2836330a53ac64afa63be5680421095c9331b271440a05f4d347c6c6eccec49a9afb602287f264b370393b3c24ca56d9baf489a6c17e93d69758097092d86b9967bfbd28412cf4f6928b8986b95adb8c77d9d6544a44a5290d68a255d29c5fb8754d2703a8f2d753f2d37a4017c26a6d2c8cfd21e298fe88772695f4d6667c926119171509c21ccafb2cc12eee7a610dcda74914d06a60c26a919d8d4110551c52c43f17"
	pubk := "0492fdb88d6f7f50d3bae0000311b955d49489b8d7cc2530898f42bb06c58fc1d3a7117f7c28dd390bba7af103767bcac3182c111e0b2f90210383718a31fd1085"
	dataB64, err := hashing.PoseidonHash(pubk)
	if err != nil {
		t.Fatal(err)
	}

	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := CheckProof(root, proof, data)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatalf("proof invalid, but should be valid")
	} else {
		t.Log("proof valid")
	}
	root = "0xb3ea57d7db3e1f0eadb2394538dfa5351f2515457f8da37d21293baa165a2405"
	proof = "0x000a00000000000000000000000000000000000000000000000000000000033ff65b97f3b441425fa49aef4d12659f5027b3fa21a00ae59a5c18ed095045df002375e810b093f1184d28e0ae9dca4d6fd46a31061c93301865604e852f95611fab515fef39d102b9045882ddae8f6bd9e2b3d6b4d829a778b5588116d6fc1b2446cc117711bb00f5091e3777034619d6337f9f415583b15740417e2ad21dba207d9e1f754da9a32306cd7e0489840c349aeb0f636cb6195b24425c4fa894ba0ec092185a92f32f1e94210dc5d207eb64ed5114aff197e85b28c7ddcc4dd8750c9aeeda5fdba4246c8f8ccb1a8af3d594cbd70738d405367164145d8f16caa2158dab2aa666be614c68d1ffcf5ff2e37ee55c7aa6b99203195ea485aaed2d6811"
	claim := "LoKdiYd1Raof9izlc0OX8pMhFAi5OHnPC6Rzhy7Uhhg="
	data, err = base64.StdEncoding.DecodeString(claim)
	if err != nil {
		t.Fatal(err)
	}
	valid, err = CheckProof(root, proof, data)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Errorf("proof is invalid, but should be valid")
	} else {
		t.Log("proof valid")
	}
}
