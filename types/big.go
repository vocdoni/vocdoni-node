package types

import (
	"fmt"
	"math/big"
)

// BigInt is a big.Int wrapper which marshals JSON to a string representation of the big number
type BigInt big.Int

func (i BigInt) MarshalText() ([]byte, error) {
	// TODO(mvdan): we shouldn't be using String/SetString for encoding
	return []byte((*big.Int)(&i).String()), nil
}

func (i *BigInt) UnmarshalText(data []byte) error {
	i2, ok := new(big.Int).SetString(string(data), 0)
	if !ok {
		return fmt.Errorf("wrong format for bigInt")
	}
	*i = (BigInt)(*i2)
	return nil
}

func (i *BigInt) GobEncode() ([]byte, error) {
	return i.ToStdBigInt().GobEncode()
}

func (i *BigInt) GobDecode(buf []byte) error {
	return i.ToStdBigInt().GobDecode(buf)
}

// String returns the string representation of the big number
func (i *BigInt) String() string {
	return (*big.Int)(i).String()
}

// SetBytes interprets buf as big-endian unsigned integer
func (i *BigInt) SetBytes(buf []byte) *BigInt {
	return (*BigInt)(i.ToStdBigInt().SetBytes(buf))
}

// SetString interprets the string as a big number
func (i *BigInt) SetString(s string) (*BigInt, error) {
	bi, ok := i.ToStdBigInt().SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("cannot set string %s", s)
	}
	return (*BigInt)(bi), nil
}

// Bytes returns the bytes representation of the big number
func (i *BigInt) Bytes() []byte {
	return (*big.Int)(i).Bytes()
}

// ToStdBigInt converts b to a big.Int.
func (i *BigInt) ToStdBigInt() *big.Int {
	return (*big.Int)(i)
}

// Add sum x+y
func (i *BigInt) Add(x *BigInt, y *BigInt) *BigInt {
	return (*BigInt)(i.ToStdBigInt().Add(x.ToStdBigInt(), y.ToStdBigInt()))
}

// Mul multiplies x*y
func (i *BigInt) Mul(x *BigInt, y *BigInt) *BigInt {
	return (*BigInt)(i.ToStdBigInt().Mul(x.ToStdBigInt(), y.ToStdBigInt()))
}

// SetUint64 sets the value of x to the big number
func (i *BigInt) SetUint64(x uint64) *BigInt {
	return (*BigInt)(i.ToStdBigInt().SetUint64(x))
}

// Equal helps us with go-cmp.
func (i *BigInt) Equal(j *BigInt) bool {
	if i == nil || j == nil {
		return (i == nil) == (j == nil)
	}
	return i.ToStdBigInt().Cmp(j.ToStdBigInt()) == 0
}
