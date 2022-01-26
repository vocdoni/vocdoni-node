package vochain

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"

	ethcommon "github.com/ethereum/go-ethereum/common"

	cfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.vocdoni.io/proto/build/go/models"
)

// verifySignatureAgainstOracles verifies that a signature match with one of the oracles
func verifySignatureAgainstOracles(oracles []ethcommon.Address, message,
	signature []byte) (bool, ethcommon.Address, error) {
	signKeys := ethereum.NewSignKeys()
	for _, oracle := range oracles {
		signKeys.AddAuthKey(oracle)
	}
	return signKeys.VerifySender(message, signature)
}

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
// This function assumes address and processID are correct.
func GenerateNullifier(address ethcommon.Address, processID []byte) []byte {
	nullifier := bytes.Buffer{}
	nullifier.Write(address.Bytes())
	nullifier.Write(processID)
	return ethereum.HashRaw(nullifier.Bytes())
}

// NewPrivateValidator returns a tendermint file private validator (key and state)
// if tmPrivKey not specified, uses the existing one or generates a new one
func NewPrivateValidator(tmPrivKey string, tconfig *cfg.Config) (*privval.FilePV, error) {
	pv, err := privval.LoadOrGenFilePV(
		tconfig.PrivValidator.KeyFile(),
		tconfig.PrivValidator.StateFile(),
	)
	if err != nil {
		log.Fatal(err)
	}
	if len(tmPrivKey) > 0 {
		var privKey crypto25519.PrivKey
		keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
		if err != nil {
			return nil, fmt.Errorf("cannot decode private key: (%s)", err)
		}
		privKey = make([]byte, 64)
		if n := copy(privKey[:], keyBytes[:]); n != 64 {
			return nil, fmt.Errorf("incorrect private key length (got %d, need 64)", n)
		}
		pv.Key.Address = privKey.PubKey().Address()
		pv.Key.PrivKey = privKey
		pv.Key.PubKey = privKey.PubKey()
	}
	return pv, nil
}

// NewNodeKey returns and saves to the disk storage a tendermint node key
func NewNodeKey(tmPrivKey string, tconfig *cfg.Config) (*tmtypes.NodeKey, error) {
	if tmPrivKey == "" {
		return nil, fmt.Errorf("nodekey not specified")
	}
	nodeKey := &tmtypes.NodeKey{}
	keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
	if err != nil {
		return nodeKey, fmt.Errorf("cannot decode private key: (%s)", err)
	}
	nodeKey.PrivKey = crypto25519.PrivKey(keyBytes)
	nodeKey.ID = tmtypes.NodeIDFromPubKey(nodeKey.PrivKey.PubKey())
	// Write nodeKey to disk
	return nodeKey, nodeKey.SaveAs(tconfig.NodeKeyFile())
}

// NewGenesis creates a new genesis and return its bytes
func NewGenesis(cfg *config.VochainCfg, chainID string, consensusParams *ConsensusParams,
	validators []privval.FilePV, oracles []string, treasurer string) ([]byte, error) {
	// default consensus params
	appState := new(GenesisAppState)
	appState.Validators = make([]GenesisValidator, len(validators))
	for idx, val := range validators {
		pubk, err := val.GetPubKey(context.Background())
		if err != nil {
			return []byte{}, err
		}
		appState.Validators[idx] = GenesisValidator{
			Address: val.GetAddress().Bytes(),
			PubKey:  TendermintPubKey{Value: pubk.Bytes(), Type: "tendermint/PubKeyEd25519"},
			Power:   "10",
			Name:    strconv.Itoa(rand.Int()),
		}
	}
	appState.Oracles = oracles
	appState.Treasurer = treasurer
	appStateBytes, err := tmjson.Marshal(appState)
	if err != nil {
		return []byte{}, err
	}
	genDoc := GenesisDoc{
		ChainID:         chainID,
		GenesisTime:     tmtime.Now(),
		ConsensusParams: consensusParams,
		Validators:      appState.Validators,
		AppState:        appStateBytes,
	}

	// Note that the genesis doc bytes are later consumed by tendermint,
	// which expects amino-flavored json. We can't use encoding/json.
	genBytes, err := tmjson.Marshal(genDoc)
	if err != nil {
		return []byte{}, err
	}

	return genBytes, nil
}

func GetFriendlyResults(results []*models.QuestionResult) [][]string {
	r := [][]string{}
	for i := range results {
		r = append(r, []string{})
		for j := range results[i].Question {
			r[i] = append(r[i], new(big.Int).SetBytes(results[i].Question[j]).String())
		}
	}
	return r
}
