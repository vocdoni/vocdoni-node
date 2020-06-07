package ethereum

import (
	"testing"
)

func TestSignature(t *testing.T) {
	t.Parallel()

	var s SignKeys
	if err := s.Generate(); err != nil {
		t.Fatal(err)
	}
	pub, priv := s.HexString()
	t.Logf("Generated pub:%s priv:%s", pub, priv)
	message := []byte("hello")
	t.Logf("Message to sign: %s", message)
	msgSign, err := s.Sign(message)
	if err != nil {
		t.Fatalf("Error while signing %s", err)
	}
	t.Logf("Signature is %s", msgSign)

	var s2 SignKeys
	if err := s2.AddHexKey(priv); err != nil {
		t.Fatalf("Error importing hex privKey: %s", err)
	}
	pub, priv = s2.HexString()
	t.Logf("Imported pub:%s priv:%s", pub, priv)
	v, err := s.Verify(message, msgSign)
	if err != nil {
		t.Fatalf("Verification error: %s", err)
	}
	if !v {
		t.Fatal("Verification failed!")
	}
	t.Logf("Testing verification... %t", v)

	t.Log("Testing compatibility with standard Ethereum signing libraries")
	hardcodedPriv := "fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19"
	hardcodedSignature := "a0d0ebc374d2a4d6357eaca3da2f5f3ff547c3560008206bc234f9032a866ace6279ffb4093fb39c8bbc39021f6a5c36ef0e813c8c94f325a53f4f395a5c82de01"
	var s3 SignKeys
	if err := s3.AddHexKey(hardcodedPriv); err != nil {
		t.Fatal(err)
	}
	_, priv = s3.HexString()
	if priv != hardcodedPriv {
		t.Fatalf("PrivKey from %s not match the hardcoded one\nGot %s\nMust have %s", hardcodedPriv, priv, hardcodedPriv[2:])
	}
	signature, err := s3.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Signature: %s", signature)
	if signature != hardcodedSignature {
		t.Fatalf("Hardcoded signature %s do not match", hardcodedSignature)
	}
}

func TestAddr(t *testing.T) {
	t.Parallel()

	var s SignKeys
	if err := s.Generate(); err != nil {
		t.Fatal(err)
	}
	pub, priv := s.HexString()
	t.Logf("Generated pub: %s \npriv: %s", pub, priv)
	addr1 := s.EthAddrString()
	addr2, err := AddrFromPublicKey(pub)
	t.Logf("Recovered address from pubKey %s", addr2)
	if err != nil {
		t.Fatal(err)
	}
	if addr1 != addr2 {
		t.Fatalf("Calculated address from pubKey do not match: %s != %s", addr1, addr2)
	}
	msg := []byte("hello vocdoni")
	signature, err := s.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Signature created: %s", signature)
	addr3, err := AddrFromSignature(msg, signature)
	if err != nil {
		t.Fatal(err)
	}
	// addr3s := fmt.Sprintf("%x", addr3)
	if addr3 != addr2 {
		t.Fatalf("Extracted signature address do not match: %s != %s", addr2, addr3)
	}

	if err := s.AddAuthKey(addr3); err != nil {
		t.Fatal(err)
	}
	v, _, err := s.VerifySender(msg, signature)
	if err != nil {
		t.Fatal(err)
	}
	if !v {
		t.Fatal("Cannot verify sender")
	}

	v, err = s.Verify(msg, signature)
	if err != nil {
		t.Fatal(err)
	}
	if !v {
		t.Fatal("Cannot verify signature")
	}

	msg = []byte("bye-bye vocdoni")
	signature2, err := s.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	addr4, err := AddrFromSignature(msg, signature2)
	if err != nil {
		t.Fatal(err)
	}
	if addr4 != addr3 {
		t.Fatal("extracted address from second message do not match")
	}
	t.Logf("%s == %s", addr3, addr4)
}
