package signature

import (
	"encoding/base64"
	"regexp"
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
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	addr4, err := AddrFromSignature("bye-bye vocdoni", signature2)
	if err != nil {
		t.Error(err)
	}
	if addr4 != addr3 {
		t.Error("extracted address from second message do not match")
	}
	t.Logf("%s == %s", addr3, addr4)
}

func TestPoseidonHashing(t *testing.T) {
	base64Regex := regexp.MustCompile("^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$")

	// mnemonic := "fly cheap color olive setup rigid april forum over grief predict pipe toddler argue give"
	pubKey := "0x045a126cbbd3c66b6d542d40d91085e3f2b5db3bbc8cda0d59615deb08784e4f833e0bb082194790143c3d01cedb4a9663cb8c7bdaaad839cb794dd309213fcf30"
	expectedHash := "nGOYvS4aqqUVAT9YjWcUzA89DlHPWaooNpBTStOaHRA="

	hash := HashPoseidon(pubKey)
	base64Hash := base64.StdEncoding.EncodeToString(hash)
	if len(hash) == 0 {
		t.Errorf("Error computing the public key")
	} else if !base64Regex.MatchString(base64Hash) {
		t.Errorf("Invalid base64 string: %s", base64Hash)
	} else if base64Hash != expectedHash {
		t.Errorf("'%s' should be '%s'", base64Hash, expectedHash)
	}

	// 2
	// mnemonic = "kangaroo improve enroll almost since stock travel grace improve welcome orbit decorate govern hospital select"
	pubKey = "0x049969c7741ade2e9f89f81d12080651038838e8089682158f3d892e57609b64e2137463c816e4d52f6688d490c35a0b8e524ac6d9722eed2616dbcaf676fc2578"
	expectedHash = "j7jJlnBN73ORKWbNbVCHG9WkoqSr+IEKDwjcsb6N4xw="
	hash = HashPoseidon(pubKey)
	base64Hash = base64.StdEncoding.EncodeToString(hash)
	if len(hash) == 0 {
		t.Errorf("Error computing the public key")
	} else if !base64Regex.MatchString(base64Hash) {
		t.Errorf("Invalid base64 string: %s", base64Hash)
	} else if base64Hash != expectedHash {
		t.Errorf("'%s' should be '%s'", base64Hash, expectedHash)
	}

	// test hash with less than 32 bytes
	pubKey = "0x0409d240a33ca9c486c090135f06c5d801aceec6eaed94b8bef1c9763b6c39708819207786fe92b22c6661957e83923e24a5ba754755b181f82fdaed2ed3914453"
	expectedHash = "3/AaoqHPrz20tfLmhLz4ay5nrlKN5WiuvlDZkfZyfgA="

	hash = HashPoseidon(pubKey)
	base64Hash = base64.StdEncoding.EncodeToString(hash)
	if len(hash) == 0 {
		t.Errorf("Error computing the public key")
	} else if !base64Regex.MatchString(base64Hash) {
		t.Errorf("Invalid base64 string: %s", base64Hash)
	} else if base64Hash != expectedHash {
		t.Errorf("'%s' should be '%s'", base64Hash, expectedHash)
	}

}
