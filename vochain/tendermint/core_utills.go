package tendermint

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	amino "github.com/tendermint/go-amino"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	types "github.com/tendermint/tendermint/types"
)

// ExportGenesisFile creates and writes the genesis configuration to disk
// an error is returned if something fails
func ExportGenesisFile(genesisFileName, chainID string, validators []types.GenesisValidator, appState json.RawMessage, genesisTime time.Time) error {
	genesisDoc := types.GenesisDoc{
		GenesisTime: genesisTime,
		ChainID:     chainID,
		Validators:  validators,
		AppState:    appState,
	}

	if err := genesisDoc.ValidateAndComplete(); err != nil {
		return err
	}

	return genesisDoc.SaveAs(genesisFileName)
}

// LoadGenesisFile loads genesis content from a JSON genesis file
func LoadGenesisFile(cdc *amino.Codec, genesisFileName string) (genesisDoc types.GenesisDoc, err error) {
	genesisContent, err := ioutil.ReadFile(genesisFileName)

	if err != nil {
		return genesisDoc, err
	}

	if err := cdc.UnmarshalJSON(genesisContent, &genesisDoc); err != nil {
		return genesisDoc, err
	}

	return genesisDoc, err
}

// InitializeNodeValidatorFiles creates a validator and its configuration files
func InitializeNodeValidatorFiles(config *cfg.Config) (nodeID string, pubKey crypto.PubKey, err error) {
	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())

	if err != nil {
		return nodeID, pubKey, err
	}

	nodeID = string(nodeKey.ID())

	pvKeyFile := config.PrivValidatorKeyFile()
	if err := cmn.EnsureDir(filepath.Dir(pvKeyFile), 0777); err != nil {
		return nodeID, pubKey, nil
	}

	pvStateFile := config.PrivValidatorStateFile()
	if err := cmn.EnsureDir(filepath.Dir(pvKeyFile), 0777); err != nil {
		return nodeID, pubKey, nil
	}

	pubKey = privval.LoadOrGenFilePV(pvKeyFile, pvStateFile).GetPubKey()
	return nodeID, pubKey, nil
}
