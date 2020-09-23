package vochain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	amino "github.com/tendermint/go-amino"
	crypto "github.com/tendermint/tendermint/crypto"
	ed25519 "github.com/tendermint/tendermint/crypto/ed25519"
	tmtypes "github.com/tendermint/tendermint/types"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/statedb"
	"gitlab.com/vocdoni/go-dvote/statedb/iavlstate"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

const (
	// db names
	AppTree     = "app"
	ProcessTree = "process"
	VoteTree    = "vote"
	// validators default power
	validatorPower          = 0
	voteCachePurgeThreshold = time.Duration(time.Second * 600)
)

var (
	// keys; not constants because of []byte
	headerKey    = []byte("header")
	oracleKey    = []byte("oracle")
	validatorKey = []byte("validator")
)

var (
	ErrProcessNotFound = fmt.Errorf("process not found")
)

// PrefixDBCacheSize is the size of the cache for the MutableTree IAVL databases
var PrefixDBCacheSize = 0

// EventListener is an interface used for executing custom functions during the
// events of the block creation process.
// The order in which events are executed is: Rollback, OnVote or OnProcess, Commit.
// The process is concurrency safe, meaning that there cannot be two sequences
// happening in parallel.
type EventListener interface {
	OnVote(*types.Vote)
	OnProcess(pid, eid, mkroot, mkuri string)
	OnCancel(pid string)
	OnProcessKeys(pid, encryptionPub, commitment string)
	OnRevealKeys(pid, encryptionPriv, reveal string)
	Commit(height int64)
	Rollback()
}

// State represents the state of the vochain application
type State struct {
	Store         statedb.StateDB
	voteCache     map[string]*types.VoteProof
	voteCacheLock sync.RWMutex
	ImmutableState
	Codec *amino.Codec

	eventListeners []EventListener
}

// ImmutableState holds the latest trees version saved on disk
type ImmutableState struct {
	// Note that the mutex locks the entirety of the three IAVL trees, both
	// their mutable and immutable components. An immutable tree is not safe
	// for concurrent use with its parent mutable tree.
	sync.RWMutex
}

// NewState creates a new State
func NewState(dataDir string, codec *amino.Codec) (*State, error) {
	var err error
	vs := &State{}
	//vs.Store = new(gravitonstate.GravitonState)
	vs.Store = new(iavlstate.IavlState)

	if err = vs.Store.Init(dataDir, "disk"); err != nil {
		return nil, err
	}

	if err := vs.Store.AddTree(AppTree); err != nil {
		return nil, err
	}
	if err := vs.Store.AddTree(ProcessTree); err != nil {
		return nil, err
	}
	if err := vs.Store.AddTree(VoteTree); err != nil {
		return nil, err
	}

	// Must be -1 in order to get the last commited block state, if not block replay will fail
	if err = vs.Store.LoadVersion(-1); err != nil {
		return nil, err
	}

	vs.Codec = codec
	vs.voteCache = make(map[string]*types.VoteProof)
	log.Infof("application trees successfully loaded at version %d", vs.Store.Version())
	return vs, nil
}

// AddEventListener adds a new event listener, to receive method calls on block
// events as documented in EventListener.
func (v *State) AddEventListener(l EventListener) {
	v.eventListeners = append(v.eventListeners, l)
}

// AddOracle adds a trusted oracle given its address if not exists
func (v *State) AddOracle(address string) error {
	var err error
	address = util.TrimHex(address)
	v.Lock()
	defer v.Unlock()
	oraclesBytes := v.Store.Tree(AppTree).Get(oracleKey)
	var oracles []string
	v.Codec.UnmarshalBinaryBare(oraclesBytes, &oracles)
	for _, v := range oracles {
		if v == address {
			return errors.New("oracle already added")
		}
	}
	oracles = append(oracles, address)
	newOraclesBytes, err := v.Codec.MarshalBinaryBare(oracles)
	if err != nil {
		return errors.New("cannot marshal oracles")
	}
	return v.Store.Tree(AppTree).Add(oracleKey, newOraclesBytes)
}

