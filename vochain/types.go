package vochain

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	voteCachePurgeThreshold = uint32(60) // in blocks, 10 minutes
	voteCacheSize           = 50000
	NewProcessCost          = 0
	SetProcessCost          = 0
)

var (
	ErrVoteDoesNotExist    = fmt.Errorf("vote does not exist")
	ErrNotEnoughBalance    = fmt.Errorf("not enough balance to transfer")
	ErrAccountNonceInvalid = fmt.Errorf("invalid account nonce")
	ErrProcessNotFound     = fmt.Errorf("process not found")
	ErrBalanceOverflow     = fmt.Errorf("balance overflow")
	ErrAccountBalanceZero  = fmt.Errorf("zero balance account not valid")
	// keys; not constants because of []byte
	voteCountKey = []byte("voteCount")
)

// PrefixDBCacheSize is the size of the cache for the MutableTree IAVL databases
var PrefixDBCacheSize = 0

// VotePackage represents the payload of a vote (usually base64 encoded)
type VotePackage struct {
	Nonce string `json:"nonce,omitempty"`
	Votes []int  `json:"votes"`
}

// UniqID returns a uniq identifier for the VoteTX. It depends on the Type.
func UniqID(tx *models.SignedTx, isAnonymous bool) string {
	if !isAnonymous {
		if len(tx.Signature) > 32 {
			return string(tx.Signature[:32])
		}
	}
	return ""
}

// ________________________ QUERIES ________________________

// QueryData is an abstraction of any kind of data a query request could have
type QueryData struct {
	Method      string `json:"method"`
	ProcessID   string `json:"processId,omitempty"`
	Nullifier   string `json:"nullifier,omitempty"`
	From        int64  `json:"from,omitempty"`
	ListSize    int64  `json:"listSize,omitempty"`
	Timestamp   int64  `json:"timestamp,omitempty"`
	ProcessType string `json:"type,omitempty"`
}

// ________________________ TRANSACTION COSTS __________________________
// TransactionCosts describes how much each operation should cost
type TransactionCosts struct {
	SetProcess            uint64 `json:"Tx_SetProcess" state:"c_setProcess"`
	RegisterKey           uint64 `json:"Tx_RegisterKey" state:"c_registerKey"`
	NewProcess            uint64 `json:"Tx_NewProcess" state:"c_newProcess"`
	SendTokens            uint64 `json:"Tx_SendTokens" state:"c_sendTokens"`
	SetAccountInfo        uint64 `json:"Tx_SetAccountInfo" state:"c_setAccountInfo"`
	AddDelegateForAccount uint64 `json:"Tx_AddDelegateForAccount" state:"c_addDelegateForAccount"`
	DelDelegateForAccount uint64 `json:"Tx_DelDelegateForAccount" state:"c_delDelegateForAccount"`
	CollectFaucet         uint64 `json:"Tx_CollectFaucet" state:"c_collectFaucet"`
}

// StructAsBytes returns the contents of TransactionCosts as a map. Its purpose
// is to keep knowledge of TransactionCosts' fields within itself, so the
// function using it only needs to iterate over the key-values.
func (t *TransactionCosts) StructAsBytes() (b map[string][]byte, err error) {
	b = make(map[string][]byte)

	tType := reflect.TypeOf(*t)
	tValue := reflect.ValueOf(*t)
	for i := 0; i < tType.NumField(); i++ {
		key := tType.Field(i).Tag.Get("state")
		value := tValue.Field(i).Uint()

		valueBytes, err := util.Uint64ToBytes(value)
		if err != nil {
			return nil, err
		}

		b[key] = valueBytes
	}
	return
}

