package state

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
)

// TODO: Move the definition to the right place
const SIKROOT_HYSTERESIS_BLOCKS = 32

// encodedHeightLen constant is the number of bytes of the encoded hysteresis
// that contains the hysteresis height value, starting from the last byte
const encodedHeightLen = 4

// sikLeafValueLen constant contains the number of bytes that a leaf value has
const sikLeafValueLen = 32

// SetSIK function creates or update the SIK of the provided address in the
// state. It covers the following cases:
//   - It checks if already exists a valid SIK for the provided address and if
//     so it returns an error.
//   - If no SIK exists for the provided address, it will create one with the
//     value provided.
//   - If exists a logically deleted SIK for the address provided, checks the
//     encoded height for that address before update its SIK.
//   - If the encoded height is greater than or equal to the update threshold,
//     it returns an error.
//   - If the encoded height is lower than the update threshold, it updates the
//     value of the sik with the provided one.
//   - The update threshold is equal to the minimun start block of on-going
//     elections plus the sik root hysteresis block.
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
	}
	if err != nil {
		return err
	}
	// check if is a valid sik
	if validSIK(sik) {
		return ErrRegisteredValidSIK
	}
	// if the sik has been deleted, its leaf stores the height when it was
	// deleted. To avoid double voting (changing the sik), the height stored
	// must be lower than the minimun on-going elections start block plus the
	// hysteresis number of blocks.
	minOnGoingStartBlock, err := v.MinOnGoingStartBlock(false)
	if err != nil {
		return err
	}
	deletedHeight := decodeHeight(sik)
	if deletedHeight < minOnGoingStartBlock+SIKROOT_HYSTERESIS_BLOCKS {
		return ErrSIKNotUpdateable
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
// not registered, it returns an error. If it is, it will encode the current
// height and set it as the SIK value to invalidate it and prevent it from being
// updated until all processes created before that height have finished.
func (v *State) DelSIK(address common.Address) error {
	// if the sik does not exists or something fails querying return the error
	sik, err := v.mainTreeViewer(false).DeepGet(address.Bytes(), StateTreeCfg(TreeSIK))
	if err != nil {
		return err
	}
	// if the stored sik is already invalidated return an error
	if !validSIK(sik) {
		return ErrSIKAlreadyInvalid
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(address.Bytes(), encodeHeight(v.CurrentHeight()), StateTreeCfg(TreeSIK))
}

// UpdateSIKRoots keep on track the last valid SIK Merkle Tree roots to support
// voting to already registered users when an election is on going and new users
// are registered. When a new sikRoot is generated, the sikRoot’s from an older
// block than the current block minus the hysteresis blocks will be deleted:
//   - If exists a sikRoot for the minimun hysteresis block number
//     (currentBlock - hysteresis), just remove all the roots with a lower block
//     number.
//   - If it does not exist, remove all roots with a lower block number except
//     for the next lower sikRoot. It is becouse it still being validate for a
//     period.
func (v *State) UpdateSIKRoots() error {
	// get sik roots key-value database associated to the siks tree
	v.Tx.Lock()
	defer v.Tx.Unlock()
	siksTree, err := v.Tx.DeepSubTree(StateTreeCfg(TreeProcess))
	if err != nil {
		return err
	}
	sikRoots := siksTree.NoState()
	// calculate the current minimun block to purge useless sik roots
	minBlock := v.CurrentHeight() - SIKROOT_HYSTERESIS_BLOCKS
	if minBlock != 0 {
		toPurge := [][]byte{}
		minBlockKey := make([]byte, 32)
		binary.LittleEndian.PutUint32(minBlockKey, minBlock)
		if _, err := sikRoots.Get(minBlockKey); err == nil {
			// if exists a sikRoot for the minimun block number just remove all
			// the roots with a lower block number
			sikRoots.Iterate([]byte{}, func(key, value []byte) bool {
				if binary.LittleEndian.Uint32(key) < minBlock {
					toPurge = append(toPurge, key)
				}
				return true
			})
		} else {
			// If not, remove all roots with a lower block number except for
			// the next lower sikRoot
			nearestLowerBlock := uint32(0)
			// iterate once to get the nearest lower block to the calculated
			// min block
			sikRoots.Iterate([]byte{}, func(key, value []byte) bool {
				blockNumber := binary.LittleEndian.Uint32(key)
				if nearestLowerBlock < blockNumber && blockNumber < minBlock {
					nearestLowerBlock = blockNumber
				}
				return true
			})
			// iterate again to get the sikRoots block numbers to delete
			sikRoots.Iterate([]byte{}, func(key, value []byte) bool {
				if binary.LittleEndian.Uint32(key) < nearestLowerBlock {
					toPurge = append(toPurge, key)
				}
				return true
			})
		}
		// delete the selected sikRoots by its block numbers
		for _, blockNumber := range toPurge {
			if err := sikRoots.Delete(blockNumber); err != nil {
				return err
			}
		}
	}
	// add the new sikRoot
	hash, err := siksTree.Root()
	if err != nil {
		return err
	}
	blockKey := make([]byte, 32)
	binary.LittleEndian.PutUint32(blockKey, v.CurrentHeight())
	return sikRoots.Set(blockKey, hash)
}

// encodeHeight funtion returns the encoded value of the encoded height
// provided ready to use in the SIK subTree as leaf value.
// It will have 32 bytes:
//   - The initial 28 bytes must be zero.
//   - The remaining 4 bytes must contain the height encoded in LittleEndian
func encodeHeight(height uint32) []byte {
	bHeight := big.NewInt(int64(height)).Bytes()
	// fill with zeros until reach the encoded height length
	for len(bHeight) < encodedHeightLen {
		bHeight = append([]byte{0}, bHeight...)
	}
	// create the encodedHeight with the right number of zeros
	encodedHeight := make([]byte, sikLeafValueLen-encodedHeightLen)
	// copy the height bytes swapping endianness in the last bytes
	for i := encodedHeightLen - 1; i >= 0; i-- {
		encodedHeight = append(encodedHeight, bHeight[i])
	}
	return encodedHeight
}

// decodeHeight funtion returns the decoded height uint32 from the leaf value
// that contains the encoded height.
func decodeHeight(leafValue []byte) uint32 {
	bHeight := []byte{}
	for i := sikLeafValueLen - 1; len(bHeight) < encodedHeightLen; i-- {
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
