package signature

import "testing"

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
	v, err := s.Verify(message, msgSign, pub)
	if err != nil {
		t.Errorf("Verification error: %s\n", err)
	}
	if !v {
		t.Error("Verification failed!")
	}
	t.Logf("Testing verification... %t\n", v)

	t.Log("Testing compatibility with standard Ethereum signing libraries")
	hardcodedPriv := "0xfad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19"
	hardcodedSignature := "0xa0d0ebc374d2a4d6357eaca3da2f5f3ff547c3560008206bc234f9032a866ace6279ffb4093fb39c8bbc39021f6a5c36ef0e813c8c94f325a53f4f395a5c82de01"
	var s3 SignKeys
	s3.AddHexKey(hardcodedPriv)
	pub, priv = s3.HexString()
	if priv != hardcodedPriv[2:] {
		t.Errorf("PrivKey from %s not match the hardcoded one\nGot %s\nMust have %s\n", hardcodedPriv, priv, hardcodedPriv[2:])
	}
	signature, err := s3.Sign(message)
	t.Logf("Signature: %s\n", signature)
	if signature != hardcodedSignature {
		t.Errorf("Hardcoded signature %s do not match\n", hardcodedSignature)
	}
}
