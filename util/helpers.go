package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

func TrimHex(s string) string {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		return s[2:]
	}
	return s
}

func RandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func Random32() [32]byte {
	var bytes [32]byte
	copy(bytes[:], RandomBytes(32))
	return bytes
}

func RandomHex(n int) string {
	return fmt.Sprintf("%x", RandomBytes(n))
}

func RandomInt(min, max int) int {
	num, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		panic(err)
	}
	return int(num.Int64()) + min
}
