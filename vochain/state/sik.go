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

// SIKROOT_HYSTERESIS_BLOCKS constant defines the number of blocks that the
// vochain will consider a sikRoot valid. In this way, any new sikRoot will be
// valid for at least for this number of blocks. If the gap between the last
// two valid roots is greater than the value of this constant, the oldest will
// be valid until a new sikroot is calculated.
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
	sik, err := v.mainTreeViewer(false).DeepGet(address.Bytes(), StateTreeCfg(TreeSIK))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	return sik, nil
}

// SetAddressSIK function creates or update the SIK of the provided address in the
// state. It covers the following cases:
//   - It checks if already exists a valid SIK for the provided address and if
//     so it returns an error.
//   - If no SIK exists for the provided address, it will create one with the
//     value provided.
//   - If it exists but it is not valid, overwrite the stored value with the
//     provided one.
func (v *State) SetAddressSIK(address common.Address, newSIK SIK) error {
	siksTree, err := v.Tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	// check if exists a registered sik for the provided address, query also for
	// no commited tree version
	v.Tx.Lock()
	rawSIK, err := siksTree.Get(address.Bytes())
	v.Tx.Unlock()
	if errors.Is(err, arbo.ErrKeyNotFound) {
		// if not exists create it
		log.Debugw("setSIK (create)",
			"address", address.String(),
			"sik", newSIK.String())
		v.Tx.Lock()
		err = siksTree.Add(address.Bytes(), newSIK)
		v.Tx.Unlock()
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSIKSet, err)
		}
		return v.UpdateSIKRoots()
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKGet, err)
	}
	// check if is a valid sik
	if SIK(rawSIK).Valid() {
		return ErrRegisteredValidSIK
	}
	log.Debugw("setSIK (update)",
		"address", address.String(),
		"sik", SIK(rawSIK).String())
	// if the hysteresis is reached update the sik for the address
	v.Tx.Lock()
	err = siksTree.Set(address.Bytes(), newSIK)
	v.Tx.Unlock()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSet, err)
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
	rawSIK, err := v.mainTreeViewer(false).DeepGet(address.Bytes(), StateTreeCfg(TreeSIK))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKGet, err)
	}
	// if the stored sik is already invalidated return an error
	if !SIK(rawSIK).Valid() {
		return ErrSIKAlreadyInvalid
	}
	v.Tx.Lock()
	invalidatedSIK := make(SIK, sikLeafValueLen).InvalidateAt(v.CurrentHeight())
	err = v.Tx.DeepSet(address.Bytes(), invalidatedSIK, StateTreeCfg(TreeSIK))
	v.Tx.Unlock()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKDelete, err)
	}
	return v.UpdateSIKRoots()
}

// ValidSIKRoots method returns the current valid SIK roots that are cached in
// the current State.
func (v *State) ValidSIKRoots() [][]byte {
	v.mtxValidSIKRoots.Lock()
	defer v.mtxValidSIKRoots.Unlock()
	return append([][]byte{}, v.validSIKRoots...)
}

// FetchValidSIKRoots updates the list of current valid SIK roots in the current
// state. It reads the roots from the key-value database associated to the SIK's
// subtree.
func (v *State) FetchValidSIKRoots() error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	siksTree, err := v.Tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	var validRoots [][]byte
	siksTree.NoState().Iterate(nil, func(_, root []byte) bool {
		validRoots = append(validRoots, root)
		return true
	})
	v.mtxValidSIKRoots.Lock()
	v.validSIKRoots = validRoots
	v.mtxValidSIKRoots.Unlock()
	return nil
}

// ExpiredSIK returns if the provided siksRoot is still valid or not, checking
// if it is included into the list of current valid sik roots.
func (v *State) ExpiredSIK(candidateRoot []byte) (bool, error) {
	for _, sikRoot := range v.ValidSIKRoots() {
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
//
//   - If exists a sikRoot for the minimun hysteresis block number
//     (currentBlock - hysteresis), just remove all the roots with a lower block
//     number.
//
//   - If it does not exist, remove all roots with a lower block number except
//     for the next lower sikRoot. It is becouse it still being validate for a
//     period.
func (v *State) UpdateSIKRoots() error {
	v.Tx.Lock()
	defer v.Tx.Unlock()
	// get sik roots key-value database associated to the siks tree
	siksTree, err := v.Tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	newSikRoot, err := siksTree.Root()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKRootsGet, err)
	}
	// instance the SIK's key-value DB and set the current block to the current
	// network height.
	sikRootsDB := siksTree.NoState()
	currentBlock := v.CurrentHeight()
	if currentBlock > SIKROOT_HYSTERESIS_BLOCKS {
		// calculate the current minimun block to purge useless sik roots
		minBlock := currentBlock - SIKROOT_HYSTERESIS_BLOCKS
		minBlockKey := make([]byte, 32)
		binary.LittleEndian.PutUint32(minBlockKey, minBlock)
		// if exists a sikRoot for the minimun block number just remove all
		// the roots with a lower block number. If not, remove all roots with a
		// lower block number except for the next lower sikRoot. To achieve that
		// iterate to select all the nearest lower block to the calculated min
		// block to delete them.
		var toPurge [][]byte
		var nearestLowerBlock uint32
		sikRootsDB.Iterate(nil, func(key, value []byte) bool {
			candidateKey := bytes.Clone(key)
			blockNumber := binary.LittleEndian.Uint32(candidateKey)
			if blockNumber < minBlock {
				if _, err := sikRootsDB.Get(minBlockKey); err == nil || blockNumber > nearestLowerBlock {
					toPurge = append(toPurge, candidateKey)
					nearestLowerBlock = blockNumber
				}
			}
			return true
		})
		// delete the selected sikRoots by its block numbers
		for _, blockToDelete := range toPurge {
			if err := sikRootsDB.Delete(blockToDelete); err != nil {
				return fmt.Errorf("%w: %w", ErrSIKRootsDelete, err)
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

	if err := sikRootsDB.Set(blockKey, newSikRoot); err != nil {
		return fmt.Errorf("%w: %w", ErrSIKRootsSet, err)
	}

	return nil
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
	var bHeight []byte
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
