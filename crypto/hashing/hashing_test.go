package hashing

import (
	"regexp"
	"testing"
)

func TestPoseidonHashing(t *testing.T) {
	base64Regex, _ := regexp.Compile("^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$")

	// mnemonic := "fly cheap color olive setup rigid april forum over grief predict pipe toddler argue give"
	pubKey := "0x045a126cbbd3c66b6d542d40d91085e3f2b5db3bbc8cda0d59615deb08784e4f833e0bb082194790143c3d01cedb4a9663cb8c7bdaaad839cb794dd309213fcf30"
	expectedHash := "A69fOQ6ObtcUgXMdFgMSbQnhk4+F6tnC/NyYokgZrc0="

	base64Hash, err := PoseidonHash(pubKey)
	if err != nil {
		t.Errorf("Error computing the public key %s\n", err)
	} else if !base64Regex.MatchString(base64Hash) {
		t.Errorf("Invalid base64 string: %s", base64Hash)
	} else if base64Hash != expectedHash {
		t.Errorf("'%s' should be '%s'", base64Hash, expectedHash)
	}

	// 2
	// mnemonic = "kangaroo improve enroll almost since stock travel grace improve welcome orbit decorate govern hospital select"
	pubKey = "0x049969c7741ade2e9f89f81d12080651038838e8089682158f3d892e57609b64e2137463c816e4d52f6688d490c35a0b8e524ac6d9722eed2616dbcaf676fc2578"
	expectedHash = "HuboX6y+3LHrhFy1cfQ/u6QAY9PvB6Yg98aELzJH7kU="

	base64Hash, err = PoseidonHash(pubKey)
	if err != nil {
		t.Errorf("Error computing the public key %s\n", err)
	} else if !base64Regex.MatchString(base64Hash) {
		t.Errorf("Invalid base64 string: %s", base64Hash)
	} else if base64Hash != expectedHash {
		t.Errorf("'%s' should be '%s'", base64Hash, expectedHash)
	}

	// 3
	// mnemonic = "soup sunset inhale lend eagle hold reduce churn alpha torch leopard phrase unfold crucial soccer"
	pubKey = "0x049622878da186a8a31f4dc03454dbbc62365060458db174618218b51d5014fa56c8ea772234341ae326ce278091c39e30c02fa1f04792035d79311fe3283f1380"
	expectedHash = "LQ5YLnHQiYagbk3f1YfQbNEM3TZNddKyulHpUpTzCw4="

	base64Hash, err = PoseidonHash(pubKey)
	if err != nil {
		t.Errorf("Error computing the public key %s\n", err)
	} else if !base64Regex.MatchString(base64Hash) {
		t.Errorf("Invalid base64 string: %s", base64Hash)
	} else if base64Hash != expectedHash {
		t.Errorf("'%s' should be '%s'", base64Hash, expectedHash)
	}

	// 4
	// mnemonic = "soul frequent purity regret noble husband weapon scheme cement lamp put regular envelope physical entire"
	pubKey = "0x0420606a7dcf293722f3eddc7dca0e2505c08d5099e3d495091782a107d006a7d64c3034184fb4cd59475e37bf40ca43e5e262be997bb74c45a9a723067505413e"
	expectedHash = "DJRuAQL3tnEqD4m+hvQLX4cXkeSgmrxVLR2bUc6ofCI="

	base64Hash, err = PoseidonHash(pubKey)
	if err != nil {
		t.Errorf("Error computing the public key %s\n", err)
	} else if !base64Regex.MatchString(base64Hash) {
		t.Errorf("Invalid base64 string: %s", base64Hash)
	} else if base64Hash != expectedHash {
		t.Errorf("'%s' should be '%s'", base64Hash, expectedHash)
	}
}
