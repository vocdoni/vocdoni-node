package vochain

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/iavl"
	"github.com/tendermint/tendermint/crypto"
	tmtypes "github.com/tendermint/tendermint/types"
	tmdb "github.com/tendermint/tm-db"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	vochaintypes "gitlab.com/vocdoni/go-dvote/types"
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

// ValidEvents contains the list of valid event names
var ValidEvents = map[string]bool{"addVote": true, "addProcess": true, "commit": true, "rollback": true}

// VochainEvent is an interface used for executing custom functions during the events of the block creation process
// The events available can be read on ValidEvents string slice
// The order in which events are executed is: Rollback(), OnVote() or OnProcess(), Commit()
// The process is concurrencty safe meaning that there cannot be two sequences happening in paralel
type VochainEvent interface {
	OnVote(*types.Vote)
	OnProcess(pid, eid string)

	Commit()
	Rollback()
}

// State represents the state of the vochain application
type State struct {
	AppTree     *iavl.MutableTree
	ProcessTree *iavl.MutableTree
	VoteTree    *iavl.MutableTree
	Codec       *amino.Codec
	Lock        sync.Mutex
	Events      map[string][]VochainEvent // addVote, addProcess....
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
	vs := State{Codec: codec}
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

	log.Infof("application trees successfully loaded. appTree:%d processTree:%d voteTree: %d", atVersion, ptVersion, vtVersion)
	vs.Events = make(map[string][]VochainEvent)
	return &vs, nil
}

// AddEvent adds a new callback function of type EventCallback which will be exeuted on event name
func (v *State) AddEvent(name string, f VochainEvent) error {
	if _, ok := ValidEvents[name]; ok {
		v.Events[name] = append(v.Events[name], f)
		return nil
	}
	return fmt.Errorf("vochain event %s not valid", name)
}

// AddOracle adds a trusted oracle given its address if not exists
func (v *State) AddOracle(address string) error {
	address = util.TrimHex(address)
	_, oraclesBytes := v.AppTree.Get(oracleKey)
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
	v.AppTree.Set(oracleKey, newOraclesBytes)
	return nil
}

// RemoveOracle removes a trusted oracle given its address if exists
func (v *State) RemoveOracle(address string) error {
	address = util.TrimHex(address)
	_, oraclesBytes := v.AppTree.Get(oracleKey)
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
			v.AppTree.Set(oracleKey, newOraclesBytes)
			return nil
		}
	}
	return errors.New("oracle not found")
}

// Oracles returns the current oracle list
func (v *State) Oracles() ([]string, error) {
	_, oraclesBytes := v.AppTree.Get(oracleKey)
	var oracles []string
	err := v.Codec.UnmarshalBinaryBare(oraclesBytes, &oracles)
	return oracles, err
}

// AddValidator adds a tendemint validator if it is not already added
func (v *State) AddValidator(pubKey crypto.PubKey, power int64) error {
	addr := pubKey.Address().String()
	_, validatorsBytes := v.AppTree.Get(validatorKey)
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
	v.AppTree.Set(validatorKey, validatorsBytes)
	return nil
}

// RemoveValidator removes a tendermint validator if exists
func (v *State) RemoveValidator(address string) error {
	_, validatorsBytes := v.AppTree.Get(validatorKey)
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
			v.AppTree.Set(validatorKey, validatorsBytes)
			return nil
		}
	}
	return errors.New("validator not found")
}

// Validators returns a list of the validators saved on persistent storage
func (v *State) Validators() ([]tmtypes.GenesisValidator, error) {
	_, validatorBytes := v.AppTree.Get(validatorKey)
	var validators []tmtypes.GenesisValidator
	err := v.Codec.UnmarshalBinaryBare(validatorBytes, &validators)
	return validators, err
}

