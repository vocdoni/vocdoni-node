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
	return i.ToInt().GobEncode()
}

func (i *BigInt) GobDecode(buf []byte) error {
	return i.ToInt().GobDecode(buf)
}

// String returns the string representation of the big number
func (i *BigInt) String() string {
	return (*big.Int)(i).String()
}

// SetBytes interprets buf as big-endian unsigned integer
func (i *BigInt) SetBytes(buf []byte) *BigInt {
	return (*BigInt)(i.ToInt().SetBytes(buf))
}

// Bytes returns the bytes representation of the big number
func (i *BigInt) Bytes() []byte {
	return (*big.Int)(i).Bytes()
}

// ToInt converts b to a big.Int.
func (i *BigInt) ToInt() *big.Int {
	return (*big.Int)(i)
}

// Add sum x+y
func (i *BigInt) Add(x *BigInt, y *BigInt) *BigInt {
	return (*BigInt)(i.ToInt().Add(x.ToInt(), y.ToInt()))
}

// Mul multiplies x*y
func (i *BigInt) Mul(x *BigInt, y *BigInt) *BigInt {
	return (*BigInt)(i.ToInt().Mul(x.ToInt(), y.ToInt()))
}

// SetUint64 sets the value of x to the big number
func (i *BigInt) SetUint64(x uint64) *BigInt {
	return (*BigInt)(i.ToInt().SetUint64(x))
}

// Equal helps us with go-cmp.
func (i *BigInt) Equal(j *BigInt) bool {
	return i.ToInt().Cmp(j.ToInt()) == 0
}
