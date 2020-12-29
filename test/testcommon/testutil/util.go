package testutil

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"go.vocdoni.io/dvote/util"
)

func Hex2byte(tb testing.TB, s string) []byte {
	b, err := hex.DecodeString(util.TrimHex(s))
	if err != nil {
		if tb == nil {
			panic(err)
		}
		tb.Fatal(err)
	}
	return b
}

func B642byte(tb testing.TB, s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		if tb == nil {
			panic(err)
		}
		tb.Fatal(err)
	}
	return b
}
