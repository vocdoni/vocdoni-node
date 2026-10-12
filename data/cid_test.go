package data

import (
	"bytes"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipfs/go-cid"
)

// testData returns n deterministic bytes.
func testData(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + i/251)
	}
	return b
}

// The expected values were produced by the IPFS (boxo) UnixFS importer with the
// fixed-size 262144 chunker, balanced layout and CIDv1 dag-pb sha2-256.
func TestCalculateCIDv1json(t *testing.T) {
	c := qt.New(t)
	for _, tc := range []struct {
		name string
		data []byte
		cid  string
	}{
		{"empty", []byte{}, "bafybeif7ztnhq65lumvvtr4ekcwd2ifwgm3awq4zfr3srh462rwyinlb4y"},
		{"nil", nil, "bafybeif7ztnhq65lumvvtr4ekcwd2ifwgm3awq4zfr3srh462rwyinlb4y"},
		{"json", []byte("{\"test\": { \"hello\": \"world\" }}"), "bafybeiatc7haowqj7aq4lnnz4wnci6mhznuoykrme5mfwa3fvypxwv4gfu"},
		{"one byte", []byte("a"), "bafybeih22o2lqjyoumhqtqjwjomq3mzvdmxxeaivw52aoh2mytrlujo7yi"},
		{"one chunk", testData(262144), "bafybeianb6mxwhdgemzb2hn3x3gzcjtlqbod4e7bkeunuykxcxybrhzov4"},
		{"chunk plus one", testData(262145), "bafybeih4pbmlrvzwajb4evjnqzfkqqjuoltqchubvv4weznb5f2ia6z6hi"},
		{"1MiB", testData(1 << 20), "bafybeighqjzfurwziatqb7cvqrphn72lnvo3jvx2ngyjksbe3icn3doyyq"},
		{"5MiB", testData(5 << 20), "bafybeidrphntrldhjwlyzus7lru2c55mz6zrieor7wwhn2kuxxlr4c47im"},
		{"repeated", bytes.Repeat([]byte("x"), 300000), "bafybeibm6flww4w3q476irns43al6kkddk66i5rzuthndtcem2zvqrn4wm"},
	} {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(CalculateCIDv1json(tc.data), qt.Equals, tc.cid)
		})
	}
}

func TestCIDequals(t *testing.T) {
	c := qt.New(t)
	v1 := "bafybeiatc7haowqj7aq4lnnz4wnci6mhznuoykrme5mfwa3fvypxwv4gfu"
	c.Assert(CIDequals(v1, "ipfs://"+v1), qt.IsTrue)
	c.Assert(CIDequals("/ipfs/"+v1, v1), qt.IsTrue)
	c.Assert(CIDequals(v1, "bafybeih22o2lqjyoumhqtqjwjomq3mzvdmxxeaivw52aoh2mytrlujo7yi"), qt.IsFalse)
	c.Assert(CIDequals(v1, "not a cid"), qt.IsFalse)

	// CIDv0 of the same content: same multihash, base58 and implicit dag-pb codec
	decoded, err := cid.Decode(v1)
	c.Assert(err, qt.IsNil)
	v0 := cid.NewCidV0(decoded.Hash()).String()
	c.Assert(v0[:2], qt.Equals, "Qm")
	c.Assert(CIDequals(v0, v1), qt.IsTrue)
}
