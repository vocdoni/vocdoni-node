package vochain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/iavl"
	crypto "github.com/tendermint/tendermint/crypto"
	ed25519 "github.com/tendermint/tendermint/crypto/ed25519"
	tmtypes "github.com/tendermint/tendermint/types"
	tmdb "github.com/tendermint/tm-db"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
)

const (
	// db names
	appTreeName     = "appTree"
	processTreeName = "processTree"
	voteTreeName    = "voteTree"
	// validators default power
	validatorPower = 0
)

var (
	// keys; not constants because of []byte
	headerKey    = []byte("header")
	oracleKey    = []byte("oracle")
	validatorKey = []byte("validator")
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
	OnProcess(pid, eid string)
	OnCancel(pid string)
	OnProcessKeys(pid, encryptionPub, commitment string)
	OnRevealKeys(pid, encryptionPriv, reveal string)
	Commit(height int64)
	Rollback()
}

// State represents the state of the vochain application
type State struct {
	AppTree     *iavl.MutableTree
	ProcessTree *iavl.MutableTree
	VoteTree    *iavl.MutableTree
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

	IAppTree     *iavl.ImmutableTree
	IProcessTree *iavl.ImmutableTree
	IVoteTree    *iavl.ImmutableTree
}

// NewState creates a new State
func NewState(dataDir string, codec *amino.Codec) (*State, error) {
	appTree, err := tmdb.NewGoLevelDB(appTreeName, dataDir)
	if err != nil {
		return nil, err
	}

	processTree, err := tmdb.NewGoLevelDB(processTreeName, dataDir)
	if err != nil {
		return nil, err
	}
	voteTree, err := tmdb.NewGoLevelDB(voteTreeName, dataDir)
	if err != nil {
		return nil, err
	}
	vs := &State{}
	vs.AppTree, err = iavl.NewMutableTree(appTree, PrefixDBCacheSize)
	if err != nil {
		return nil, err
	}
	vs.ProcessTree, err = iavl.NewMutableTree(processTree, PrefixDBCacheSize)
	if err != nil {
		return nil, err
	}
	vs.VoteTree, err = iavl.NewMutableTree(voteTree, PrefixDBCacheSize)
	if err != nil {
		return nil, err
	}

	version, err := vs.AppTree.Load()
	if err != nil {
		return nil, err
	}

	var atVersion, ptVersion, vtVersion int64
	if version > 0 {
		version--
	}
	atVersion, err = vs.AppTree.LoadVersionForOverwriting(version)
	if err != nil {
		return nil, err
	}
	ptVersion, err = vs.ProcessTree.LoadVersionForOverwriting(version)
	if err != nil {
		return nil, err
	}
	vtVersion, err = vs.VoteTree.LoadVersionForOverwriting(version)
	if err != nil {
		return nil, err
	}

	// We set the mutable tree fields; ensure that, for consistency, the
	// immutable tree fields aren't nil.
	// This is important in the case an endpoint call comes in while we're
	// still initializing, and InitChain hasn't run yet. In that scenario,
	// we would get a nil pointer dereference panic.
	// TODO(mvdan): this is still racy, since we can't use GetImmutable, but
	// at least we won't panic every single time.
	vs.IAppTree = vs.AppTree.ImmutableTree
	vs.IProcessTree = vs.ProcessTree.ImmutableTree
	vs.IVoteTree = vs.VoteTree.ImmutableTree

	vs.Codec = codec

	log.Infof("application trees successfully loaded. appTree:%d processTree:%d voteTree: %d", atVersion, ptVersion, vtVersion)
	return vs, nil
}

// Immutable creates immutable state
func (v *State) Immutable() error {
	var err error
	// get immutable app tree of the latest app tree version saved
	v.IAppTree, err = v.AppTree.GetImmutable(v.AppTree.Version())
	if err != nil {
		return err
	}

	// get immutable process tree of the latest process tree version saved
	v.IProcessTree, err = v.ProcessTree.GetImmutable(v.ProcessTree.Version())
	if err != nil {
		return err
	}

	// get immutable vote tree of the latest vote tree version saved
	v.IVoteTree, err = v.VoteTree.GetImmutable(v.VoteTree.Version())
	if err != nil {
		return err
	}
	return nil
}