// RemoveOracle removes a trusted oracle given its address if exists
func (v *State) RemoveOracle(address string) error {
	var oracles []string
	address = util.TrimHex(address)
	v.Lock()
	defer v.Unlock()
	oraclesBytes := v.Store.Tree(AppTree).Get(oracleKey)
	v.Codec.UnmarshalBinaryBare(oraclesBytes, &oracles)
	for i, o := range oracles {
		if o == address {
			// remove oracle
			copy(oracles[i:], oracles[i+1:])
			oracles[len(oracles)-1] = ""
			oracles = oracles[:len(oracles)-1]
			newOraclesBytes, err := v.Codec.MarshalBinaryBare(oracles)
			if err != nil {
				return errors.New("cannot marshal oracles")
			}
			return v.Store.Tree(AppTree).Add(oracleKey, newOraclesBytes)
		}
	}
	return errors.New("oracle not found")
}

// Oracles returns the current oracle list
func (v *State) Oracles(isQuery bool) ([]string, error) {
	var oraclesBytes []byte
	var err error
	v.RLock()
	defer v.RUnlock()
	if isQuery {
		oraclesBytes = v.Store.ImmutableTree(AppTree).Get(oracleKey)
	} else {
		oraclesBytes = v.Store.Tree(AppTree).Get(oracleKey)
	}
	var oracles []string
	err = v.Codec.UnmarshalBinaryBare(oraclesBytes, &oracles)
	return oracles, err
}

func hexPubKeyToTendermintEd25519(pubKey string) (crypto.PubKey, error) {
	var tmkey ed25519.PubKeyEd25519
	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return nil, err
	}
	if len(pubKeyBytes) != 32 {
		return nil, fmt.Errorf("pubKey lenght is invalid")
	}
	copy(tmkey[:], pubKeyBytes[:])
	return tmkey, nil
}

// AddValidator adds a tendemint validator if it is not already added
func (v *State) AddValidator(pubKey crypto.PubKey, power int64) error {
	var err error
	addr := pubKey.Address().String()
	v.Lock()
	defer v.Unlock()
	validatorsBytes := v.Store.Tree(AppTree).Get(validatorKey)
	var validators []tmtypes.GenesisValidator
	v.Codec.UnmarshalBinaryBare(validatorsBytes, &validators)
	for _, v := range validators {
		if v.PubKey.Address().String() == addr {
			return errors.New("validator already added")
		}
	}
	newVal := tmtypes.GenesisValidator{
		Address: pubKey.Address(),
		PubKey:  pubKey,
		Power:   validatorPower,
	}
	validators = append(validators, newVal)

	validatorsBytes, err = v.Codec.MarshalBinaryBare(validators)
	if err != nil {
		return errors.New("cannot marshal validator")
	}
	return v.Store.Tree(AppTree).Add(validatorKey, validatorsBytes)
}

// RemoveValidator removes a tendermint validator if exists
func (v *State) RemoveValidator(address string) error {
	v.RLock()
	validatorsBytes := v.Store.Tree(AppTree).Get(validatorKey)
	v.RUnlock()
	var validators []tmtypes.GenesisValidator
	v.Codec.UnmarshalBinaryBare(validatorsBytes, &validators)
	for i, val := range validators {
		if val.Address.String() == address {
			// remove validator
			copy(validators[i:], validators[i+1:])
			validators[len(validators)-1] = tmtypes.GenesisValidator{}
			validators = validators[:len(validators)-1]
			validatorsBytes, err := v.Codec.MarshalBinaryBare(validators)
			if err != nil {
				return errors.New("cannot marshal validators")
			}
			v.Lock()
			defer v.Unlock()
			return v.Store.Tree(AppTree).Add(validatorKey, validatorsBytes)
		}
	}
	return errors.New("validator not found")
}

