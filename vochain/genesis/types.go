package genesis

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.vocdoni.io/dvote/types"
)

// Doc is the genesis document of a chain: its identity and initial state.
type Doc struct {
	GenesisTime   time.Time       `json:"genesis_time"`
	ChainID       string          `json:"chain_id"`
	InitialHeight int64           `json:"initial_height"`
	AppState      json.RawMessage `json:"app_state,omitempty"`
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
	h := sha256.Sum256(g.Marshal())
	return h[:]
}

// Validate checks that the mandatory fields of the genesis.Doc are set.
func (g *Doc) Validate() error {
	if g.ChainID == "" {
		return fmt.Errorf("genesis doc must include a non-empty chain_id")
	}
	if g.InitialHeight < 0 {
		return fmt.Errorf("initial_height cannot be negative (got %d)", g.InitialHeight)
	}
	return nil
}

// SaveAs writes the JSON encoding of the genesis.Doc to the given file.
func (g *Doc) SaveAs(path string) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadFromFile loads and unmarshals a genesis.json, returning a genesis.Doc
func LoadFromFile(path string) (*Doc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	g := &Doc{}
	if err := json.Unmarshal(data, g); err != nil {
		return nil, fmt.Errorf("cannot unmarshal genesis %s: %w", path, err)
	}
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("invalid genesis %s: %w", path, err)
	}
	return g, nil
}

// ________________________ GENESIS APP STATE ________________________

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

// Account represents an account in the genesis app state
type Account struct {
	Address types.HexBytes `json:"address"`
	Balance uint64         `json:"balance"`
}
