package vochain

import (
	"errors"
	"fmt"
	"strings"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/iavl"
	tmtypes "github.com/tendermint/tendermint/abci/types"
	tmdb "github.com/tendermint/tm-db"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	vochaintypes "gitlab.com/vocdoni/go-dvote/types"
)

const (
	// db names
	appTreeName     = "appTree"
	processTreeName = "processTree"
	voteTreeName    = "voteTree"
	// keys
	headerKey    = "header"
	oracleKey    = "oracle"
	validatorKey = "validator"
	processKey   = "process"
	voteKey      = "vote"
	// validators default power
	validatorPower = 10
)

// PrefixDBCacheSize is the size of the cache for the MutableTree IAVL databases
var PrefixDBCacheSize = 0

// VochainState represents the state of the vochain application
type VochainState struct {
	AppTree     *iavl.MutableTree
	ProcessTree *iavl.MutableTree
	VoteTree    *iavl.MutableTree
	Codec       *amino.Codec
	Lock        bool
}

// NewVochainState creates a new VochainState
func NewVochainState(dataDir string, codec *amino.Codec) (*VochainState, error) {
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
	var vs = VochainState{
		AppTree:     iavl.NewMutableTree(appTree, PrefixDBCacheSize),
		ProcessTree: iavl.NewMutableTree(processTree, PrefixDBCacheSize),
		VoteTree:    iavl.NewMutableTree(voteTree, PrefixDBCacheSize),
		Codec:       codec,
	}
	vs.AppTree.Load()
	vs.ProcessTree.Load()
	vs.VoteTree.Load()
	return &vs, err
}

// AddOracle adds a trusted oracle given its address if not exists
func (v *VochainState) AddOracle(address string) error {
	address = signature.TrimHex(address)
	_, oraclesBytes := v.AppTree.Get([]byte(oracleKey))
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
	v.AppTree.Set([]byte(oracleKey), newOraclesBytes)
	return nil
}

// RemoveOracle removes a trusted oracle given its address if exists
func (v *VochainState) RemoveOracle(address string) error {
	address = signature.TrimHex(address)
	_, oraclesBytes := v.AppTree.Get([]byte(oracleKey))
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
			v.AppTree.Set([]byte(oracleKey), newOraclesBytes)
			return nil
		}
	}
	return errors.New("oracle not found")
}

// Oracles returns the current oracle list
func (v *VochainState) Oracles() ([]string, error) {
	_, oraclesBytes := v.AppTree.Get([]byte(oracleKey))
	var oracles []string
	err := v.Codec.UnmarshalBinaryBare(oraclesBytes, &oracles)
	return oracles, err
}

// AddValidator adds a tendemint validator if it is not already added
func (v *VochainState) AddValidator(pubKey string, power int64) error {
	pubKey = signature.TrimHex(pubKey)
	_, validatorsBytes := v.AppTree.Get([]byte(validatorKey))
	var validators []vochaintypes.Validator
	v.Codec.UnmarshalBinaryBare(validatorsBytes, &validators)
	for _, v := range validators {
		if v.PubKey.Value == pubKey {
			return errors.New("validator already added")
		}
	}
	newVal := vochaintypes.Validator{
		Address: GenerateAddressFromEd25519PublicKeyString(pubKey),
		PubKey: vochaintypes.PubKey{
			Type:  "tendermint/PubKeyEd25519",
			Value: pubKey,
		},
		Power: validatorPower,
		Name:  "",
	}
	validators = append(validators, newVal)

	validatorsBytes, err := v.Codec.MarshalBinaryBare(validators)
	if err != nil {
		return errors.New("cannot marshal validator")
	}
	v.AppTree.Set([]byte(validatorKey), validatorsBytes)
	return nil
}

// RemoveValidator removes a tendermint validator if exists
func (v *VochainState) RemoveValidator(address string) error {
	address = signature.TrimHex(address)
	_, validatorsBytes := v.AppTree.Get([]byte(validatorKey))
	var validators []vochaintypes.Validator
	v.Codec.UnmarshalBinaryBare(validatorsBytes, &validators)
	for i, val := range validators {
		if val.Address == address {
			// remove validator
			copy(validators[i:], validators[i+1:])
			validators[len(validators)-1] = vochaintypes.Validator{}
			validators = validators[:len(validators)-1]
			validatorsBytes, err := v.Codec.MarshalBinaryBare(validators)
			if err != nil {
				return errors.New("cannot marshal validators")
			}
			v.AppTree.Set([]byte(validatorKey), validatorsBytes)
			return nil
		}
	}
	return errors.New("validator not found")
}

// Validators returns a list of the validators saved on persistent storage
func (v *VochainState) Validators() ([]vochaintypes.Validator, error) {
	_, validatorBytes := v.AppTree.Get([]byte(validatorKey))
	var validators []vochaintypes.Validator
	err := v.Codec.UnmarshalBinaryBare(validatorBytes, &validators)
	return validators, err
}

// AddProcess adds a new process to vochain if not already added
func (v *VochainState) AddProcess(p *vochaintypes.Process, pid string) error {
	pid = signature.TrimHex(pid)
	newProcessBytes, err := v.Codec.MarshalBinaryBare(p)
	if err != nil {
		return errors.New("cannot marshal process bytes")
	}
	v.ProcessTree.Set([]byte(pid), newProcessBytes)
	return nil
}