// Validators returns a list of the validators saved on persistent storage
func (v *State) Validators(isQuery bool) ([]tmtypes.GenesisValidator, error) {
	var validatorBytes []byte
	var err error
	v.RLock()
	if isQuery {
		validatorBytes = v.Store.ImmutableTree(AppTree).Get(validatorKey)
	} else {
		validatorBytes = v.Store.Tree(AppTree).Get(validatorKey)
	}
	v.RUnlock()
	var validators []tmtypes.GenesisValidator
	err = v.Codec.UnmarshalBinaryBare(validatorBytes, &validators)
	return validators, err
}

// AddProcessKeys adds the keys to the process
func (v *State) AddProcessKeys(tx *types.AdminTx) error {
	pid := util.TrimHex(tx.ProcessID)
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	if len(tx.CommitmentKey) > 0 {
		process.CommitmentKeys[tx.KeyIndex] = util.TrimHex(tx.CommitmentKey)
		log.Debugf("added commitment key for process %s: %s", pid, tx.CommitmentKey)
	}
	if len(tx.EncryptionPublicKey) > 0 {
		process.EncryptionPublicKeys[tx.KeyIndex] = util.TrimHex(tx.EncryptionPublicKey)
		log.Debugf("added encryption key for process %s: %s", pid, tx.EncryptionPublicKey)
	}
	process.KeyIndex++
	if err := v.setProcess(process, pid); err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnProcessKeys(pid, tx.EncryptionPublicKey, tx.CommitmentKey)
	}
	return nil
}

// RevealProcessKeys reveals the keys of a process
func (v *State) RevealProcessKeys(tx *types.AdminTx) error {
	pid := util.TrimHex(tx.ProcessID)
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	if len(tx.RevealKey) > 0 {
		process.RevealKeys[tx.KeyIndex] = util.TrimHex(tx.RevealKey)
		log.Debugf("revealed commitment key for process %s: %s", pid, tx.RevealKey)
	}
	if len(tx.EncryptionPrivateKey) > 0 {
		process.EncryptionPrivateKeys[tx.KeyIndex] = util.TrimHex(tx.EncryptionPrivateKey)
		log.Debugf("revealed encryption key for process %s: %s", pid, tx.EncryptionPrivateKey)
	}
	if process.KeyIndex < 1 {
		return fmt.Errorf("no more keys to reveal, keyIndex is < 1")
	}
	process.KeyIndex--
	if err := v.setProcess(process, pid); err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnRevealKeys(pid, tx.EncryptionPrivateKey, tx.RevealKey)
	}
	return nil
}

// AddProcess adds a new process to vochain if not already added
func (v *State) AddProcess(p types.Process, pid, mkuri string) error {
	pid = util.TrimHex(pid)
	p.EntityID = util.TrimHex(p.EntityID)
	newProcessBytes, err := v.Codec.MarshalBinaryBare(p)
	if err != nil {
		return errors.New("cannot marshal process bytes")
	}
	v.Lock()
	err = v.Store.Tree(ProcessTree).Add([]byte(pid), newProcessBytes)
	v.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnProcess(pid, p.EntityID, p.MkRoot, mkuri)
	}
	return nil
}

// CancelProcess sets the process canceled atribute to true
func (v *State) CancelProcess(pid string) error {
	pid = util.TrimHex(pid)
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	if process.Canceled {
		return nil
	}
	process.Canceled = true
	updatedProcessBytes, err := v.Codec.MarshalBinaryBare(process)
	if err != nil {
		return errors.New("cannot marshal updated process bytes")
	}
	v.Lock()
	err = v.Store.Tree(ProcessTree).Add([]byte(pid), updatedProcessBytes)
	v.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnCancel(pid)
	}
	return nil
}

// PauseProcess sets the process paused atribute to true
func (v *State) PauseProcess(pid string) error {
	pid = util.TrimHex(pid)
	v.RLock()
	processBytes := v.Store.Tree(ProcessTree).Get([]byte(pid))
	v.RUnlock()
	var process types.Process
	if err := v.Codec.UnmarshalBinaryBare(processBytes, &process); err != nil {
		return errors.New("cannot unmarshal process")
	}
	// already paused
	if process.Paused {
		return nil
	}
	process.Paused = true
	return v.setProcess(&process, pid)
}

