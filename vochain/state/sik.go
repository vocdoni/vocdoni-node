package state

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/tree/arbo"
)

var (
	sikDBPrefix     = []byte("sik/")
	anonSIKDBPrefix = []byte("asik/")
)

const (
	// SIKROOT_HYSTERESIS_BLOCKS constant defines the number of blocks that the
	// vochain will consider a sikRoot valid. In this way, any new sikRoot will be
	// valid for at least for this number of blocks. If the gap between the last
	// two valid roots is greater than the value of this constant, the oldest will
	// be valid until a new sikroot is calculated.
	// TODO: Move the definition to the right place
	SIKROOT_HYSTERESIS_BLOCKS = 32
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
	v.tx.RLock()
	defer v.tx.RUnlock()

	siksTree, err := v.tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	sik, err := siksTree.Get(address.Bytes())
	if err != nil {
		if errors.Is(err, arbo.ErrKeyNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrSIKNotFound, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrSIKGet, err)
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
	v.tx.Lock()
	defer v.tx.Unlock()
	siksTree, err := v.tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	// check if exists a registered sik for the provided address, query also for
	// no committed tree version
	rawSIK, err := siksTree.Get(address.Bytes())
	if errors.Is(err, arbo.ErrKeyNotFound) {
		// if not exists create it
		log.Debugw("setSIK (create)",
			"address", address.String(),
			"sik", newSIK.String())
		if err := siksTree.Add(address.Bytes(), newSIK); err != nil {
			return fmt.Errorf("%w: %w", ErrSIKSet, err)
		}
		return nil
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
	if err := siksTree.Set(address.Bytes(), newSIK); err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSet, err)
	}
	return nil
}

// InvalidateSIK function removes logically the registered SIK for the address
// provided. If it is not registered, it returns an error. If it is, it will
// encode the current height and set it as the SIK value to invalidate it and
// prevent it from being updated until all processes created before that height
// have finished.
func (v *State) InvalidateSIK(address common.Address) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	siksTree, err := v.tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	// if the sik does not exists or something fails querying return the error
	rawSIK, err := siksTree.Get(address.Bytes())
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKGet, err)
	}
	// if the stored sik is already invalidated return an error
	if !SIK(rawSIK).Valid() {
		return ErrSIKAlreadyInvalid
	}
	invalidatedSIK := make(SIK, sikLeafValueLen).InvalidateAt(v.CurrentHeight())
	if err := siksTree.Set(address.Bytes(), invalidatedSIK); err != nil {
		return fmt.Errorf("%w: %w", ErrSIKDelete, err)
	}
	return nil
}

// ValidSIKRoots method returns the current valid SIK roots that are cached in
// the current State. It is thread safe, but the returned value should not be
// modified by the caller.
func (v *State) ValidSIKRoots() [][]byte {
	v.mtxValidSIKRoots.Lock()
	defer v.mtxValidSIKRoots.Unlock()
	return slices.Clone(v.validSIKRoots)
}

// FetchValidSIKRoots updates the list of current valid SIK roots in the current
// state. It reads the roots from the key-value database associated to the SIK's
// subtree.
func (v *State) FetchValidSIKRoots() error {
	var validRoots [][]byte
	if err := v.NoState(true).Iterate(sikDBPrefix, func(_, root []byte) bool {
		validRoots = append(validRoots, root)
		return true
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrSIKIterate, err)
	}
	v.mtxValidSIKRoots.Lock()
	v.validSIKRoots = validRoots
	v.mtxValidSIKRoots.Unlock()
	return nil
}

