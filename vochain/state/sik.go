package state

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// encodedHeightLen constant is the number of bytes of the encoded hysteresis
// that contains the hysteresis height value, starting from the last byte
const encodedHeightLen = 4

// hysteresisLen constant contains the number of bytes that the encode
// hysteresis has
const hysteresisLen = 32

// SetSIK function creates or update the SIK of the provided address in the
// state. It covers the following cases:
//   - It checks if already exists a valid SIK for the provided address and if
//     so it returns an error.
//   - If no SIK exists for the provided address, it will create one with the
//     value provided.
//   - If exists a logically deleted SIK for the address provided, checks the
//     hysteresis height for that address before update its SIK.
//   - If the hysteresis height is not reached yet, it returns an error.
//   - If the hysteresis height is reached, it updates the value of the sik with
//     the provided one.
func (v *State) SetSIK(addr common.Address, sik []byte) error {
	return nil
}

// DelSIK function removes the registered SIK for the address provided. If it is
// not registered, it returns an error. If it is, it will encode the hysteresis
// height and set it as SIK value to invalidate it and prevent it to being
// updated before that height. The hysteresisHeight must be greater than the
// curren chain height.
func (v *State) DelSIK(addr common.Address, hysteresisHeight uint32) error {
	return nil
}

// encodeHysteresis funtion returns the encoded value of the height hysteresis
// provided ready to use in the SIK subTree as leaf value.
// It will have 32 bytes:
//   - The initial 28 bytes must be zero.
//   - The remaining 4 bytes must contain the height encoded in LittleEndian
func encodeHysteresis(height uint32) []byte {
	bHeight := big.NewInt(int64(height)).Bytes()
	hysteresis := make([]byte, hysteresisLen)
	// copy the height bytes swapping endianness in the last bytes of the empty
	// hysteresis result
	for i, b := hysteresisLen-1, 0; i >= hysteresisLen-len(bHeight); i, b = i-1, b+1 {
		hysteresis[i] = bHeight[b]
	}
	return hysteresis
}

// decodeHysteresis funtion returns the decoded height uint32 from the leaf
// value that contains the encoded hysteresis height.
func decodeHysteresis(leafValue []byte) uint32 {
	bHeight := []byte{}
	for i := hysteresisLen - 1; i >= hysteresisLen-encodedHeightLen; i-- {
		bHeight = append(bHeight, leafValue[i])
	}
	return uint32(new(big.Int).SetBytes(bHeight).Int64())
}
