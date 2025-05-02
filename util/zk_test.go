package util

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBytesToArboStr(t *testing.T) {
	c := qt.New(t)

	input := new(big.Int).SetInt64(1233)
	encoded := BytesToArboSplitStr(input.Bytes())
	expected := []string{
		"145749485520268040037154566173721592631",
		"243838562910071029186006881148627719363",
	}
	c.Assert(encoded, qt.DeepEquals, expected)
}

func TestConversionConsistency(t *testing.T) {
	originalInput := RandomBytes(20)
	// Convert to strings
	strParts := BytesToArboSplitStr(originalInput)
	// Convert strings back to bytes
	reconstructedBytes := SplittedArboStrToBytes(strParts[0], strParts[1], true, true)
	// Hash the original input to compare with reconstructed bytes
	expectedHash := sha256.Sum256(originalInput)
	// Check if reconstructed bytes match the hash of the original input
	qt.Assert(t, bytes.Equal(reconstructedBytes, expectedHash[:]), qt.IsTrue)
}