// ExpiredSIKRoot returns if the provided siksRoot is still valid or not,
// checking if it is included into the list of current valid sik roots.
func (v *State) ExpiredSIKRoot(candidateRoot []byte) bool {
	for _, sikRoot := range v.ValidSIKRoots() {
		if bytes.Equal(sikRoot, candidateRoot) {
			return false
		}
	}
	return true
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
//     for the next lower sikRoot. It is because it still being validate for a
//     period.
func (v *State) UpdateSIKRoots() error {
	// instance the SIK's key-value DB and set the current block to the current
	// network height.
	v.mtxValidSIKRoots.Lock()
	defer v.mtxValidSIKRoots.Unlock()
	sikNoStateDB := v.NoState(false)
	currentBlock := v.CurrentHeight()

	// get sik roots key-value database associated to the siks tree
	v.tx.RLock()
	defer v.tx.RUnlock()
	siksTree, err := v.tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	// get new sik tree root hash
	newSikRoot, err := siksTree.Root()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKRootsGet, err)
	}
	// check if the new sik root is already in the list of valid roots, if so return
	for _, sikRoot := range v.validSIKRoots {
		if bytes.Equal(sikRoot, newSikRoot) {
			return nil
		}
	}
	// purge the oldest sikRoots if the hysteresis is reached
	if currentBlock > SIKROOT_HYSTERESIS_BLOCKS {
		// calculate the current minimun block to purge useless sik roots
		minBlock := currentBlock - SIKROOT_HYSTERESIS_BLOCKS
		minBlockKey := make([]byte, 32)
		binary.LittleEndian.PutUint32(minBlockKey, minBlock)
		minBlockKey = toPrefixKey(sikDBPrefix, minBlockKey)
		// if exists a sikRoot for the minimun block number just remove all
		// the roots with a lower block number. If not, remove all roots with a
		// lower block number except for the next lower sikRoot. To achieve that
		// iterate to select all the nearest lower block to the calculated min
		// block to delete them.
		var toPurge [][]byte
		var nearestLowerBlock uint32
		if err := sikNoStateDB.Iterate(sikDBPrefix, func(key, value []byte) bool {
			candidateKey := bytes.Clone(key)
			blockNumber := binary.LittleEndian.Uint32(candidateKey)
			if blockNumber < minBlock {
				if _, err := sikNoStateDB.Get(minBlockKey); err == nil || blockNumber > nearestLowerBlock {
					toPurge = append(toPurge, candidateKey)
					nearestLowerBlock = blockNumber
				}
			}
			return true
		}); err != nil {
			return fmt.Errorf("%w: %w", ErrSIKIterate, err)
		}
		// delete the selected sikRoots by its block numbers
		for _, blockToDelete := range toPurge {
			key := toPrefixKey(sikDBPrefix, blockToDelete)
			if err := sikNoStateDB.Delete(key); err != nil {
				return fmt.Errorf("%w: %w", ErrSIKRootsDelete, err)
			}
			log.Debugw("updateSIKRoots (deleted)",
				"blockNumber", binary.LittleEndian.Uint32(blockToDelete))
		}
	}
	// encode current blockNumber as key
	blockKey := make([]byte, 32)
	binary.LittleEndian.PutUint32(blockKey, currentBlock)
	key := toPrefixKey(sikDBPrefix, blockKey)
	// store the new sik tree root in no-state database
	if err := sikNoStateDB.Set(key, newSikRoot); err != nil {
		return fmt.Errorf("%w: %w", ErrSIKRootsSet, err)
	}
	// include the new root into the cached list
	v.validSIKRoots = append(v.validSIKRoots, newSikRoot)
	log.Debugw("updateSIKRoots (created)",
		"newSikRoot", hex.EncodeToString(newSikRoot),
		"blockNumber", currentBlock)
	return nil
}

// IncreaseRegisterSIKCounter method allows to keep in track the number of
// RegisterSIK actions by electionId (or processID). This helps to prevent
// attacks using the free tx of RegisterSIKTx. If the desired election has not
// any RegisterSIKTx associated, it will initialize the counter.
func (v *State) IncreaseRegisterSIKCounter(pid []byte) error {
	// get sik roots key-value database associated to the siks tree
	// instance the SIK's key-value DB
	sikNoStateDB := v.NoState(true)
	// prepare the prefixed key and get the current counter
	key := toPrefixKey(anonSIKDBPrefix, pid)
	rawCount, err := sikNoStateDB.Get(key)
	if err != nil {
		// if the key not exists, initialize the counter
		if errors.Is(err, db.ErrKeyNotFound) {
			rawCount = make([]byte, 32)
			binary.LittleEndian.PutUint32(rawCount, 1)
			if err := sikNoStateDB.Set(key, rawCount); err != nil {
				return fmt.Errorf("%w: %w", ErrSIKSet, err)
			}
			return nil
		}
		return fmt.Errorf("%w: %w", ErrSIKGet, err)
	}
	// If not, decode the current counter and increase by one
	count := binary.LittleEndian.Uint32(rawCount)
	binary.LittleEndian.PutUint32(rawCount, count+1)
	if err := sikNoStateDB.Set(key, rawCount); err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSet, err)
	}
	return nil
}

