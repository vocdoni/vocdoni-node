package types

import (
	"encoding/json"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBigInt(t *testing.T) {
	// Test basic
	a := new(BigInt).SetUint64(100)
	b := new(BigInt).SetUint64(200)

	a.Add(a, b)
	qt.Assert(t, a.String(), qt.DeepEquals, "300")

	a.ToStdBigInt().Add(a.ToStdBigInt(), b.ToStdBigInt())
	qt.Assert(t, a.String(), qt.DeepEquals, "500")

	// Test single text Marshaling
	j, err := a.MarshalText()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, string(j), qt.DeepEquals, "500")

	c := new(BigInt)
	err = c.UnmarshalText([]byte("123"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, c.String(), qt.DeepEquals, "123")

	// Test text Marshaling inside a struct
	js := &jsonStructTest{
		Name:   "first",
		BigInt: new(BigInt).SetUint64(12312312312312312312),
	}
	data, err := json.Marshal(js)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, string(data), qt.DeepEquals,
		`{"name":"first","number":"12312312312312312312"}`)

	// Test bytes compatibility with standard library
	d := new(big.Int).SetInt64(456).Bytes()
	c.SetBytes(d)
	qt.Assert(t, c.String(), qt.DeepEquals, "456")
}

type jsonStructTest struct {
	Name   string  `json:"name"`
	BigInt *BigInt `json:"number"`
}
