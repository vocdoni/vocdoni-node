package genesis

import (
	"encoding/json"

	comethash "github.com/cometbft/cometbft/crypto/tmhash"
	comettypes "github.com/cometbft/cometbft/types"

	"go.vocdoni.io/dvote/types"
)

// Doc is a wrapper around the CometBFT GenesisDoc,
// that adds some useful methods like Hash
type Doc struct {
	comettypes.GenesisDoc
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
	return comethash.Sum(data)
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
