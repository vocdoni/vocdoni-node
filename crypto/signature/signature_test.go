package signature

import "testing"

func TestSignature(t *testing.T) {
	t.Log("Testing signature creation and verification")
	var s SignKeys
	s.Generate()
	pub, priv := s.HexString()
	t.Logf("Generated pub:%s priv:%s\n", pub, priv)
	message := "Hello, this is gonna be signed!"
	t.Logf("Message to sign: %s\n", message)
	msgSign, err := s.Sign(message)
	if err != nil {
		t.Errorf("Error while signing %s\n", err)
	}
	t.Logf("Signature of message: %s\n", msgSign)
	v, err := s.Verify(message, msgSign, pub)
	if err != nil {
		t.Errorf("Verification error: %s\n", err)
	}
	if !v {
		t.Error("Verification failed!")
	}
	t.Logf("Testing verification... %t\n", v)
}