// AddProcess adds a new process to vochain if not already added
func (v *State) AddProcess(p *vochaintypes.Process, pid string) error {
	pid = util.TrimHex(pid)
	newProcessBytes, err := v.Codec.MarshalBinaryBare(p)
	if err != nil {
		return errors.New("cannot marshal process bytes")
	}
	v.ProcessTree.Set([]byte(pid), newProcessBytes)
	if events, ok := v.Events["addProcess"]; ok {
		for _, e := range events {
			e.OnProcess(pid, p.EntityID)
		}
	}
	return nil
}

// CancelProcess sets the process canceled atribute to true
func (v *State) CancelProcess(pid string) error {
	pid = util.TrimHex(pid)
	_, processBytes := v.ProcessTree.Get([]byte(pid))
	var process vochaintypes.Process
	if err := v.Codec.UnmarshalBinaryBare(processBytes, &process); err != nil {
		return errors.New("cannot unmarshal process")
	}
	if process.Canceled {
		return nil
	}
	process.Canceled = true
	updatedProcessBytes, err := v.Codec.MarshalBinaryBare(process)
	if err != nil {
		return errors.New("cannot marshal updated process bytes")
	}
	v.ProcessTree.Set([]byte(pid), updatedProcessBytes)
	return nil
}

// PauseProcess sets the process paused atribute to true
func (v *State) PauseProcess(pid string) error {
	pid = util.TrimHex(pid)
	_, processBytes := v.ProcessTree.Get([]byte(pid))
	var process vochaintypes.Process
	if err := v.Codec.UnmarshalBinaryBare(processBytes, &process); err != nil {
		return errors.New("cannot unmarshal process")
	}
	// already paused
	if process.Paused {
		return nil
	}
	process.Paused = true
	updatedProcessBytes, err := v.Codec.MarshalBinaryBare(process)
	if err != nil {
		return errors.New("cannot marshal updated process bytes")
	}
	v.ProcessTree.Set([]byte(pid), updatedProcessBytes)
	return nil
}

// ResumeProcess sets the process paused atribute to true
func (v *State) ResumeProcess(pid string) error {
	pid = util.TrimHex(pid)
	_, processBytes := v.ProcessTree.Get([]byte(pid))
	var process vochaintypes.Process
	if err := v.Codec.UnmarshalBinaryBare(processBytes, &process); err != nil {
		return errors.New("cannot unmarshal process")
	}
	// non paused process cannot be resumed
	if !process.Paused {
		return nil
	}
	process.Paused = false
	updatedProcessBytes, err := v.Codec.MarshalBinaryBare(process)
	if err != nil {
		return errors.New("cannot marshal updated process bytes")
	}
	v.ProcessTree.Set([]byte(pid), updatedProcessBytes)
	return nil
}

// Process returns a process info given a processId if exists
func (v *State) Process(pid string) (*vochaintypes.Process, error) {
	var newProcess *vochaintypes.Process
	pid = util.TrimHex(pid)
	if !v.ProcessTree.Has([]byte(pid)) {
		return nil, fmt.Errorf("cannot find process with id (%s)", pid)
	}
	_, processBytes := v.ProcessTree.Get([]byte(pid))
	if len(processBytes) == 0 {
		return nil, fmt.Errorf("cannot find process with id (%s)", pid)
	}
	err := v.Codec.UnmarshalBinaryBare(processBytes, &newProcess)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal process with id (%s)", pid)
	}
	return newProcess, nil
}

// AddVote adds a new vote to a process if the process exists and the vote is not already submmited
func (v *State) AddVote(vote *vochaintypes.Vote) error {
	voteID := fmt.Sprintf("%s_%s", util.TrimHex(vote.ProcessID), util.TrimHex(vote.Nullifier))
	newVoteBytes, err := v.Codec.MarshalBinaryBare(vote)
	if err != nil {
		return errors.New("cannot marshal vote")
	}
	v.VoteTree.Set([]byte(voteID), newVoteBytes)
	if events, ok := v.Events["addVote"]; ok {
		for _, e := range events {
			e.OnVote(vote)
		}
	}
	return nil
}

