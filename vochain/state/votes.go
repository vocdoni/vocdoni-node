package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/statedb"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

var (
	// keys; not constants because of []byte
	voteCountKey = []byte("voteCount")
)

// Vote represents a vote in the Vochain state.
type Vote struct {
	ProcessID            types.HexBytes
	Nullifier            types.HexBytes
	Height               uint32
	VotePackage          []byte
	EncryptionKeyIndexes []uint32
	Weight               *big.Int
	VoterID              VoterID
	Overwrites           uint32
}

// VotePackage represents the payload of a vote (usually base64 encoded).
type VotePackage struct {
	Nonce string `json:"nonce,omitempty"`
	Votes []int  `json:"votes"`
}

// NewVotePackage creates a new vote package with the given vote slice.
func NewVotePackage(votes []int) *VotePackage {
	return &VotePackage{
		Nonce: util.RandomHex(16),
		Votes: votes,
	}
}

// Encode returns the json encoded vote package.
func (vp *VotePackage) Encode() ([]byte, error) {
	return json.Marshal(vp)
}

// Decode decodes the json encoded vote package.
func (vp *VotePackage) Decode(data []byte) error {
	return json.Unmarshal(data, vp)
}

// WeightBytes returns the vote weight as a byte slice. If the weight is nil, it returns a byte slice of 1.
func (v *Vote) WeightBytes() []byte {
	if v.Weight != nil {
		return v.Weight.Bytes()
	}
	return big.NewInt(1).Bytes()
}

// Hash returns the hash of the vote. Only the fields that are an essential part of the vote are hashed.
func (v *Vote) Hash() []byte {
	h := bytes.Buffer{}
	h.Write(v.ProcessID) // processID includes the chainID encoded in the first bytes
	h.Write(v.Nullifier)
	h.Write(v.VotePackage)
	h.Write(v.WeightBytes())
	return ethereum.HashRaw(h.Bytes())
}

// DeepCopy returns a deep copy of the Vote struct.
func (v *Vote) DeepCopy() *Vote {
	voteCopy := &Vote{
		ProcessID:            slices.Clone(v.ProcessID),
		Nullifier:            slices.Clone(v.Nullifier),
		Height:               v.Height,
		VotePackage:          slices.Clone(v.VotePackage),
		EncryptionKeyIndexes: slices.Clone(v.EncryptionKeyIndexes),
		Weight:               new(big.Int).Set(v.Weight),
		VoterID:              slices.Clone(v.VoterID),
		Overwrites:           v.Overwrites,
	}
	return voteCopy
}

