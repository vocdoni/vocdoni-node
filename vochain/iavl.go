package vochain

import (
	"errors"
	"fmt"
	"strings"

	amino "github.com/tendermint/go-amino"
	iavl "github.com/tendermint/iavl"
	tmtypes "github.com/tendermint/tendermint/abci/types"
	tmdb "github.com/tendermint/tm-db"
	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
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

	// set keys
	appTree.Set([]byte(headerKey), []byte{})
	appTree.Set([]byte(oracleKey), []byte{})
	appTree.Set([]byte(validatorKey), []byte{})
	appTree.Set([]byte(processKey), []byte{}) // iavl process tree mkroot
	appTree.Set([]byte(voteKey), []byte{})    // iavl vote tree mkroot

	processTree, err := tmdb.NewGoLevelDB(processTreeName, dataDir)
	if err != nil {
		return nil, err
	}
	voteTree, err := tmdb.NewGoLevelDB(voteTreeName, dataDir)
	if err != nil {
		return nil, err
	}
	return &VochainState{
		AppTree:     iavl.NewMutableTree(appTree, PrefixDBCacheSize),
		ProcessTree: iavl.NewMutableTree(processTree, PrefixDBCacheSize),
		VoteTree:    iavl.NewMutableTree(voteTree, PrefixDBCacheSize),
		Codec:       codec,
	}, nil
}

// AddOracle adds a trusted oracle given its address if not exists
func (v *VochainState) AddOracle(address string) error {
	address = sanitizeHex(address)
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
	address = sanitizeHex(address)
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

// GetOracles returns the current oracle list
func (v *VochainState) GetOracles() ([]string, error) {
	_, oraclesBytes := v.AppTree.Get([]byte(oracleKey))
	var oracles []string
	err := v.Codec.UnmarshalBinaryBare(oraclesBytes, &oracles)
	return oracles, err
}

// AddValidator adds a tendemint validator if it is not already added
func (v *VochainState) AddValidator(pubKey string, power int64) error {
	pubKey = sanitizeHex(pubKey)
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
	address = sanitizeHex(address)
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

// GetValidators returns a list of the validators saved on persistent storage
func (v *VochainState) GetValidators() ([]vochaintypes.Validator, error) {
	_, validatorBytes := v.AppTree.Get([]byte(validatorKey))
	var validators []vochaintypes.Validator
	err := v.Codec.UnmarshalBinaryBare(validatorBytes, &validators)
	return validators, err
}

// AddProcess adds a new process to vochain if not already added
func (v *VochainState) AddProcess(p *vochaintypes.Process, pid string) error {
	pid = sanitizeHex(pid)
	newProcessBytes, err := v.Codec.MarshalBinaryBare(p)
	if err != nil {
		return errors.New("cannot marshal process bytes")
	}
	v.ProcessTree.Set([]byte(pid), newProcessBytes)
	return nil
}

// GetProcess returns a process info given a processId if exists
func (v *VochainState) GetProcess(pid string) (*vochaintypes.Process, error) {
	var newProcess *vochaintypes.Process
	pid = sanitizeHex(pid)
	if !v.ProcessTree.Has([]byte(pid)) {
		return nil, fmt.Errorf("cannot find process with id (%s)", pid)
	}
	_, processBytes := v.ProcessTree.Get([]byte(pid))
	if processBytes == nil {
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
	voteID := fmt.Sprintf("%s_%s", sanitizeHex(vote.ProcessID), sanitizeHex(vote.Nullifier))
	newVoteBytes, err := v.Codec.MarshalBinaryBare(vote)
	if err != nil {
		return errors.New("cannot marshal vote")
	}
	v.VoteTree.Set([]byte(voteID), newVoteBytes)
	return nil
}

// GetEnvelope returns the info of a vote if already exists
func (v *VochainState) GetEnvelope(voteID string) (*vochaintypes.Vote, error) {
	var vote *vochaintypes.Vote
	voteID = sanitizeHex(voteID)
	if !v.VoteTree.Has([]byte(voteID)) {
		return nil, fmt.Errorf("vote with id (%s) does not exists", voteID)
	}
	_, voteBytes := v.VoteTree.Get([]byte(voteID))
	if len(voteBytes) == 0 {
		return nil, fmt.Errorf("vote with id (%s) does not exists", voteID)
	}
	//log.Debugf("get envelope votebytes: %b", voteBytes)
	err := v.Codec.UnmarshalBinaryBare(voteBytes, &vote)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal vote with id (%s)", voteID)
	}
	//log.Debugf("get envelope value: %+v", vote)
	return vote, nil
}

// CountVotes returns the number of votes registered for a given process id
func (v *VochainState) CountVotes(processID string) int64 {
	processID = sanitizeHex(processID)
	var count int64
	v.VoteTree.Iterate(func(key []byte, value []byte) bool {
		k := strings.Split(string(key), "_")
		if k[0] == processID {
			count++
		}
		return false
	})
	return count
}

// GetHeight returns the blockchain last block commited height
func (v *VochainState) GetHeight() int64 {
	_, headerBytes := v.AppTree.Get([]byte(headerKey))
	var header tmtypes.Header
	err := v.Codec.UnmarshalBinaryBare(headerBytes, &header)
	if err != nil {
		return 0
	}
	return header.Height
}

// GetAppHash returns last hash of the application
func (v *VochainState) GetAppHash() []byte {
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
	h1, _, _ := v.AppTree.SaveVersion()
	h2, _, _ := v.ProcessTree.SaveVersion()
	h3, _, _ := v.VoteTree.SaveVersion()
	return signature.HashRaw(fmt.Sprintf("%s%s%s", h1, h2, h3))
}

// Rollback rollbacks to the last persistent db data version
func (v *VochainState) Rollback() {
	v.AppTree.Rollback()
	v.ProcessTree.Rollback()
	v.VoteTree.Rollback()
}

// GetHash returns the hash of the vochain trees mkroots
// hash(appTree+processTree+voteTree)
func (v *VochainState) GetWorkingHash() []byte {
	return signature.HashRaw(fmt.Sprintf("%s%s%s", v.AppTree.WorkingHash(), v.ProcessTree.WorkingHash(), v.VoteTree.WorkingHash()))
}

func sanitizeHex(hexStr string) string {
	if strings.HasPrefix(hexStr, "0x") {
		return fmt.Sprintf("%s", hexStr[2:])
	}
	return hexStr
}

// ProcessList returns a list of processId given an entityId
// func (v *VochainState) GetProcessList(entityId string) []Process

// EnvelopeList returns a list of envelopes given a processId
// func (v *VochainState) GetEnvelopeList(processId string) []Vote
