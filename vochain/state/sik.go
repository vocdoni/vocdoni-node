package state

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
)

// TODO: Move the definition to the right place
const SIKROOT_HYSTERESIS_BLOCKS = 32

const (
	// encodedHeightLen constant is the number of bytes of the encoded
	// hysteresis that contains the hysteresis height value, starting from the
	// last byte
	encodedHeightLen = 4
	// sikLeafValueLen constant contains the number of bytes that a leaf value
	// has
	sikLeafValueLen = 32
)

// SIK type abstracts a slice of bytes that contains the Secret Identity Key
// value of a user
type SIK []byte

// SIKFromAddress function return the current SIK value associated to the provided
// address.
func (v *State) SIKFromAddress(address common.Address) (SIK, error) {
	return v.mainTreeViewer(false).DeepGet(address.Bytes(), StateTreeCfg(TreeSIK))
}

// SetAddressSIK function creates or update the SIK of the provided address in the
// state. It covers the following cases:
//   - It checks if already exists a valid SIK for the provided address and if
//     so it returns an error.
//   - If no SIK exists for the provided address, it will create one with the
//     value provided.
//   - If it exists but it is not valid, overwrite the stored value with the
//     provided one.
func (v *State) SetAddressSIK(address common.Address, newSik SIK) error {
	siksTree, err := v.Tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return err
	}
	// check if exists a registered sik for the provided address, query also for
	// no commited tree version
	v.Tx.Lock()
	rawSik, err := siksTree.Get(address.Bytes())
	v.Tx.Unlock()
	if errors.Is(err, arbo.ErrKeyNotFound) {
		// if not exists create it
		log.Debugw("setSIK (create)",
			"address", address.String(),
			"sik", newSik.String())
		v.Tx.Lock()
		err = siksTree.Add(address.Bytes(), newSik)
		v.Tx.Unlock()
		if err != nil {
			return err
		}
		return v.UpdateSIKRoots()
	}
	if err != nil {
		return err
	}
	// check if is a valid sik
	if SIK(rawSik).Valid() {
		return ErrRegisteredValidSIK
	}
	log.Debugw("setSIK (update)",
		"address", address.String(),
		"sik", SIK(rawSik).String())
	// if the hysteresis is reached update the sik for the address
	v.Tx.Lock()
	err = siksTree.Set(address.Bytes(), newSik)
	v.Tx.Unlock()
	if err != nil {
		return err
	}
	return v.UpdateSIKRoots()
}

// InvalidateSIK function removes logically the registered SIK for the address
// provided. If it is not registered, it returns an error. If it is, it will
// encode the current height and set it as the SIK value to invalidate it and
// prevent it from being updated until all processes created before that height
// have finished.
func (v *State) InvalidateSIK(address common.Address) error {
	// if the sik does not exists or something fails querying return the error
	rawSik, err := v.mainTreeViewer(false).DeepGet(address.Bytes(), StateTreeCfg(TreeSIK))
	if err != nil {
		return err
	}
	// if the stored sik is already invalidated return an error
	if !SIK(rawSik).Valid() {
		return ErrSIKAlreadyInvalid
	}
	v.Tx.Lock()
	invalidatedSIK := make(SIK, sikLeafValueLen).InvalidateAt(v.CurrentHeight())
	err = v.Tx.DeepSet(address.Bytes(), invalidatedSIK, StateTreeCfg(TreeSIK))
	v.Tx.Unlock()
	if err != nil {
		return err
	}
	return v.UpdateSIKRoots()
}

// ValidSIKRoots returns the list of current valid roots from the SIK's merkle
// tree. It reads the roots from the key-value database associated to the SIK's
// subtree.
func (v *State) ValidSIKRoots() ([][]byte, error) {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	siksTree, err := v.Tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return nil, err
	}
	validRoots := [][]byte{}
	siksTree.NoState().Iterate(nil, func(_, root []byte) bool {
		validRoots = append(validRoots, root)
		return true
	})
	return validRoots, nil
}

