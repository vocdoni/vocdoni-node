package vochain

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ed25519 "github.com/tendermint/tendermint/crypto/ed25519"
	models "github.com/vocdoni/dvote-protobuf/build/go/models"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/statedb"
	"gitlab.com/vocdoni/go-dvote/statedb/iavlstate"
	"gitlab.com/vocdoni/go-dvote/types"
	"google.golang.org/protobuf/proto"
)

const (
	// db names
	AppTree                 = "app"
	ProcessTree             = "process"
	VoteTree                = "vote"
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
	OnVote(*models.Vote)
	OnProcess(pid, eid []byte, mkroot, mkuri string)
	OnCancel(pid []byte)
	OnProcessKeys(pid []byte, encryptionPub, commitment string)
	OnRevealKeys(pid []byte, encryptionPriv, reveal string)
	Commit(height int64)
	Rollback()
}

// State represents the state of the vochain application
type State struct {
	Store         statedb.StateDB
	voteCache     map[string]*types.VoteProof
	voteCacheLock sync.RWMutex
	ImmutableState

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
func NewState(dataDir string) (*State, error) {
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
func (v *State) AddOracle(address common.Address) error {
	var err error
	v.Lock()
	defer v.Unlock()
	oraclesBytes := v.Store.Tree(AppTree).Get(oracleKey)
	var oracleList models.OracleList
	if len(oraclesBytes) > 0 {
		if err = proto.Unmarshal(oraclesBytes, &oracleList); err != nil {
			return err
		}
	}
	oracles := oracleList.GetOracles()
	for _, v := range oracles {
		if bytes.Equal(v, address.Bytes()) {
			return errors.New("oracle already added")
		}
	}
	oracles = append(oracles, address.Bytes())
	oracleList.Oracles = oracles
	newOraclesBytes, err := proto.Marshal(&oracleList)
	if err != nil {
		return fmt.Errorf("cannot marshal oracles: (%s)", err)
	}
	return v.Store.Tree(AppTree).Add(oracleKey, newOraclesBytes)
}

// RemoveOracle removes a trusted oracle given its address if exists
func (v *State) RemoveOracle(address common.Address) error {
	v.Lock()
	defer v.Unlock()
	oraclesBytes := v.Store.Tree(AppTree).Get(oracleKey)
	var oracleList models.OracleList

	if err := proto.Unmarshal(oraclesBytes, &oracleList); err != nil {
		return err
	}
	oracles := oracleList.GetOracles()

	for i, o := range oracles {
		if bytes.Equal(o, address.Bytes()) {
			// remove oracle
			copy(oracles[i:], oracles[i+1:])
			oracles[len(oracles)-1] = []byte{}
			oracles = oracles[:len(oracles)-1]
			oracleList.Oracles = oracles
			newOraclesBytes, err := proto.Marshal(&oracleList)
			if err != nil {
				return fmt.Errorf("cannot marshal oracles: (%s)", err)
			}
			return v.Store.Tree(AppTree).Add(oracleKey, newOraclesBytes)
		}
	}
	return fmt.Errorf("oracle not found")
}

// Oracles returns the current oracle list
func (v *State) Oracles(isQuery bool) ([]common.Address, error) {
	var oraclesBytes []byte
	var err error
	v.RLock()
	defer v.RUnlock()
	if isQuery {
		oraclesBytes = v.Store.ImmutableTree(AppTree).Get(oracleKey)
	} else {
		oraclesBytes = v.Store.Tree(AppTree).Get(oracleKey)
	}
	var oracleList models.OracleList
	if err = proto.Unmarshal(oraclesBytes, &oracleList); err != nil {
		return nil, err
	}
	var oracles []common.Address
	for _, o := range oracleList.GetOracles() {
		oracles = append(oracles, common.BytesToAddress(o))
	}
	return oracles, err
}

func hexPubKeyToTendermintEd25519(pubKey string) (types.PubKey, error) {
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
func (v *State) AddValidator(validator *models.Validator) error {
	var err error
	v.Lock()
	defer v.Unlock()
	validatorsBytes := v.Store.Tree(AppTree).Get(validatorKey)
	var validatorsList models.ValidatorList
	if len(validatorsBytes) > 0 {
		if err = proto.Unmarshal(validatorsBytes, &validatorsList); err != nil {
			return err
		}
	}
	for _, v := range validatorsList.Validators {
		if bytes.Equal(v.Address, validator.Address) {
			return nil
		}
	}
	newVal := &models.Validator{
		Address: validator.GetAddress(),
		PubKey:  validator.GetPubKey(),
		Power:   validator.GetPower(),
	}
	validatorsList.Validators = append(validatorsList.Validators, newVal)
	validatorsBytes, err = proto.Marshal(&validatorsList)
	if err != nil {
		return fmt.Errorf("cannot marshal validators: %v", err)
	}
	return v.Store.Tree(AppTree).Add(validatorKey, validatorsBytes)
}

// RemoveValidator removes a tendermint validator identified by its address
func (v *State) RemoveValidator(address []byte) error {
	v.RLock()
	validatorsBytes := v.Store.Tree(AppTree).Get(validatorKey)
	v.RUnlock()
	var validators models.ValidatorList
	if err := proto.Unmarshal(validatorsBytes, &validators); err != nil {
		return err
	}
	for i, val := range validators.Validators {
		if bytes.Equal(val.Address, address) {
			// remove validator
			copy(validators.Validators[i:], validators.Validators[i+1:])
			validators.Validators[len(validators.Validators)-1] = &models.Validator{}
			validators.Validators = validators.Validators[:len(validators.Validators)-1]
			validatorsBytes, err := proto.Marshal(&validators)
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
func (v *State) Validators(isQuery bool) ([]*models.Validator, error) {
	var validatorBytes []byte
	var err error
	v.RLock()
	if isQuery {
		validatorBytes = v.Store.ImmutableTree(AppTree).Get(validatorKey)
	} else {
		validatorBytes = v.Store.Tree(AppTree).Get(validatorKey)
	}
	v.RUnlock()
	var validators models.ValidatorList
	err = proto.Unmarshal(validatorBytes, &validators)
	return validators.Validators, err
}

// AddProcessKeys adds the keys to the process
func (v *State) AddProcessKeys(tx *models.AdminTx) error {
	if tx.ProcessId == nil || tx.KeyIndex == nil {
		return fmt.Errorf("no processId or keyIndex provided on AddProcessKeys")
	}
	process, err := v.Process(tx.ProcessId, false)
	if err != nil {
		return err
	}
	if tx.CommitmentKey != nil {
		process.CommitmentKeys[*tx.KeyIndex] = fmt.Sprintf("%x", tx.CommitmentKey)
		log.Debugf("added commitment key %d for process %x: %x", *tx.KeyIndex, tx.ProcessId, tx.CommitmentKey)
	}
	if tx.EncryptionPublicKey != nil {
		process.EncryptionPublicKeys[*tx.KeyIndex] = fmt.Sprintf("%x", tx.EncryptionPublicKey)
		log.Debugf("added encryption key %d for process %x: %x", *tx.KeyIndex, tx.ProcessId, tx.EncryptionPublicKey)
	}
	if process.KeyIndex == nil {
		process.KeyIndex = new(uint32)
	}
	*process.KeyIndex++
	if err := v.setProcess(process, tx.ProcessId); err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnProcessKeys(tx.ProcessId, fmt.Sprintf("%x", tx.EncryptionPublicKey), fmt.Sprintf("%x", tx.CommitmentKey))
	}
	return nil
}

// RevealProcessKeys reveals the keys of a process
func (v *State) RevealProcessKeys(tx *models.AdminTx) error {
	if tx.ProcessId == nil || tx.KeyIndex == nil {
		return fmt.Errorf("no processId or keyIndex provided on AddProcessKeys")
	}
	process, err := v.Process(tx.ProcessId, false)
	if err != nil {
		return err
	}
	if process.KeyIndex == nil || *process.KeyIndex < 1 {
		return fmt.Errorf("no keys to reveal, keyIndex is < 1")
	}
	rkey := ""
	if tx.RevealKey != nil {
		rkey = fmt.Sprintf("%x", tx.RevealKey)
		process.RevealKeys[*tx.KeyIndex] = rkey // TBD: Change hex strings for []byte
		log.Debugf("revealed commitment key %d for process %x: %x", *tx.KeyIndex, tx.ProcessId, tx.RevealKey)
	}
	ekey := ""
	if tx.EncryptionPrivateKey != nil {
		ekey = fmt.Sprintf("%x", tx.EncryptionPrivateKey)
		process.EncryptionPrivateKeys[*tx.KeyIndex] = ekey
		log.Debugf("revealed encryption key %d for process %x: %x", *tx.KeyIndex, tx.ProcessId, tx.EncryptionPrivateKey)
	}
	*process.KeyIndex--
	if err := v.setProcess(process, tx.ProcessId); err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnRevealKeys(tx.ProcessId, ekey, rkey)
	}
	return nil
}

// AddProcess adds a new process to vochain
func (v *State) AddProcess(p *models.Process, pid []byte) error {
	newProcessBytes, err := proto.Marshal(p)
	if err != nil {
		return errors.New("cannot marshal process bytes")
	}
	v.Lock()
	err = v.Store.Tree(ProcessTree).Add(pid, newProcessBytes)
	v.Unlock()
	if err != nil {
		return err
	}
	mkuri := ""
	if p.CensusMkURI != nil {
		mkuri = *p.CensusMkURI
	}
	for _, l := range v.eventListeners {
		l.OnProcess(pid, p.EntityId, fmt.Sprintf("%x", p.CensusMkRoot), mkuri)
	}
	return nil
}

// CancelProcess sets the process canceled atribute to true
func (v *State) CancelProcess(pid []byte) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	if process.Status == models.ProcessStatus_CANCELED {
		return nil
	}
	process.Status = models.ProcessStatus_CANCELED
	updatedProcessBytes, err := proto.Marshal(process)
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
func (v *State) PauseProcess(pid []byte) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	// already paused
	if process.Status == models.ProcessStatus_PAUSED {
		return fmt.Errorf("process already paused")
	}
	process.Status = models.ProcessStatus_PAUSED
	return v.setProcess(process, pid)
}

// ResumeProcess sets the process paused atribute to true
func (v *State) ResumeProcess(pid []byte) error {
	process, err := v.Process(pid, false)
	if err != nil {
		return err
	}
	// non paused process cannot be resumed
	if process.Status != models.ProcessStatus_PAUSED {
		return fmt.Errorf("process is not paused")
	}
	process.Status = models.ProcessStatus_READY
	return v.setProcess(process, pid)
}

// Process returns a process info given a processId if exists
func (v *State) Process(pid []byte, isQuery bool) (*models.Process, error) {
	var processBytes []byte
	var err error
	v.RLock()
	if isQuery {
		processBytes = v.Store.ImmutableTree(ProcessTree).Get(pid)
	} else {
		processBytes = v.Store.Tree(ProcessTree).Get(pid)
	}
	v.RUnlock()
	if processBytes == nil {
		return nil, ErrProcessNotFound
	}
	process := new(models.Process)
	err = proto.Unmarshal(processBytes, process)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal process (%s): %w", pid, err)
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
func (v *State) setProcess(process *models.Process, pid []byte) error {
	if process == nil || len(process.ProcessId) != types.ProcessIDsize {
		return ErrProcessNotFound
	}
	updatedProcessBytes, err := proto.Marshal(process)
	if err != nil {
		return errors.New("cannot marshal updated process bytes")
	}
	v.Lock()
	defer v.Unlock()
	if err := v.Store.Tree(ProcessTree).Add(pid, updatedProcessBytes); err != nil {
		return err
	}
	return nil
}

// AddVote adds a new vote to a process if the process exists and the vote is not already submmited
func (v *State) AddVote(vote *models.Vote) error {
	vid, err := v.voteID(vote.ProcessId, vote.Nullifier)
	if err != nil {
		return err
	}
	// save block number
	vote.Height = uint32(v.Header(false).Height)
	newVoteBytes, err := proto.Marshal(vote)
	if err != nil {
		return fmt.Errorf("cannot marshal vote")
	}
	v.Lock()
	err = v.Store.Tree(VoteTree).Add(vid, newVoteBytes)
	v.Unlock()
	if err != nil {
		return err
	}
	for _, l := range v.eventListeners {
		l.OnVote(vote)
	}
	return nil
}

// voteID = byte( processID+nullifier )
func (v *State) voteID(pid, nullifier []byte) ([]byte, error) {
	if len(pid) != types.ProcessIDsize {
		return nil, fmt.Errorf("wrong processID size %d", len(pid))
	}
	if len(nullifier) != types.VoteNullifierSize {
		return nil, fmt.Errorf("wrong nullifier size %d", len(nullifier))
	}
	vid := make([]byte, 0, len(pid)+len(nullifier))
	vid = append(vid, pid...)
	vid = append(vid, nullifier...)
	return vid, nil
}

// Envelope returns the info of a vote if already exists.
// voteID must be equals to processID_Nullifier
func (v *State) Envelope(processID, nullifier []byte, isQuery bool) (_ *models.Vote, err error) {
	// TODO(mvdan): remove the recover once
	// https://github.com/tendermint/iavl/issues/212 is fixed
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered panic: %v", r)
		}
	}()

	vote := new(models.Vote)
	var voteBytes []byte
	vid, err := v.voteID(processID, nullifier)
	if err != nil {
		return nil, err
	}
	v.RLock()
	defer v.RUnlock() // needs to be deferred due to the recover above
	if isQuery {
		voteBytes = v.Store.ImmutableTree(VoteTree).Get(vid)
	} else {
		voteBytes = v.Store.Tree(VoteTree).Get(vid)
	}
	if voteBytes == nil {
		return nil, fmt.Errorf("vote with id (%x) does not exist", vid)
	}
	if err := proto.Unmarshal(voteBytes, vote); err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote with id (%x)", vid)
	}
	return vote, nil
}

// EnvelopeExists returns true if the envelope identified with voteID exists
func (v *State) EnvelopeExists(processID, nullifier []byte) bool {
	voteID, err := v.voteID(processID, nullifier)
	if err != nil {
		return false
	}
	v.RLock()
	defer v.RUnlock()
	b := v.Store.ImmutableTree(VoteTree).Get(voteID)
	return len(b) > 0
}

func (v *State) iterateProcessID(processID []byte, fn func(key []byte, value []byte) bool, isQuery bool) bool {
	v.RLock()
	defer v.RUnlock()
	if isQuery {
		v.Store.ImmutableTree(VoteTree).Iterate(processID, fn)
	} else {
		v.Store.Tree(VoteTree).Iterate(processID, fn)
	}
	return true
}

// CountVotes returns the number of votes registered for a given process id
func (v *State) CountVotes(processID []byte, isQuery bool) uint32 {
	var count uint32
	v.iterateProcessID(processID, func(key []byte, value []byte) bool {
		count++
		return false
	}, isQuery)
	return count
}

// EnvelopeList returns a list of registered envelopes nullifiers given a processId
func (v *State) EnvelopeList(processID []byte, from, listSize int64, isQuery bool) (nullifiers [][]byte) {
	// TODO(mvdan): remove the recover once
	// https://github.com/tendermint/iavl/issues/212 is fixed
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered panic: %v", r)
			// TODO(mvdan): this func should return an error instead
			// err = fmt.Errorf("recovered panic: %v", r)
		}
	}()
	idx := int64(0)
	v.iterateProcessID(processID, func(key []byte, value []byte) bool {
		if idx >= from+listSize {
			return true
		}
		if idx >= from {
			nullifiers = append(nullifiers, key[32:])
		}
		idx++
		return false
	}, isQuery)
	return nullifiers
}

// Header returns the blockchain last block commited height
func (v *State) Header(isQuery bool) *models.TendermintHeader {
	var headerBytes []byte
	v.RLock()
	if isQuery {
		headerBytes = v.Store.ImmutableTree(AppTree).Get(headerKey)
	} else {
		headerBytes = v.Store.Tree(AppTree).Get(headerKey)
	}
	v.RUnlock()
	var header models.TendermintHeader
	err := proto.Unmarshal(headerBytes, &header)
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
	var header models.TendermintHeader
	err := proto.Unmarshal(headerBytes, &header)
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