// AddEventListener adds a new event listener, to receive method calls on block
// events as documented in EventListener.
func (v *State) AddEventListener(l EventListener) {
	v.eventListeners = append(v.eventListeners, l)
}

// AddOracle adds a trusted oracle given its address if not exists
func (v *State) AddOracle(address string) error {
	address = util.TrimHex(address)
	v.RLock()
	_, oraclesBytes := v.AppTree.Get(oracleKey)
	v.RUnlock()
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
	v.Lock()
	v.AppTree.Set(oracleKey, newOraclesBytes)
	v.Unlock()
	return nil
}

// RemoveOracle removes a trusted oracle given its address if exists
func (v *State) RemoveOracle(address string) error {
	address = util.TrimHex(address)
	v.RLock()
	_, oraclesBytes := v.AppTree.Get(oracleKey)
	v.RUnlock()
	var oracles []string
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
			v.Lock()
			v.AppTree.Set(oracleKey, newOraclesBytes)
			v.Unlock()
			return nil
		}
	}
	return errors.New("oracle not found")
}

// Oracles returns the current oracle list
func (v *State) Oracles(isQuery bool) ([]string, error) {
	var oraclesBytes []byte
	v.RLock()
	if isQuery {
		_, oraclesBytes = v.IAppTree.Get(oracleKey)
	} else {
		_, oraclesBytes = v.AppTree.Get(oracleKey)
	}
	v.RUnlock()
	var oracles []string
	err := v.Codec.UnmarshalBinaryBare(oraclesBytes, &oracles)
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
	addr := pubKey.Address().String()
	v.RLock()
	_, validatorsBytes := v.AppTree.Get(validatorKey)
	v.RUnlock()
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

	validatorsBytes, err := v.Codec.MarshalBinaryBare(validators)
	if err != nil {
		return errors.New("cannot marshal validator")
	}
	v.Lock()
	v.AppTree.Set(validatorKey, validatorsBytes)
	v.Unlock()
	return nil
}

// RemoveValidator removes a tendermint validator if exists
func (v *State) RemoveValidator(address string) error {
	v.RLock()
	_, validatorsBytes := v.AppTree.Get(validatorKey)
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
			v.AppTree.Set(validatorKey, validatorsBytes)
			v.Unlock()
			return nil
		}
	}
	return errors.New("validator not found")
}