// Envelope returns the info of a vote if already exists
func (v *State) Envelope(voteID string) (*vochaintypes.Vote, error) {
	var vote *vochaintypes.Vote
	voteID = util.TrimHex(voteID)
	if !v.VoteTree.Has([]byte(voteID)) {
		return nil, fmt.Errorf("vote with id (%s) does not exist", voteID)
	}
	_, voteBytes := v.VoteTree.Get([]byte(voteID))
	if len(voteBytes) == 0 {
		return nil, fmt.Errorf("vote with id (%s) does not exist", voteID)
	}
	// log.Debugf("get envelope votebytes: %b", voteBytes)
	err := v.Codec.UnmarshalBinaryBare(voteBytes, &vote)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote with id (%s)", voteID)
	}
	// log.Debugf("get envelope value: %+v", vote)
	return vote, nil
}

// The prefix is "processID_", where processID is a stringified hash of fixed length.
// To iterate over all the keys with said prefix, the start point can simply be "processID_".
// We don't know what the next processID hash will be, so we use "processID}"
// as the end point, since "}" comes after "_" when sorting lexicographically.
func (v *State) iterateProcessID(processID string, fn func(key []byte, value []byte) bool) bool {
	return v.VoteTree.IterateRange([]byte(processID+"_"), []byte(processID+"}"), true, fn)
}

// CountVotes returns the number of votes registered for a given process id
func (v *State) CountVotes(processID string) int64 {
	processID = util.TrimHex(processID)
	var count int64
	fn := func(key []byte, value []byte) bool {
		count++
		return false
	}
	v.iterateProcessID(processID, fn)
	return count
}

// EnvelopeList returns a list of registered envelopes nullifiers given a processId
func (v *State) EnvelopeList(processID string, from, listSize int64) []string {
	processID = util.TrimHex(processID)
	var nullifiers []string
	idx := int64(0)
	fn := func(key []byte, value []byte) bool {
		if idx >= listSize {
			return true
		}
		if idx >= from {
			k := strings.Split(string(key), "_")
			nullifiers = append(nullifiers, k[1])
		}
		idx++
		return false
	}
	v.iterateProcessID(processID, fn)
	return nullifiers
}

// Height returns the blockchain last block commited height
func (v *State) Height() int64 {
	_, headerBytes := v.AppTree.Get(headerKey)
	var header tmtypes.Header
	err := v.Codec.UnmarshalBinaryBare(headerBytes, &header)
	if err != nil {
		log.Errorf("cannot get vochain height: %s", err)
		return 0
	}
	return header.Height
}

// AppHash returns last hash of the application
func (v *State) AppHash() []byte {
	_, headerBytes := v.AppTree.Get(headerKey)
	var header tmtypes.Header
	err := v.Codec.UnmarshalBinaryBare(headerBytes, &header)
	if err != nil {
		return []byte{}
	}
	return header.AppHash
}

// Save persistent save of vochain mem trees
func (v *State) Save() []byte {
	if events, ok := v.Events["commit"]; ok {
		for _, e := range events {
			e.Commit()
		}
	}
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

	return signature.HashRaw(fmt.Sprintf("%s%s%s", h1, h2, h3))
}

// Rollback rollbacks to the last persistent db data version
func (v *State) Rollback() {
	if events, ok := v.Events["rollback"]; ok {
		for _, e := range events {
			e.Rollback()
		}
	}
	v.AppTree.Rollback()
	v.ProcessTree.Rollback()
	v.VoteTree.Rollback()
}

// WorkingHash returns the hash of the vochain trees mkroots
// hash(appTree+processTree+voteTree)
func (v *State) WorkingHash() []byte {
	return signature.HashRaw(fmt.Sprintf("%s%s%s", v.AppTree.WorkingHash(), v.ProcessTree.WorkingHash(), v.VoteTree.WorkingHash()))
}

// ProcessList returns a list of processId given an entityId
// func (v *State) ProcessList(entityId string) []Process

// EnvelopeList returns a list of envelopes given a processId
// func (v *State) EnvelopeList(processId string) []Vote
