package signature

import (
	"testing"
)

func TestSignature(t *testing.T) {
	t.Log("Testing signature creation and verification")
	var s SignKeys
	s.Generate()
	pub, priv := s.HexString()
	t.Logf("Generated pub:%s priv:%s\n", pub, priv)
	message := "hello"
	t.Logf("Message to sign: %s\n", message)
	msgSign, err := s.Sign(message)
	if err != nil {
		t.Errorf("Error while signing %s\n", err)
	}
	t.Logf("Signature is %s\n", msgSign)

	var s2 SignKeys
	err = s2.AddHexKey(priv)
	if err != nil {
		t.Errorf("Error importing hex privKey: %s\n", err)
	}
	pub, priv = s2.HexString()
	t.Logf("Imported pub:%s priv:%s\n", pub, priv)
	v, err := s.Verify(message, msgSign)
	if err != nil {
		t.Errorf("Verification error: %s\n", err)
	}
	if !v {
		t.Error("Verification failed!")
	}
	t.Logf("Testing verification... %t\n", v)

	t.Log("Testing compatibility with standard Ethereum signing libraries")
	hardcodedPriv := "fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19"
	hardcodedSignature := "a0d0ebc374d2a4d6357eaca3da2f5f3ff547c3560008206bc234f9032a866ace6279ffb4093fb39c8bbc39021f6a5c36ef0e813c8c94f325a53f4f395a5c82de01"
	var s3 SignKeys
	s3.AddHexKey(hardcodedPriv)
	pub, priv = s3.HexString()
	if priv != hardcodedPriv {
		t.Errorf("PrivKey from %s not match the hardcoded one\nGot %s\nMust have %s\n", hardcodedPriv, priv, hardcodedPriv[2:])
	}
	signature, err := s3.Sign(message)
	t.Logf("Signature: %s\n", signature)
	if signature != hardcodedSignature {
		t.Errorf("Hardcoded signature %s do not match\n", hardcodedSignature)
	}
	pub2, err := DecompressPubKey(pub)
	if err != nil {
		t.Errorf("Failed decompressing key: %s\n", err)
	}
	pubComp, err := CompressPubKey(pub2)
	if err != nil {
		t.Errorf("Failed compressing key: %s\n", err)
	}
	if pub != pubComp {
		t.Errorf("Compression/Decompression of pubkey do not match (%s != %s)", pub, pubComp)
	}
}

func TestEncryption(t *testing.T) {
	t.Log("Testing encryption and decryption")
	var s SignKeys
	s.Generate()
	pub, priv := s.HexString()
	t.Logf("Generated pub:%s priv:%s\n", pub, priv)
	message := "hello"
	t.Logf("Message to encrypt: %s\n", message)
	msgEnc, err := s.Encrypt(message)
	if err != nil {
		t.Errorf("Error while encrypting %s\n", err)
	}
	t.Logf("Encrypted hexString is %s\n", msgEnc)
	t.Logf("Testing Decryption")
	msgPlain, err := s.Decrypt(msgEnc)
	if err != nil {
		t.Errorf("Error while decrypting %s\n", err)
	}
	t.Logf("Decrypted plain String is %s\n", msgPlain)
}

func TestAddr(t *testing.T) {
	t.Log("Testing Ethereum address compatibility")
	var s SignKeys
	s.Generate()
	pub, priv := s.HexString()
	t.Logf("Generated pub: %s \npriv: %s\n", pub, priv)
	addr1 := s.EthAddrString()
	addr2, err := AddrFromPublicKey(pub)
	t.Logf("Recovered address from pubKey %s", addr2)
	if err != nil {
		t.Error(err)
	}
	if addr1 != addr2 {
		t.Errorf("Calculated address from pubKey do not match: %s != %s\n", addr1, addr2)
	}
	signature, err := s.Sign("hello vocdoni")
	if err != nil {
		t.Error(err)
	}
	t.Logf("Signature created: %s\n", signature)
	addr3, err := AddrFromSignature("hello vocdoni", signature)
	if err != nil {
		t.Error(err)
	}
	// addr3s := fmt.Sprintf("%x", addr3)
	if addr3 != addr2 {
		t.Errorf("Extracted signature address do not match: %s != %s\n", addr2, addr3)
	}

	err = s.AddAuthKey(addr3)
	if err != nil {
		t.Error(err)
	}
	v, _, err := s.VerifySender("hello vocdoni", signature)
	if err != nil {
		t.Error(err)
	}
	if !v {
		t.Error("Cannot verify sender")
	}

	v, err = s.Verify("hello vocdoni", signature)
	if err != nil {
		t.Error(err)
	}
	if !v {
		t.Error("Cannot verify signature")
	}

	signature2, err := s.Sign("bye-bye vocdoni")
	addr4, err := AddrFromSignature("bye-bye vocdoni", signature2)
	if err != nil {
		t.Error(err)
	}
	if addr4 != addr3 {
		t.Error("extracted address from second message do not match")
	}
	t.Logf("%s == %s", addr3, addr4)
}
