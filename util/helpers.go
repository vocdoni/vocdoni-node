package util

import (
	"encoding/base64"
	"encoding/hex"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"testing"
	"time"
)

var validHexRegex = regexp.MustCompile("^([0-9a-fA-F])+$")

// IsHex checks if the given string contains only valid hex symbols
func IsHex(str string) bool { return validHexRegex.MatchString(str) }

// IsHexEncodedStringWithLength checks if the given string contains only valid hex symbols and have the desired length
func IsHexEncodedStringWithLength(str string, length int) bool {
	str = TrimHex(str)
	return hex.DecodedLen(len(str)) == length && IsHex(str)
}

func Hex2int64(s string) int64 {
	// base 16 for hexadecimal
	result, err := strconv.ParseUint(TrimHex(s), 16, 64)
	if err != nil {
		panic(err)
	}
	return int64(result)
}

func TrimHex(s string) string {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		return s[2:]
	}
	return s
}

var randReader = rand.New(rand.NewSource(time.Now().UnixNano()))

func RandomBytes(n int) []byte {
	bytes := make([]byte, n)
	if _, err := io.ReadFull(randReader, bytes); err != nil {
		panic(err)
	}
	return bytes
}

func RandomHex(n int) string {
	return hex.EncodeToString(RandomBytes(n))
}

func RandomInt(min, max int) int {
	return randReader.Intn(max-min) + min
}

func Hex2byte(tb testing.TB, s string) []byte {
	b, err := hex.DecodeString(TrimHex(s))
	if err != nil {
		if tb == nil {
			panic(err)
		}
		tb.Fatal(err)
	}
	return b
}

func Hex2byte32(tb testing.TB, s string) [32]byte {
	b, err := hex.DecodeString(TrimHex(s))
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

func SplitBytes(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf[:])
	}
	return chunks
}
