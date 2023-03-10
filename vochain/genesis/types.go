package genesis

import (
	"encoding/json"
	"strconv"
	"time"

	tmtypes "github.com/tendermint/tendermint/types"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
)

// VochainGenesis is a struct containing the genesis details.
type VochainGenesis struct {
	AutoUpdateGenesis bool
	SeedNodes         []string
	CircuitsConfigTag string
	Genesis           *GenesisDoc
}

// The genesis app state types are copied from
// github.com/tendermint/tendermint/types, for the sake of making this package
// lightweight and not have it import heavy indirect dependencies like grpc or
// crypto/*.

// GenesisDoc defines the initial conditions for a Vocdoni blockchain.
// It is mostly a wrapper around the Tendermint GenesisDoc.
type GenesisDoc struct {
	GenesisTime     time.Time        `json:"genesis_time"`
	ChainID         string           `json:"chain_id"`
	ConsensusParams *ConsensusParams `json:"consensus_params,omitempty"`
	AppHash         types.HexBytes   `json:"app_hash"`
	AppState        GenesisAppState  `json:"app_state,omitempty"`
}

// TendermintDoc returns the Tendermint GenesisDoc from the Vocdoni GenesisDoc.
func (g *GenesisDoc) TendermintDoc() tmtypes.GenesisDoc {
	appState, err := json.Marshal(g.AppState)
	if err != nil {
		// must never happen
		panic(err)
	}
	return tmtypes.GenesisDoc{
		GenesisTime: g.GenesisTime,
		ChainID:     g.ChainID,
		ConsensusParams: &tmtypes.ConsensusParams{
			Block: tmtypes.BlockParams{
				MaxBytes: int64(g.ConsensusParams.Block.MaxBytes),
				MaxGas:   int64(g.ConsensusParams.Block.MaxGas),
			},
		},
		Validators: []tmtypes.GenesisValidator{},
		AppHash:    []byte(g.AppHash),
		AppState:   appState,
	}
}

// Marshal returns the JSON encoding of the GenesisDoc.
func (g *GenesisDoc) Marshal() []byte {
	data, err := json.Marshal(g)
	if err != nil {
		panic(err)
	}
	return data
}

// Hash returns the hash of the GenesisDoc.
func (g *GenesisDoc) Hash() []byte {
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
}

// BlockParams define limits on the block size and gas plus minimum time
// between blocks. This comes from Tendermint.
type BlockParams struct {
	MaxBytes StringifiedInt64 `json:"max_bytes"`
	MaxGas   StringifiedInt64 `json:"max_gas"`
}

type EvidenceParams struct {
	MaxAgeNumBlocks StringifiedInt64 `json:"max_age_num_blocks"`
	// only accept new evidence more recent than this
	MaxAgeDuration StringifiedInt64 `json:"max_age_duration"`
}

type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

type VersionParams struct {
	AppVersion StringifiedInt64 `json:"app_version"`
}

// ________________________ GENESIS APP STATE ________________________

// GenesisAccount represents an account in the genesis app state
type GenesisAccount struct {
	Address types.HexBytes `json:"address"`
	Balance uint64         `json:"balance"`
}

// GenesisAppState is the main application state in the genesis file.
type GenesisAppState struct {
	Validators      []AppStateValidators `json:"validators"`
	Oracles         []types.HexBytes     `json:"oracles"`
	Accounts        []GenesisAccount     `json:"accounts"`
	Treasurer       types.HexBytes       `json:"treasurer"`
	TxCost          TransactionCosts     `json:"tx_cost"`
	MaxElectionSize uint64               `json:"max_election_size"`
}

// AppStateValidators represents a validator in the genesis app state.
type AppStateValidators struct {
	Address  types.HexBytes `json:"signer_address"`
	PubKey   types.HexBytes `json:"consensus_pub_key"`
	Power    uint64         `json:"power"`
	Name     string         `json:"name"`
	KeyIndex uint8          `json:"key_index"`
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
