package testutil

import (
	"encoding/base64"
	"encoding/hex"
	"math/rand"
	"sync"
	"testing"

	iden3cryptoutils "github.com/iden3/go-iden3-crypto/utils"
	"go.vocdoni.io/dvote/tree/arbo"

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

type Random struct {
	randMu sync.Mutex
	rand   *rand.Rand
}

func NewRandom(seed int64) Random {
	return Random{
		rand: rand.New(rand.NewSource(seed)),
	}
}

func (r *Random) RandomBytes(n int) []byte {
	b := make([]byte, n)
	r.randMu.Lock()
	defer r.randMu.Unlock()
	_, err := r.rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func (r *Random) Random32() [32]byte {
	b := r.RandomBytes(32)
	var p [32]byte
	copy(p[:], b)
	return p
}

func (r *Random) RandomIntn(n int) int {
	r.randMu.Lock()
	defer r.randMu.Unlock()
	return r.rand.Intn(n)
}

// RandomInZKField returns a random litte-endian encoded element in the
// ZK SNARK field.
func (r *Random) RandomInZKField() []byte {
	b := make([]byte, 32)
	r.randMu.Lock()
	defer r.randMu.Unlock()
	for {
		_, err := r.rand.Read(b)
		if err != nil {
			panic(err)
		}
		b[31] &= 0b00111111
		if iden3cryptoutils.CheckBigIntInField(arbo.BytesLEToBigInt(b)) {
			return b
		}
	}
}
