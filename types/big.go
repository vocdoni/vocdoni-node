package types

import (
	"math/big"
)

// BigInt is a big.Int wrapper which marshals JSON to a string representation of the big number.
// Note that a nil pointer value marshals as the empty string.
type BigInt big.Int

func (i *BigInt) MarshalText() ([]byte, error) {
	return (*big.Int)(i).MarshalText()
}

func (i *BigInt) UnmarshalText(data []byte) error {
	if err := (*big.Int)(i).UnmarshalText(data); err != nil {
		panic(err)
	}
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

// Bytes returns the bytes representation of the big number
func (i *BigInt) Bytes() []byte {
	return (*big.Int)(i).Bytes()
}

// MathBigInt converts b to a math/big *Int.
func (i *BigInt) MathBigInt() *big.Int {
	return (*big.Int)(i)
}

// Add sum x+y
func (i *BigInt) Add(x, y *BigInt) *BigInt {
	return (*BigInt)(i.MathBigInt().Add(x.MathBigInt(), y.MathBigInt()))
}

// Sub subs x-y
func (i *BigInt) Sub(x, y *BigInt) *BigInt {
	return (*BigInt)(i.MathBigInt().Sub(x.MathBigInt(), y.MathBigInt()))
}

// Mul multiplies x*y
func (i *BigInt) Mul(x, y *BigInt) *BigInt {
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