// ResumeProcess sets the process paused atribute to true
func (v *State) ResumeProcess(pid string) error {
	pid = util.TrimHex(pid)
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	// non paused process cannot be resumed
	if !process.Paused {
		return nil
	}
	process.Paused = false
	return v.setProcess(process, pid)
}

// Process returns a process info given a processId if exists
func (v *State) Process(pid string, isQuery bool) (*types.Process, error) {
	var process *types.Process
	var processBytes []byte
	var err error
	pid = util.TrimHex(pid)
	v.RLock()
	if isQuery {
		processBytes = v.Store.ImmutableTree(ProcessTree).Get([]byte(pid))
	} else {
		processBytes = v.Store.Tree(ProcessTree).Get([]byte(pid))
	}
	v.RUnlock()
	if processBytes == nil {
		return nil, ErrProcessNotFound
	}
	err = v.Codec.UnmarshalBinaryBare(processBytes, &process)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal process with id (%s)", pid)
	}
	return process, nil
}

// CountProcesses returns the overall number of processes the vochain has
func (v *State) CountProcesses(isQuery bool) int64 {
	v.RLock()
	defer v.RUnlock()
	if isQuery {
		return int64(v.Store.ImmutableTree(ProcessTree).Count())
	}
	return int64(v.Store.Tree(ProcessTree).Count())

}

// set process stores in the database the process
func (v *State) setProcess(process *types.Process, pid string) error {
	if process == nil {
		return ErrProcessNotFound
	}
	updatedProcessBytes, err := v.Codec.MarshalBinaryBare(process)
	if err != nil {
		return errors.New("cannot marshal updated process bytes")
	}
	v.Lock()
	defer v.Unlock()
	if err := v.Store.Tree(ProcessTree).Add([]byte(pid), updatedProcessBytes); err != nil {
		return err
	}
	return nil
}

// AddVote adds a new vote to a process if the process exists and the vote is not already submmited
func (v *State) AddVote(vote *types.Vote) error {
	voteID := fmt.Sprintf("%s_%s", util.TrimHex(vote.ProcessID), util.TrimHex(vote.Nullifier))
	// save block number
	vote.Height = v.Header(false).Height
	newVoteBytes, err := v.Codec.MarshalBinaryBare(vote)
	if err != nil {
		return fmt.Errorf("cannot marshal vote")
	}
	v.Lock()
	err = v.Store.Tree(VoteTree).Add([]byte(voteID), newVoteBytes)
	v.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnVote(vote)
	}
	return nil
}

// Envelope returns the info of a vote if already exists.
// voteID must be equals to processID_Nullifier
func (v *State) Envelope(voteID string, isQuery bool) (_ *types.Vote, err error) {
	// TODO(mvdan): remove the recover once
	// https://github.com/tendermint/iavl/issues/212 is fixed
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered panic: %v", r)
		}
	}()
	var vote *types.Vote
	var voteBytes []byte
	v.RLock()
	defer v.RUnlock() // needs to be deferred due to the recover above
	if isQuery {
		voteBytes = v.Store.ImmutableTree(VoteTree).Get([]byte(voteID))
	} else {
		voteBytes = v.Store.Tree(VoteTree).Get([]byte(voteID))
	}
	if voteBytes == nil {
		return nil, fmt.Errorf("vote with id (%s) does not exist", voteID)
	}
	if err := v.Codec.UnmarshalBinaryBare(voteBytes, &vote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote with id (%s)", voteID)
	}
	return vote, nil
}

// EnvelopeExists returns true if the envelope identified with voteID exists
func (v *State) EnvelopeExists(processID, nullifier string) bool {
	voteID := fmt.Sprintf("%s_%s", util.TrimHex(processID), util.TrimHex(nullifier))
	v.RLock()
	defer v.RUnlock()
	b := v.Store.ImmutableTree(VoteTree).Get([]byte(voteID)) // TBD: This is quite dirty, should be changed
	return len(b) > 0
}

