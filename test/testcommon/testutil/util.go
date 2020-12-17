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

func Hex2byte32(tb testing.TB, s string) [32]byte {
	b, err := hex.DecodeString(util.TrimHex(s))
	if err != nil {
		if tb == nil {
			panic(err)
		}
		tb.Fatal(err)
	}
	var b32 [32]byte
	copy(b32[:], b)
	return b32
}

func Hex2byte32ptr(tb testing.TB, s string) *[32]byte {
	b := Hex2byte32(tb, s)
	return &b
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
