package genesis

import (
	"encoding/json"
	"strconv"
	"time"

	comettypes "github.com/cometbft/cometbft/types"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
)

// Vochain is a struct containing the genesis details.
type Vochain struct {
	AutoUpdateGenesis bool
	SeedNodes         []string
	StateSync         map[string]StateSyncParams
	Genesis           *Doc
}

// The genesis app state types are copied from
// github.com/cometbft/cometbft/types, for the sake of making this package
// lightweight and not have it import heavy indirect dependencies like grpc or
// crypto/*.

// Doc defines the initial conditions for a Vocdoni blockchain.
// It is mostly a wrapper around the Tendermint GenesisDoc.
type Doc struct {
	GenesisTime     time.Time        `json:"genesis_time"`
	ChainID         string           `json:"chain_id"`
	ConsensusParams *ConsensusParams `json:"consensus_params,omitempty"`
	AppHash         types.HexBytes   `json:"app_hash"`
	AppState        AppState         `json:"app_state,omitempty"`
}

// TendermintDoc returns the Tendermint GenesisDoc from the Vocdoni genesis.Doc.
func (g *Doc) TendermintDoc() comettypes.GenesisDoc {
	appState, err := json.Marshal(g.AppState)
	if err != nil {
		// must never happen
		panic(err)
	}
	return comettypes.GenesisDoc{
		GenesisTime: g.GenesisTime,
		ChainID:     g.ChainID,
		ConsensusParams: &comettypes.ConsensusParams{
			Block: comettypes.BlockParams{
				MaxBytes: int64(g.ConsensusParams.Block.MaxBytes),
				MaxGas:   int64(g.ConsensusParams.Block.MaxGas),
			},
		},
		Validators: []comettypes.GenesisValidator{},
		AppHash:    []byte(g.AppHash),
		AppState:   appState,
	}
}

// Marshal returns the JSON encoding of the genesis.Doc.
func (g *Doc) Marshal() []byte {
	data, err := json.Marshal(g)
	if err != nil {
		panic(err)
	}
	return data
}

// Hash returns the hash of the genesis.Doc.
func (g *Doc) Hash() []byte {
	data, err := json.Marshal(g)
	if err != nil {
		panic(err)
	}
	return ethereum.HashRaw(data)
}

// ConsensusParams defines the consensus critical parameters that determine the
// validity of blocks. This comes from Tendermint.
type ConsensusParams struct {
	Block     BlockParams     `json:"block"`
	Evidence  EvidenceParams  `json:"evidence"`
	Validator ValidatorParams `json:"validator"`
	Version   VersionParams   `json:"version"`

	Synchrony comettypes.SynchronyParams `json:"synchrony"`
}

// BlockParams define limits on the block size and gas plus minimum time
// between blocks. This comes from Tendermint.
type BlockParams struct {
	MaxBytes StringifiedInt64 `json:"max_bytes"`
	MaxGas   StringifiedInt64 `json:"max_gas"`
}

// EvidenceParams define limits on max evidence age and max duration
type EvidenceParams struct {
	MaxAgeNumBlocks StringifiedInt64 `json:"max_age_num_blocks"`
	// only accept new evidence more recent than this
	MaxAgeDuration StringifiedInt64 `json:"max_age_duration"`
}

// ValidatorParams define the validator key
type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

// VersionParams define the version app information
type VersionParams struct {
	AppVersion StringifiedInt64 `json:"app_version"`
}

// ________________________ GENESIS APP STATE ________________________

// Account represents an account in the genesis app state
type Account struct {
	Address types.HexBytes `json:"address"`
	Balance uint64         `json:"balance"`
}

// AppState is the main application state in the genesis file.
type AppState struct {
	Validators      []AppStateValidators `json:"validators"`
	Accounts        []Account            `json:"accounts"`
	TxCost          TransactionCosts     `json:"tx_cost"`
	MaxElectionSize uint64               `json:"max_election_size"`
	NetworkCapacity uint64               `json:"network_capacity"`
}

// AppStateValidators represents a validator in the genesis app state.
type AppStateValidators struct {
	Address  types.HexBytes `json:"signer_address"`
	PubKey   types.HexBytes `json:"consensus_pub_key"`
	Power    uint64         `json:"power"`
	Name     string         `json:"name"`
	KeyIndex uint8          `json:"key_index"`
}

// StateSyncParams define the parameters used by StateSync
type StateSyncParams struct {
	TrustHeight int64
	TrustHash   types.HexBytes
}

// StringifiedInt64 is a wrapper around int64 that marshals/unmarshals as a string.
// This is a dirty non-sense workaround. Blame Tendermint not me.
// For some (unknown) reason Tendermint requires the integer values to be strings in
// the JSON genesis file.
type StringifiedInt64 int64

// MarshalJSON implements the json.Marshaler interface.
func (i StringifiedInt64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(i), 10))
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (i *StringifiedInt64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*i = StringifiedInt64(v)
	return nil
}