// ExpiredSik returns if the provided siksRoot is still valid or not, checking
// if it is included into the list of current valid sik roots.
func (v *State) ExpiredSik(candidateRoot []byte) (bool, error) {
	validRoots, err := v.ValidSIKRoots()
	if err != nil {
		return false, err
	}
	for _, sikRoot := range validRoots {
		if bytes.Equal(sikRoot, candidateRoot) {
			return false, nil
		}
	}
	return true, nil
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
	siksTree, err := v.Tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return err
	}
	newSikRoot, err := siksTree.Root()
	if err != nil {
		return err
	}
	sikRootsDB := siksTree.NoState()
	// calculate the current minimun block to purge useless sik roots
	currentBlock := v.CurrentHeight()
	if currentBlock > SIKROOT_HYSTERESIS_BLOCKS {
		toPurge := [][]byte{}
		minBlock := currentBlock - SIKROOT_HYSTERESIS_BLOCKS
		minBlockKey := make([]byte, 32)
		binary.LittleEndian.PutUint32(minBlockKey, minBlock)
		if _, err := sikRootsDB.Get(minBlockKey); err == nil {
			// if exists a sikRoot for the minimun block number just remove all
			// the roots with a lower block number
			sikRootsDB.Iterate(nil, func(key, value []byte) bool {
				if binary.LittleEndian.Uint32(key) < minBlock {
					toPurge = append(toPurge, bytes.Clone(key))
				}
				return true
			})
		} else {
			// If not, remove all roots with a lower block number except for
			// the next lower sikRoot
			nearestLowerBlock := minBlock
			// iterate once to get the nearest lower block to the calculated
			// min block
			sikRootsDB.Iterate(nil, func(key, value []byte) bool {
				candidateKey := bytes.Clone(key)
				blockNumber := binary.LittleEndian.Uint32(candidateKey)
				if blockNumber < minBlock && blockNumber > nearestLowerBlock {
					nearestLowerBlock = blockNumber
				}
				return true
			})
			// iterate again to get the sikRoots block numbers to delete
			sikRootsDB.Iterate(nil, func(key, value []byte) bool {
				if binary.LittleEndian.Uint32(key) < nearestLowerBlock {
					toPurge = append(toPurge, bytes.Clone(key))
				}
				return true
			})
		}
		// delete the selected sikRoots by its block numbers
		for _, blockToDelete := range toPurge {
			if err := sikRootsDB.Delete(blockToDelete); err != nil {
				return err
			}
			log.Debugw("updateSIKRoots (deleted)",
				"blockNumber", binary.LittleEndian.Uint32(blockToDelete))
		}
	}
	log.Debugw("updateSIKRoots (created)",
		"newSikRoot", hex.EncodeToString(newSikRoot),
		"blockNumber", currentBlock)
	blockKey := make([]byte, 32)
	binary.LittleEndian.PutUint32(blockKey, currentBlock)
	return sikRootsDB.Set(blockKey, newSikRoot)
}

// InvalidateAt funtion sets the current SIK value to the encoded value of the
// height provided, ready to use in the SIK subTree as leaf value to invalidate
// it. The encoded value will have 32 bytes:
//   - The initial 28 bytes must be zero.
//   - The remaining 4 bytes must contain the height encoded in LittleEndian
func (s SIK) InvalidateAt(height uint32) SIK {
	bHeight := big.NewInt(int64(height)).Bytes()
	// fill with zeros until reach the encoded height length
	for len(bHeight) < encodedHeightLen {
		bHeight = append([]byte{0}, bHeight...)
	}
	// create the encodedHeight with the right number of zeros
	s = make([]byte, sikLeafValueLen-encodedHeightLen)
	// copy the height bytes swapping endianness in the last bytes
	for i := encodedHeightLen - 1; i >= 0; i-- {
		s = append(s, bHeight[i])
	}
	return s
}

// DecodeInvalidatedHeight funtion returns the decoded height uint32 from the
// leaf value that contains an invalidated SIK.
func (s SIK) DecodeInvalidatedHeight() uint32 {
	bHeight := []byte{}
	for i := sikLeafValueLen - 1; len(bHeight) < encodedHeightLen; i-- {
		bHeight = append(bHeight, s[i])
	}
	return uint32(new(big.Int).SetBytes(bHeight).Int64())
}

// Valid function returns if the current SIK is a valid one or not.
func (s SIK) Valid() bool {
	for i := 0; i < len(s)-encodedHeightLen; i++ {
		if s[i] != 0 {
			return true
		}
	}
	return false
}

// String function return the human readable version of the current SIK, if it
// is a valid one, return the SIK value as hex. If it is already invalidated,
// return the decoded height.
func (s SIK) String() string {
	if s.Valid() {
		return hex.EncodeToString(s)
	}
	return fmt.Sprint(s.DecodeInvalidatedHeight())
}
