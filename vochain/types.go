package vochain

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	voteCachePurgeThreshold = uint32(60) // in blocks, 10 minutes
	voteCacheSize           = 50000
	costPrefix              = "c_"
)

var (
	ErrVoteDoesNotExist    = fmt.Errorf("vote does not exist")
	ErrNotEnoughBalance    = fmt.Errorf("not enough balance to transfer")
	ErrAccountNonceInvalid = fmt.Errorf("invalid account nonce")
	ErrProcessNotFound     = fmt.Errorf("process not found")
	ErrBalanceOverflow     = fmt.Errorf("balance overflow")
	ErrAccountBalanceZero  = fmt.Errorf("zero balance account not valid")
	ErrAccountNotFound     = fmt.Errorf("account does not exist")
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

type sendTokensTxCheckValues struct {
	From, To common.Address
	Value    uint64
	Nonce    uint32
}

type setAccountInfoTxCheckValues struct {
	Account, TxSender common.Address
	Create            bool
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
	SetProcessStatus        uint64 `json:"Tx_SetProcessStatus"`
	SetProcessCensus        uint64 `json:"Tx_SetProcessCensus"`
	SetProcessResults       uint64 `json:"Tx_SetProcessResults"`
	SetProcessQuestionIndex uint64 `json:"Tx_SetProcessQuestionIndex"`
	RegisterKey             uint64 `json:"Tx_RegisterKey"`
	NewProcess              uint64 `json:"Tx_NewProcess"`
	SendTokens              uint64 `json:"Tx_SendTokens"`
	SetAccountInfo          uint64 `json:"Tx_SetAccountInfo"`
	AddDelegateForAccount   uint64 `json:"Tx_AddDelegateForAccount"`
	DelDelegateForAccount   uint64 `json:"Tx_DelDelegateForAccount"`
	CollectFaucet           uint64 `json:"Tx_CollectFaucet"`
}

// AsMap returns the contents of TransactionCosts as a map. Its purpose
// is to keep knowledge of TransactionCosts' fields within itself, so the
// function using it only needs to iterate over the key-values.
func (t *TransactionCosts) AsMap() map[models.TxType]uint64 {
	b := make(map[models.TxType]uint64)

	tType := reflect.TypeOf(*t)
	tValue := reflect.ValueOf(*t)
	for i := 0; i < tType.NumField(); i++ {
		key := TxCostNameToTxType(tType.Field(i).Name)
		b[key] = tValue.Field(i).Uint()
	}
	return b
}

var TxCostNameToTxTypeMap = map[string]models.TxType{
	"SetProcessStatus":        models.TxType_SET_PROCESS_STATUS,
	"SetProcessCensus":        models.TxType_SET_PROCESS_CENSUS,
	"SetProcessResults":       models.TxType_SET_PROCESS_RESULTS,
	"SetProcessQuestionIndex": models.TxType_SET_PROCESS_QUESTION_INDEX,
	"SendTokens":              models.TxType_SEND_TOKENS,
	"SetAccountInfo":          models.TxType_SET_ACCOUNT_INFO,
	"RegisterKey":             models.TxType_REGISTER_VOTER_KEY,
	"NewProcess":              models.TxType_NEW_PROCESS,
	"AddDelegateForAccount":   models.TxType_ADD_DELEGATE_FOR_ACCOUNT,
	"DelDelegateForAccount":   models.TxType_DEL_DELEGATE_FOR_ACCOUNT,
	"CollectFaucet":           models.TxType_COLLECT_FAUCET,
}

func TxCostNameToTxType(key string) models.TxType {
	if _, ok := TxCostNameToTxTypeMap[key]; ok {
		return TxCostNameToTxTypeMap[key]
	}
	return models.TxType_TX_UNKNOWN
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
	Version   VersionParams   `json:"version"`
}

// BlockParams define limits on the block size and gas plus minimum time
// between blocks.
type BlockParams struct {
	MaxBytes int64 `json:"max_bytes"`
	MaxGas   int64 `json:"max_gas"`
}

type EvidenceParams struct {
	MaxAgeNumBlocks int64 `json:"max_age_num_blocks"`
	// only accept new evidence more recent than this
	MaxAgeDuration time.Duration `json:"max_age_duration"`
}

type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

type VersionParams struct {
	AppVersion uint64 `json:"app_version"`
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
