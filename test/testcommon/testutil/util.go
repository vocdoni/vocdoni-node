package testutil

import (
	"encoding/base64"
	"encoding/hex"
	"math/rand"
	"testing"

	iden3cryptoutils "github.com/iden3/go-iden3-crypto/utils"
	"github.com/vocdoni/arbo"

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
	rand *rand.Rand
}

func NewRandom(seed int64) Random {
	return Random{
		rand: rand.New(rand.NewSource(seed)),
	}
}

func (r *Random) RandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := r.rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// RandomInSnarkField returns a random litte-endian encoded element in the
// ZK SNARK field.
func (r *Random) RandomInZKField() []byte {
	b := make([]byte, 32)
	for {
		_, err := r.rand.Read(b)
		if err != nil {
			panic(err)
		}
		b[31] &= 0b00111111
		if iden3cryptoutils.CheckBigIntInField(arbo.BytesToBigInt(b)) {
			return b
		}
	}
}