// Validators returns a list of the validators saved on persistent storage
func (v *State) Validators(isQuery bool) ([]tmtypes.GenesisValidator, error) {
	var validatorBytes []byte
	v.RLock()
	if isQuery {
		_, validatorBytes = v.IAppTree.Get(validatorKey)
	} else {
		_, validatorBytes = v.AppTree.Get(validatorKey)
	}
	v.RUnlock()
	var validators []tmtypes.GenesisValidator
	err := v.Codec.UnmarshalBinaryBare(validatorBytes, &validators)
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
func (v *State) AddProcess(p *types.Process, pid string) error {
	pid = util.TrimHex(pid)
	newProcessBytes, err := v.Codec.MarshalBinaryBare(p)
	if err != nil {
		return errors.New("cannot marshal process bytes")
	}
	v.Lock()
	v.ProcessTree.Set([]byte(pid), newProcessBytes)
	v.Unlock()
	for _, l := range v.eventListeners {
		l.OnProcess(pid, p.EntityID)
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
	v.ProcessTree.Set([]byte(pid), updatedProcessBytes)
	v.Unlock()
	for _, l := range v.eventListeners {
		l.OnCancel(pid)
	}
	return nil
}

// PauseProcess sets the process paused atribute to true
func (v *State) PauseProcess(pid string) error {
	pid = util.TrimHex(pid)
	v.RLock()
	_, processBytes := v.ProcessTree.Get([]byte(pid))
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
	v.RLock()
	_, processBytes := v.ProcessTree.Get([]byte(pid))
	v.RUnlock()
	var process types.Process
	if err := v.Codec.UnmarshalBinaryBare(processBytes, &process); err != nil {
		return errors.New("cannot unmarshal process")
	}
	// non paused process cannot be resumed
	if !process.Paused {
		return nil
	}
	process.Paused = false
	return v.setProcess(&process, pid)
}

// Process returns a process info given a processId if exists
func (v *State) Process(pid string, isQuery bool) (*types.Process, error) {
	var process *types.Process
	var processBytes []byte
	pid = util.TrimHex(pid)
	v.RLock()
	if isQuery {
		_, processBytes = v.IProcessTree.Get([]byte(pid))
	} else {
		_, processBytes = v.ProcessTree.Get([]byte(pid))
	}
	v.RUnlock()
	if processBytes == nil {
		return nil, fmt.Errorf("cannot find process with id (%s)", pid)
	}
	err := v.Codec.UnmarshalBinaryBare(processBytes, &process)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal process with id (%s)", pid)
	}
	return process, nil
}

// set process stores in the database the process
func (v *State) setProcess(process *types.Process, pid string) error {
	if process == nil {
		return fmt.Errorf("process is nil")
	}
	updatedProcessBytes, err := v.Codec.MarshalBinaryBare(process)
	if err != nil {
		return errors.New("cannot marshal updated process bytes")
	}
	v.Lock()
	v.ProcessTree.Set([]byte(pid), updatedProcessBytes)
	v.Unlock()
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
	v.VoteTree.Set([]byte(voteID), newVoteBytes)
	v.Unlock()
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
		_, voteBytes = v.IVoteTree.Get([]byte(voteID))
	} else {
		_, voteBytes = v.VoteTree.Get([]byte(voteID))
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
	return v.IVoteTree.Has([]byte(voteID))
}

// The prefix is "processID_", where processID is a stringified hash of fixed length.
// To iterate over all the keys with said prefix, the start point can simply be "processID_".
// We don't know what the next processID hash will be, so we use "processID}"
// as the end point, since "}" comes after "_" when sorting lexicographically.
func (v *State) iterateProcessID(processID string, fn func(key []byte, value []byte) bool, isQuery bool) bool {
	v.RLock()
	defer v.RUnlock()
	if isQuery {
		return v.IVoteTree.IterateRange([]byte(processID+"_"), []byte(processID+"}"), true, fn)
	}
	return v.VoteTree.IterateRange([]byte(processID+"_"), []byte(processID+"}"), true, fn)
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
		if idx >= listSize {
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
		_, headerBytes = v.IAppTree.Get(headerKey)
	} else {
		_, headerBytes = v.AppTree.Get(headerKey)
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
		_, headerBytes = v.IAppTree.Get(headerKey)
	} else {
		_, headerBytes = v.AppTree.Get(headerKey)
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
	h1, _, err := v.AppTree.SaveVersion()
	if err != nil {
		log.Errorf("cannot save vochain state to disk: %s", err)
	}
	h2, _, err := v.ProcessTree.SaveVersion()
	if err != nil {
		log.Errorf("cannot save vochain state to disk: %s", err)
	}
	h3, _, err := v.VoteTree.SaveVersion()
	if err != nil {
		log.Errorf("cannot save vochain state to disk: %s", err)
	}
	if err := v.Immutable(); err != nil {
		log.Errorf("cannot set immutable tree")
	}
	v.Unlock()
	if h := v.Header(false); h != nil {
		for _, l := range v.eventListeners {
			l.Commit(h.Height)
		}
	}

	return ethereum.HashRaw([]byte(fmt.Sprintf("%s%s%s", h1, h2, h3)))
}

// Rollback rollbacks to the last persistent db data version
func (v *State) Rollback() {
	for _, l := range v.eventListeners {
		l.Rollback()
	}
	v.Lock()
	v.AppTree.Rollback()
	v.ProcessTree.Rollback()
	v.VoteTree.Rollback()
	v.Unlock()
}

// WorkingHash returns the hash of the vochain trees mkroots
// hash(appTree+processTree+voteTree)
func (v *State) WorkingHash() []byte {
	v.RLock()
	appHash := v.AppTree.WorkingHash()
	procHash := v.ProcessTree.WorkingHash()
	voteHash := v.VoteTree.WorkingHash()
	v.RUnlock()
	return ethereum.HashRaw([]byte(fmt.Sprintf("%s%s%s", appHash, procHash, voteHash)))
}