// PurgeRegisterSIK method removes the counter of RegisterSIKTx for the provided
// electionId (or processID)
func (v *State) PurgeRegisterSIK(pid []byte) error {
	// instance the SIK's key-value DB
	sikNoStateDB := v.NoState(true)
	// prepare the prefixed key and delete its records counter
	key := toPrefixKey(anonSIKDBPrefix, pid)
	if err := sikNoStateDB.Delete(key); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return fmt.Errorf("%w: %w", ErrSIKDelete, err)
	}
	return nil
}

// CountRegisterSIK method returns the number of RegisterSIKTx associated to
// the provided electionId (or processID)
func (v *State) CountRegisterSIK(pid []byte) (uint32, error) {
	// instance the SIK's key-value DB
	sikNoStateDB := v.NoState(true)
	// prepare the prefixed key and get its records counter
	rawCount, err := sikNoStateDB.Get(toPrefixKey(anonSIKDBPrefix, pid))
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("%w: %w", ErrSIKDelete, err)
	}
	return binary.LittleEndian.Uint32(rawCount), nil
}

// AssignSIKToElection function persists the relation between a created SIK
// (without registered account) and the election where the SIK is valid. This
// relation allows to remove all SIKs when the election ends.
func (v *State) AssignSIKToElection(pid []byte, address common.Address) error {
	// instance the SIK's key-value DB
	sikNoStateDB := v.NoState(true)
	// compose the key with the pid and the address and store with no value on
	// the no state database
	key := toPrefixKey(pid, address.Bytes())
	if err := sikNoStateDB.Set(key, nil); err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSet, err)
	}
	return nil
}

// PurgeSIKsByElection function iterates over the stored relations between a
// process and a SIK without account and remove both of them, the SIKs and also
// the relation.
func (v *State) PurgeSIKsByElection(pid []byte) error {
	v.tx.Lock()
	defer v.tx.Unlock()
	// get sik roots key-value database associated to the siks tree
	siksTree, err := v.tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	// instance the SIK's key-value DB
	sikNoStateDB := v.NoState(false)
	// iterate to remove the assigned SIK to every address of this process and
	// also the relation between them
	toPurge := [][]byte{}
	if err := sikNoStateDB.Iterate(pid, func(address, _ []byte) bool {
		toPurge = append(toPurge, bytes.Clone(address))
		return true
	}); err != nil {
		return fmt.Errorf("%w: %w", ErrSIKDelete, err)
	}
	for _, address := range toPurge {
		// remove the SIK by the address
		if err := siksTree.Del(address); err != nil {
			return fmt.Errorf("%w: %w", ErrSIKDelete, err)
		}
		// remove the relation between process and address
		if err := sikNoStateDB.Delete(toPrefixKey(pid, address)); err != nil {
			return fmt.Errorf("%w: %w", ErrSIKDelete, err)
		}
		log.Infow("temporal SIK purged", "address", hex.EncodeToString(address))
	}
	return nil
}

// SIKGenProof returns the proof of the provided address in the SIKs tree.
// The first returned value is the leaf value and the second the proof siblings.
func (v *State) SIKGenProof(address common.Address) ([]byte, []byte, error) {
	v.tx.RLock()
	defer v.tx.RUnlock()
	siksTree, err := v.tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	// get the sik proof
	return siksTree.GenProof(address.Bytes())
}

// SIKRoot returns the last root hash of the SIK merkle tree.
func (v *State) SIKRoot() ([]byte, error) {
	v.tx.RLock()
	defer v.tx.RUnlock()
	siksTree, err := v.tx.DeepSubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		v.tx.Unlock()
		return nil, fmt.Errorf("%w: %w", ErrSIKSubTree, err)
	}
	currentRoot, err := siksTree.Root()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSIKRootsGet, err)
	}
	return currentRoot, nil
}

// CountSIKs returns the overall number of SIKs the vochain has
func (v *State) CountSIKs(committed bool) (uint64, error) {
	// TODO: Once statedb.TreeView.Size() works, replace this by that.
	if !committed {
		v.tx.RLock()
		defer v.tx.RUnlock()
	}
	t, err := v.mainTreeViewer(committed).SubTree(StateTreeCfg(TreeSIK))
	if err != nil {
		return 0, err
	}
	return t.Size()
}

// InvalidateAt function sets the current SIK value to the encoded value of the
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
