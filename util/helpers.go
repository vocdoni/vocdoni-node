package util

import (
	"fmt"
	"io"
	"math/rand"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
)

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

func Random32() [32]byte {
	var bytes [32]byte
	if _, err := io.ReadFull(randReader, bytes[:]); err != nil {
		panic(err)
	}
	return bytes
}

func RandomHex(n int) string {
	return fmt.Sprintf("%x", RandomBytes(n))
}

func RandomInt(min, max int) int {
	return randReader.Intn(max-min) + min
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

// CreateEthRandomKeysBatch creates a set of eth random signing keys
func CreateEthRandomKeysBatch(n int) []*ethereum.SignKeys {
	s := make([]*ethereum.SignKeys, n)
	for i := 0; i < n; i++ {
		s[i] = ethereum.NewSignKeys()
		if err := s[i].Generate(); err != nil {
			panic(err)
		}
	}
	return s
}
