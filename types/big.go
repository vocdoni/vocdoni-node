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
		return fmt.Errorf("wrong format for bigInt: %q", string(data))
	}
	*i = (BigInt)(*i2)
	return nil
}

func (i *BigInt) GobEncode() ([]byte, error) {
	return i.MathBigInt().GobEncode()
}

func (i *BigInt) GobDecode(buf []byte) error {
	return i.MathBigInt().GobDecode(buf)
}

// String returns the string representation of the big number
func (i *BigInt) String() string {
	return (*big.Int)(i).String()
}

// SetBytes interprets buf as big-endian unsigned integer
func (i *BigInt) SetBytes(buf []byte) *BigInt {
	return (*BigInt)(i.MathBigInt().SetBytes(buf))
}

// SetString interprets the string as a big number
func (i *BigInt) SetString(s string) (*BigInt, error) {
	bi, ok := i.MathBigInt().SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("cannot set string %s", s)
	}
	return (*BigInt)(bi), nil
}

// Bytes returns the bytes representation of the big number
func (i *BigInt) Bytes() []byte {
	return (*big.Int)(i).Bytes()
}

// MathBigInt converts b to a math/big *Int.
func (i *BigInt) MathBigInt() *big.Int {
	return (*big.Int)(i)
}

// Add sum x+y
func (i *BigInt) Add(x *BigInt, y *BigInt) *BigInt {
	return (*BigInt)(i.MathBigInt().Add(x.MathBigInt(), y.MathBigInt()))
}

// Sub subs x-y
func (i *BigInt) Sub(x *BigInt, y *BigInt) *BigInt {
	return (*BigInt)(i.MathBigInt().Sub(x.MathBigInt(), y.MathBigInt()))
}

// Mul multiplies x*y
func (i *BigInt) Mul(x *BigInt, y *BigInt) *BigInt {
	return (*BigInt)(i.MathBigInt().Mul(x.MathBigInt(), y.MathBigInt()))
}

// SetUint64 sets the value of x to the big number
func (i *BigInt) SetUint64(x uint64) *BigInt {
	return (*BigInt)(i.MathBigInt().SetUint64(x))
}

// Equal helps us with go-cmp.
func (i *BigInt) Equal(j *BigInt) bool {
	if i == nil || j == nil {
		return (i == nil) == (j == nil)
	}
	return i.MathBigInt().Cmp(j.MathBigInt()) == 0
}