// The prefix is "processID_", where processID is a stringified hash of fixed length.
// To iterate over all the keys with said prefix, the start point can simply be "processID_".
// We don't know what the next processID hash will be, so we use "processID}"
// as the end point, since "}" comes after "_" when sorting lexicographically.
func (v *State) iterateProcessID(processID string, fn func(key []byte, value []byte) bool, isQuery bool) bool {
	v.RLock()
	defer v.RUnlock()
	if isQuery {
		v.Store.ImmutableTree(VoteTree).Iterate(processID+"_", processID+"}", fn)
	} else {
		v.Store.Tree(VoteTree).Iterate(processID+"_", processID+"}", fn)
	}
	return true
}

// CountVotes returns the number of votes registered for a given process id
func (v *State) CountVotes(processID string, isQuery bool) int64 {
	processID = util.TrimHex(processID)
	var count int64
	v.iterateProcessID(processID, func(key []byte, value []byte) bool {
		count++
		return false
	}, isQuery)
	return count
}

// EnvelopeList returns a list of registered envelopes nullifiers given a processId
func (v *State) EnvelopeList(processID string, from, listSize int64, isQuery bool) []string {
	// TODO(mvdan): remove the recover once
	// https://github.com/tendermint/iavl/issues/212 is fixed
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered panic: %v", r)
			// TODO(mvdan): this func should return an error instead
			// err = fmt.Errorf("recovered panic: %v", r)
		}
	}()
	processID = util.TrimHex(processID)
	var nullifiers []string
	idx := int64(0)
	v.iterateProcessID(processID, func(key []byte, value []byte) bool {
		if idx >= from+listSize {
			return true
		}
		if idx >= from {
			k := strings.Split(string(key), "_")
			nullifiers = append(nullifiers, k[1])
		}
		idx++
		return false
	}, isQuery)
	return nullifiers
}

// Header returns the blockchain last block commited height
func (v *State) Header(isQuery bool) *tmtypes.Header {
	var headerBytes []byte
	v.RLock()
	if isQuery {
		headerBytes = v.Store.ImmutableTree(AppTree).Get(headerKey)
	} else {
		headerBytes = v.Store.Tree(AppTree).Get(headerKey)
	}
	v.RUnlock()
	var header tmtypes.Header
	err := v.Codec.UnmarshalBinaryBare(headerBytes, &header)
	if err != nil {
		log.Errorf("cannot get vochain height: %s", err)
		return nil
	}
	return &header
}

// AppHash returns last hash of the application
func (v *State) AppHash(isQuery bool) []byte {
	var headerBytes []byte
	v.RLock()
	if isQuery {
		headerBytes = v.Store.ImmutableTree(AppTree).Get(headerKey)
	} else {
		headerBytes = v.Store.Tree(AppTree).Get(headerKey)
	}
	v.RUnlock()
	var header tmtypes.Header
	err := v.Codec.UnmarshalBinaryBare(headerBytes, &header)
	if err != nil {
		return []byte{}
	}
	return header.AppHash
}

// Save persistent save of vochain mem trees
func (v *State) Save() []byte {
	v.Lock()
	hash, err := v.Store.Commit()
	if err != nil {
		panic(fmt.Sprintf("cannot commit state trees: (%s)", err))
	}
	log.Debugf("commited state tree version %d, hash %x", v.Store.Version(), hash)
	v.Unlock()
	if h := v.Header(false); h != nil {
		for _, l := range v.eventListeners {
			l.Commit(h.Height)
		}
	}
	return hash
}

// Rollback rollbacks to the last persistent db data version
func (v *State) Rollback() {
	for _, l := range v.eventListeners {
		l.Rollback()
	}
	v.Lock()
	defer v.Unlock()
	if err := v.Store.Rollback(); err != nil {
		panic(fmt.Sprintf("cannot rollback state tree: (%s)", err))
	}
}

// WorkingHash returns the hash of the vochain trees mkroots
// hash(appTree+processTree+voteTree)
func (v *State) WorkingHash() []byte {
	v.RLock()
	defer v.RUnlock()
	return v.Store.Hash()
}
