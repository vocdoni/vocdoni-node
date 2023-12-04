package util

import (
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBytesToArboStr(t *testing.T) {
	c := qt.New(t)

	input := new(big.Int).SetInt64(1233)
	encoded := BytesToArboStr(input.Bytes())
	expected := []string{
		"145749485520268040037154566173721592631",
		"243838562910071029186006881148627719363",
	}
	c.Assert(encoded, qt.DeepEquals, expected)
}