// CountTotalVotes return the global vote count.
// When committed is false, the operation is executed also on not yet committed
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last committed version.
func (s *State) CountTotalVotes() (uint64, error) {
	noState := s.NoState(true)
	voteCountLE, err := noState.Get(voteCountKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(voteCountLE), nil
}

// voteCountInc increases by 1 the global vote count.
func (s *State) voteCountInc() error {
	noState := s.NoState(false)
	voteCountLE, err := noState.Get(voteCountKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		voteCountLE = make([]byte, 8)
	} else if err != nil {
		return err
	}
	voteCount := binary.LittleEndian.Uint64(voteCountLE)
	voteCount++
	binary.LittleEndian.PutUint64(voteCountLE, voteCount)
	return noState.Set(voteCountKey, voteCountLE)
}

// AddVote adds a new vote to a process and call the even listeners to OnVote.
// If the vote already exists it will be overwritten and overwrite counter will be increased.
// Note that the vote is not committed to the StateDB until the StateDB transaction is committed.
// Note that the vote is not verified, so it is the caller responsibility to verify the vote.
func (s *State) AddVote(vote *Vote) error {
	vid, err := s.voteID(vote.ProcessID, vote.Nullifier)
	if err != nil {
		return err
	}
	// save block number
	vote.Height = s.CurrentHeight()

	// get the vote from state database
	sdbVote, err := s.Vote(vote.ProcessID, vote.Nullifier, false)
	if err != nil {
		if errors.Is(err, ErrVoteNotFound) {
			sdbVote = &models.StateDBVote{
				VoteHash:             vote.Hash(),
				Nullifier:            vote.Nullifier,
				Weight:               vote.WeightBytes(),
				VotePackage:          vote.VotePackage,
				EncryptionKeyIndexes: vote.EncryptionKeyIndexes,
			}
		} else {
			return err
		}
	} else {
		// overwrite vote if it already exists
		sdbVote.VoteHash = vote.Hash()
		sdbVote.VotePackage = vote.VotePackage
		sdbVote.Weight = vote.WeightBytes()
		sdbVote.EncryptionKeyIndexes = vote.EncryptionKeyIndexes
		if sdbVote.OverwriteCount != nil {
			*sdbVote.OverwriteCount++
		} else {
			sdbVote.OverwriteCount = new(uint32)
			*sdbVote.OverwriteCount = 1
		}
	}
	sdbVoteBytes, err := proto.Marshal(sdbVote)
	if err != nil {
		return fmt.Errorf("cannot marshal sdbVote: %w", err)
	}
	s.tx.Lock()
	err = func() error {
		treeCfg := StateChildTreeCfg(ChildTreeVotes)
		if err := s.tx.DeepSet(vid, sdbVoteBytes,
			StateTreeCfg(TreeProcess), treeCfg.WithKey(vote.ProcessID)); err != nil {
			return err
		}
		return s.voteCountInc()
	}()
	s.tx.Unlock()
	if err != nil {
		return err
	}
	if sdbVote.OverwriteCount != nil {
		vote.Overwrites = *sdbVote.OverwriteCount
	}
	for _, l := range s.eventListeners {
		l.OnVote(vote, s.TxCounter())
	}
	return nil
}

// NOTE(Edu): Changed this from byte(processID+nullifier) to
// hash(processID+nullifier) to allow using it as a key in Arbo tree.
// voteID = hash(processID+nullifier)
func (*State) voteID(pid, nullifier []byte) ([]byte, error) {
	if len(pid) != types.ProcessIDsize {
		return nil, fmt.Errorf("wrong processID size %d", len(pid))
	}
	// The vote nullifier generated during anonymous voting with zksnarks could
	// have 31 or 32 bytes of length. Now, here only check that the nullifier is
	// not empty and then, if it is wrong, the verification will be fail.
	// TODO: (lucas) Set a minimun and maximum length for the nullifier and
	// check it.
	if len(nullifier) == 0 {
		return nil, fmt.Errorf("empty or nil nullifier")
	}
	vid := sha256.New()
	vid.Write(pid)
	vid.Write(nullifier)
	return vid.Sum(nil), nil
}

// Vote returns the stored vote if exists. Returns ErrProcessNotFound if the
// process does not exist, ErrVoteNotFound if the vote does not exist.
// When committed is false, the operation is executed also on not yet committed
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last committed version.
func (s *State) Vote(processID, nullifier []byte, committed bool) (*models.StateDBVote, error) {
	vid, err := s.voteID(processID, nullifier)
	if err != nil {
		return nil, err
	}
	if !committed {
		// acquire a write lock, since DeepSubTree will create some temporary trees in memory
		// that might be read concurrently by DeliverTx path during block commit, leading to race #581
		// https://github.com/vocdoni/vocdoni-node/issues/581
		s.tx.Lock()
		defer s.tx.Unlock()
	}
	treeCfg := StateChildTreeCfg(ChildTreeVotes)
	votesTree, err := s.mainTreeViewer(committed).DeepSubTree(
		StateTreeCfg(TreeProcess), treeCfg.WithKey(processID))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, ErrProcessNotFound
	} else if err != nil {
		return nil, err
	}

	sdbVoteBytes, err := votesTree.Get(vid)
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, ErrVoteNotFound
	} else if err != nil {
		return nil, err
	}
	var sdbVote models.StateDBVote
	if err := proto.Unmarshal(sdbVoteBytes, &sdbVote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal sdbVote: %w", err)
	}
	return &sdbVote, nil
}