// TransactionCostsFieldFromStateKey transforms "c_setProcess" to "SetProcess" for all of
// TransactionCosts' fields
func TransactionCostsFieldFromStateKey(key string) (string, error) {
	if key[0:2] != "c_" {
		return "", fmt.Errorf("state keys must start with 'c_', got %s", key)
	}
	name := strings.TrimLeft(key, "c_")

	// strings.Title will misbehave when there are punctuation marks in the
	// string. To clean the input up, we ensure there are only alphabetical
	// characters left in the string
	isAlpha := regexp.MustCompile(`^[A-Za-z]+$`).MatchString
	if !isAlpha(name) {
		return "", fmt.Errorf("%s needs to be alphabetical only", name)
	}
	Name := strings.Title(name)

	// check if TransactionCosts actually has such a field
	_, found := reflect.TypeOf(TransactionCosts{}).FieldByName(Name)
	if !found {
		return "", fmt.Errorf("no such field %s exists on TransactionCosts", Name)
	}
	return Name, nil
}

// ________________________ GENESIS APP STATE ________________________

// GenesisAppState application state in genesis
type GenesisAppState struct {
	Validators []GenesisValidator `json:"validators"`
	Oracles    []string           `json:"oracles"`
	Treasurer  string             `json:"treasurer"`
	TxCost     TransactionCosts   `json:"tx_cost"`
}

// The rest of these genesis app state types are copied from
// github.com/tendermint/tendermint/types, for the sake of making this package
// lightweight and not have it import heavy indirect dependencies like grpc or
// crypto/*.

type GenesisDoc struct {
	GenesisTime     time.Time          `json:"genesis_time"`
	ChainID         string             `json:"chain_id"`
	ConsensusParams *ConsensusParams   `json:"consensus_params,omitempty"`
	Validators      []GenesisValidator `json:"validators,omitempty"`
	AppHash         types.HexBytes     `json:"app_hash"`
	AppState        json.RawMessage    `json:"app_state,omitempty"`
}

type ConsensusParams struct {
	Block     BlockParams     `json:"block"`
	Evidence  EvidenceParams  `json:"evidence"`
	Validator ValidatorParams `json:"validator"`
}

type BlockParams struct {
	MaxBytes int64 `json:"max_bytes"`
	MaxGas   int64 `json:"max_gas"`
	// Minimum time increment between consecutive blocks (in milliseconds)
	// Not exposed to the application.
	TimeIotaMs int64 `json:"time_iota_ms"`
}

type EvidenceParams struct {
	MaxAgeNumBlocks int64 `json:"max_age_num_blocks"`
	// only accept new evidence more recent than this
	MaxAgeDuration time.Duration `json:"max_age_duration"`
}

type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

type GenesisValidator struct {
	Address types.HexBytes   `json:"address"`
	PubKey  TendermintPubKey `json:"pub_key"`
	Power   string           `json:"power"`
	Name    string           `json:"name"`
}

type TendermintPubKey struct {
	Type  string `json:"type"`
	Value []byte `json:"value"`
}

// _________________________ CENSUS ORIGINS __________________________

type CensusProperties struct {
	Name              string
	AllowCensusUpdate bool
	NeedsDownload     bool
	NeedsIndexSlot    bool
	NeedsURI          bool
	WeightedSupport   bool
}

var CensusOrigins = map[models.CensusOrigin]CensusProperties{
	models.CensusOrigin_OFF_CHAIN_TREE: {Name: "offchain tree",
		NeedsDownload: true, NeedsURI: true, AllowCensusUpdate: true},
	models.CensusOrigin_OFF_CHAIN_TREE_WEIGHTED: {
		Name: "offchain weighted tree", NeedsDownload: true, NeedsURI: true,
		WeightedSupport: true, AllowCensusUpdate: true,
	},
	models.CensusOrigin_ERC20: {Name: "erc20", NeedsDownload: true,
		WeightedSupport: true, NeedsIndexSlot: true},
	models.CensusOrigin_OFF_CHAIN_CA: {Name: "ca", WeightedSupport: true,
		NeedsURI: true, AllowCensusUpdate: true},
}
