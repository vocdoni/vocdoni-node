package testutil

import (
	"encoding/base64"
	"encoding/hex"
	"io/ioutil"
	"os"
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

func TmpDir(tb testing.TB) string {
	tmp, err := ioutil.TempDir("", "vocdoni-test")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := os.RemoveAll(tmp); err != nil {
			tb.Error(err)
		}
	})
	return tmp
}
