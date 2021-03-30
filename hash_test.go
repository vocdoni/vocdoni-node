package arbo

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashSha256(t *testing.T) {
	// Sha256 hash
	hashFunc := &HashSha256{}
	b := []byte("test")
	h, err := hashFunc.Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t,
		"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		hex.EncodeToString(h))
}

func TestHashPoseidon(t *testing.T) {
	// Poseidon hash
	hashFunc := &HashPoseidon{}
	h, err := hashFunc.Hash(
		BigIntToBytes(big.NewInt(1)),
		BigIntToBytes(big.NewInt(2)))
	if err != nil {
		t.Fatal(err)
	}
	hBI := BytesToBigInt(h)
	// value checked with circomlib
	assert.Equal(t,
		"7853200120776062878684798364095072458815029376092732009249414926327459813530",
		hBI.String())
}