// Process returns a process info given a processId if exists
func (v *VochainState) Process(pid string) (*vochaintypes.Process, error) {
	var newProcess *vochaintypes.Process
	pid = signature.TrimHex(pid)
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
func (v *VochainState) AddVote(vote *vochaintypes.Vote) error {
	voteID := fmt.Sprintf("%s_%s", signature.TrimHex(vote.ProcessID), signature.TrimHex(vote.Nullifier))
	newVoteBytes, err := v.Codec.MarshalBinaryBare(vote)
	if err != nil {
		return errors.New("cannot marshal vote")
	}
	v.VoteTree.Set([]byte(voteID), newVoteBytes)
	return nil
}

// Envelope returns the info of a vote if already exists
func (v *VochainState) Envelope(voteID string) (*vochaintypes.Vote, error) {
	var vote *vochaintypes.Vote
	voteID = signature.TrimHex(voteID)
	if !v.VoteTree.Has([]byte(voteID)) {
		return nil, fmt.Errorf("vote with id (%s) does not exists", voteID)
	}
	_, voteBytes := v.VoteTree.Get([]byte(voteID))
	if len(voteBytes) == 0 {
		return nil, fmt.Errorf("vote with id (%s) does not exists", voteID)
	}
	// log.Debugf("get envelope votebytes: %b", voteBytes)
	err := v.Codec.UnmarshalBinaryBare(voteBytes, &vote)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote with id (%s)", voteID)
	}
	// log.Debugf("get envelope value: %+v", vote)
	return vote, nil
}

// CountVotes returns the number of votes registered for a given process id
/*
func (v *VochainState) CountVotes(processID string) int64 {
	processID = signature.TrimHex(processID)
	var count int64
	v.VoteTree.IterateRange([]byte(processID), []byte{}, true, func(key []byte, value []byte) bool {
		k := strings.Split(string(key), "_")
		if k[0] == processID {
			count++
		}
		return false
	})
	return count
}
*/
func (v *VochainState) CountVotes(processID string) int64 {
	processID = signature.TrimHex(processID)
	var count int64
	v.VoteTree.IterateRange([]byte(processID), nil, true, func(key []byte, value []byte) bool {
		count++
		return false
	})
	return count
}

// EnvelopeList returns a list of registered envelopes nullifiers given a processId
func (v *VochainState) EnvelopeList(processID string, from, listSize int64) []string {
	processID = signature.TrimHex(processID)
	var nullifiers []string
	idx := int64(0)
	v.VoteTree.IterateRange([]byte(processID), nil, true, func(key []byte, value []byte) bool {
		if idx >= listSize {
			return true
		}
		if idx >= from {
			k := strings.Split(string(key), "_")
			nullifiers = append(nullifiers, k[1])
		}
		idx++
		return false
	})
	return nullifiers
}

// Height returns the blockchain last block commited height
func (v *VochainState) Height() int64 {
	_, headerBytes := v.AppTree.Get([]byte(headerKey))
	var header tmtypes.Header
	err := v.Codec.UnmarshalBinaryBare(headerBytes, &header)
	if err != nil {
		log.Errorf("cannot get vochain height: %s", err)
		return 0
	}
	return header.Height
}

// AppHash returns last hash of the application
func (v *VochainState) AppHash() []byte {
	_, headerBytes := v.AppTree.Get([]byte(headerKey))
	var header tmtypes.Header
	err := v.Codec.UnmarshalBinaryBare(headerBytes, &header)
	if err != nil {
		return []byte{}
	}
	return header.AppHash
}

// Save persistent save of vochain mem trees
func (v *VochainState) Save() []byte {
	h1, _, err := v.AppTree.SaveVersion()
	if err != nil {
		log.Errorf("cannot sve vochain state to disk: %s", err)
	}
	h2, _, err := v.ProcessTree.SaveVersion()
	if err != nil {
		log.Errorf("cannot sve vochain state to disk: %s", err)
	}
	h3, _, err := v.VoteTree.SaveVersion()
	if err != nil {
		log.Errorf("cannot sve vochain state to disk: %s", err)
	}

	return signature.HashRaw(fmt.Sprintf("%s%s%s", h1, h2, h3))
}

// Rollback rollbacks to the last persistent db data version
func (v *VochainState) Rollback() {
	v.AppTree.Rollback()
	v.ProcessTree.Rollback()
	v.VoteTree.Rollback()
}

// Hash returns the hash of the vochain trees mkroots
// hash(appTree+processTree+voteTree)
func (v *VochainState) WorkingHash() []byte {
	return signature.HashRaw(fmt.Sprintf("%s%s%s", v.AppTree.WorkingHash(), v.ProcessTree.WorkingHash(), v.VoteTree.WorkingHash()))
}

// ProcessList returns a list of processId given an entityId
// func (v *VochainState) ProcessList(entityId string) []Process

// EnvelopeList returns a list of envelopes given a processId
// func (v *VochainState) EnvelopeList(processId string) []Vote
