package state

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
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
func (v *State) SetSIK(address common.Address, newSik []byte) error {
	// check if exists a registered sik for the provided address, query also for
	// no commited tree version
	sik, err := v.mainTreeViewer(false).DeepGet(address.Bytes(), StateTreeCfg(TreeSIK))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		// if not exists create it
		log.Debugw("setSIK (create)",
			"address", address.String(),
			"sik", hex.EncodeToString(sik))
		v.Tx.Lock()
		defer v.Tx.Unlock()
		return v.Tx.DeepAdd(address.Bytes(), newSik, StateTreeCfg(TreeSIK))
	} else if err != nil {
		return err
	}
	// check if is a valid sik
	if validSIK(sik) {
		return ErrRegisteredValidSIK
	}
	// check if the hysteresis height is reached
	hysteresis := decodeHysteresis(sik)
	if hysteresis > v.CurrentHeight() {
		return ErrHysteresisNotReached
	}
	log.Debugw("setSIK (update)",
		"address", address.String(),
		"sik", hex.EncodeToString(newSik))
	// if the hysteresis is reached update the sik for the address
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(address.Bytes(), newSik, StateTreeCfg(TreeSIK))
}

// DelSIK function removes the registered SIK for the address provided. If it is
// not registered, it returns an error. If it is, it will encode the hysteresis
// height and set it as SIK value to invalidate it and prevent it to being
// updated before that height. The hysteresisHeight must be greater than the
// curren chain height.
func (v *State) DelSIK(address common.Address, hysteresisHeight uint32) error {
	// if the sik does not exists or something fails querying return the error
	sik, err := v.mainTreeViewer(false).DeepGet(address.Bytes(), StateTreeCfg(TreeSIK))
	if err != nil {
		return err
	}
	// if the stored sik is already invalidated return an error
	if !validSIK(sik) {
		return ErrSIKAlreadyInvalid
	}
	// if the provided hysteresis height is lower or equal to the current height
	// return an error
	if hysteresisHeight <= v.CurrentHeight() {
		return ErrInvalidHysteresis
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(address.Bytes(), encodeHysteresis(hysteresisHeight), StateTreeCfg(TreeSIK))
}

// encodeHysteresis funtion returns the encoded value of the height hysteresis
// provided ready to use in the SIK subTree as leaf value.
// It will have 32 bytes:
//   - The initial 28 bytes must be zero.
//   - The remaining 4 bytes must contain the height encoded in LittleEndian
func encodeHysteresis(height uint32) []byte {
	bHeight := big.NewInt(int64(height)).Bytes()
	// fill with zeros until reach the encoded height length
	for len(bHeight) < encodedHeightLen {
		bHeight = append([]byte{0}, bHeight...)
	}
	// create the hysteresis with the rigth number of zeros
	hysteresis := make([]byte, hysteresisLen-encodedHeightLen)
	// copy the height bytes swapping endianness in the last bytes
	for i := encodedHeightLen - 1; i >= 0; i-- {
		hysteresis = append(hysteresis, bHeight[i])
	}
	return hysteresis
}

// decodeHysteresis funtion returns the decoded height uint32 from the leaf
// value that contains the encoded hysteresis height.
func decodeHysteresis(leafValue []byte) uint32 {
	bHeight := []byte{}
	for i := hysteresisLen - 1; len(bHeight) < encodedHeightLen; i-- {
		bHeight = append(bHeight, leafValue[i])
	}
	return uint32(new(big.Int).SetBytes(bHeight).Int64())
}

// validSIK function returns if the provided SIK is a valid one or invalid.
func validSIK(sik []byte) bool {
	for i := 0; i < len(sik)-encodedHeightLen; i++ {
		if sik[i] != 0 {
			return true
		}
	}
	return false
}