// IterateVotes iterates over all the votes of a process. The callback function is executed for each vote.
// Once the callback returns true, the iteration stops.
func (s *State) IterateVotes(processID []byte, committed bool, callback func(vote *models.StateDBVote) bool) error {
	if !committed {
		s.tx.Lock()
		defer s.tx.Unlock()
	}
	treeCfg := StateChildTreeCfg(ChildTreeVotes)
	votesTree, err := s.mainTreeViewer(committed).DeepSubTree(
		StateTreeCfg(TreeProcess), treeCfg.WithKey(processID))
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return ErrProcessNotFound
	} else if err != nil {
		return err
	}
	votesTree.Iterate(func(_, v []byte) bool {
		var sdbVote models.StateDBVote
		if err := proto.Unmarshal(v, &sdbVote); err != nil {
			log.Errorw(err, "cannot unmarshal vote")
			return false
		}
		return callback(&sdbVote)
	})
	return nil
}

// VoteExists returns true if the envelope identified with voteID exists
// When committed is false, the operation is executed also on not yet committed
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last committed version.
func (s *State) VoteExists(processID, nullifier []byte, committed bool) (bool, error) {
	_, err := s.Vote(processID, nullifier, committed)
	if errors.Is(err, ErrProcessNotFound) {
		return false, nil
	} else if errors.Is(err, ErrVoteNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// iterateVotes iterates fn over state tree entries with the processID prefix.
// When committed is false, the operation is executed also on not yet committed
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last committed version.
func (s *State) iterateVotes(processID []byte,
	fn func(vid []byte, sdbVote *models.StateDBVote) bool, committed bool) error {
	if !committed {
		s.tx.RLock()
		defer s.tx.RUnlock()
	}
	treeCfg := StateChildTreeCfg(ChildTreeVotes)
	votesTree, err := s.mainTreeViewer(committed).DeepSubTree(
		StateTreeCfg(TreeProcess), treeCfg.WithKey(processID))
	if err != nil {
		return err
	}
	var callbackErr error
	if err := votesTree.Iterate(func(key, value []byte) bool {
		var sdbVote models.StateDBVote
		if err := proto.Unmarshal(value, &sdbVote); err != nil {
			callbackErr = err
			return true
		}
		return fn(key, &sdbVote)
	}); err != nil {
		return err
	}
	if callbackErr != nil {
		return callbackErr
	}
	return nil
}

// CountVotes returns the number of votes registered for a given process id
// When committed is false, the operation is executed also on not yet committed
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last committed version.
func (s *State) CountVotes(processID []byte, committed bool) (uint64, error) {
	votesTree, err := s.mainTreeViewer(committed).DeepSubTree(
		StateTreeCfg(TreeProcess),
		StateChildTreeCfg(ChildTreeVotes).WithKey(processID),
	)
	if err != nil {
		if errors.Is(err, statedb.ErrEmptyTree) {
			return 0, nil
		}
		return 0, err
	}
	size, err := votesTree.Size()
	if errors.Is(err, statedb.ErrEmptyTree) {
		return 0, nil
	}
	return size, err
}

// EnvelopeList returns a list of registered envelopes nullifiers given a processId
// When committed is false, the operation is executed also on not yet committed
// data from the currently open StateDB transaction.
// When committed is true, the operation is executed on the last committed version.
func (s *State) EnvelopeList(processID []byte, from, listSize int,
	committed bool) (nullifiers [][]byte) {
	idx := 0
	s.iterateVotes(processID, func(vid []byte, sdbVote *models.StateDBVote) bool {
		if idx >= from+listSize {
			return true
		}
		if idx >= from {
			nullifiers = append(nullifiers, sdbVote.Nullifier)
		}
		idx++
		return false
	}, committed)
	return nullifiers
}
