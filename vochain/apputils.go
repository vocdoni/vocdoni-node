package vochain

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	models "go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	ethcommon "github.com/ethereum/go-ethereum/common"

	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
)

// NewPrivateValidator returns a tendermint file private validator (key and state)
// if tmPrivKey not specified, uses the existing one or generates a new one
func NewPrivateValidator(tmPrivKey, keyFilePath, stateFilePath string) (*privval.FilePV, error) {
	pv, err := privval.LoadOrGenFilePV(keyFilePath, stateFilePath)
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
func NewNodeKey(tmPrivKey, nodeKeyFilePath string) (*tmtypes.NodeKey, error) {
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
	return nodeKey, nodeKey.SaveAs(nodeKeyFilePath)
}

// NewGenesis creates a new genesis and return its bytes
func NewGenesis(cfg *config.VochainCfg, chainID string, consensusParams *ConsensusParams,
	validators []privval.FilePV, oracles, accounts []string, initAccountsBalance int, treasurer string, txCosts *TransactionCosts) ([]byte, error) {
	// default consensus params
	appState := new(GenesisAppState)
	appState.Validators = make([]GenesisValidator, len(validators))
	for idx, val := range validators {
		pubk, err := val.GetPubKey(context.Background())
		if err != nil {
			return nil, err
		}
		appState.Validators[idx] = GenesisValidator{
			Address: val.GetAddress().Bytes(),
			PubKey:  TendermintPubKey{Value: pubk.Bytes(), Type: "tendermint/PubKeyEd25519"},
			Power:   "10",
			Name:    strconv.Itoa(rand.Int()),
		}
	}
	for _, os := range oracles {
		os, err := hex.DecodeString(util.TrimHex(os))
		if err != nil {
			return nil, err
		}
		appState.Oracles = append(appState.Oracles, os)
	}
	for _, acc := range accounts {
		accAddressBytes, err := hex.DecodeString(util.TrimHex(acc))
		if err != nil {
			return nil, err
		}
		appState.Accounts = append(appState.Accounts, GenesisAccount{
			Address: accAddressBytes,
			Balance: uint64(initAccountsBalance),
		})
	}

	if txCosts != nil {
		appState.TxCost = *txCosts
	}
	tb, err := hex.DecodeString(util.TrimHex(treasurer))
	if err != nil {
		return nil, err
	}
	appState.Treasurer = tb
	appStateBytes, err := json.Marshal(appState)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return genBytes, nil
}

// GenerateFaucetPackage generates a faucet package
func GenerateFaucetPackage(from *ethereum.SignKeys, to ethcommon.Address, value, identifier uint64) (*models.FaucetPackage, error) {
	rand.Seed(time.Now().UnixNano())
	payload := &models.FaucetPayload{
		Identifier: identifier,
		To:         to.Bytes(),
		Amount:     value,
	}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadSignature, err := from.SignEthereum(payloadBytes)
	if err != nil {
		return nil, err
	}
	return &models.FaucetPackage{
		Payload:   payloadBytes,
		Signature: payloadSignature,
	}, nil
}

// NewTemplateGenesisFile creates a genesis file with the given number of validators and its private keys.
// Also includes an oracle, treasurer and faucet account.
func NewTemplateGenesisFile(dir string, validators int) error {
	gd := tmtypes.GenesisDoc{}
	gd.ChainID = "test-chain-1"
	gd.GenesisTime = time.Now()
	gd.InitialHeight = 0
	gd.ConsensusParams = tmtypes.DefaultConsensusParams()
	gd.ConsensusParams.Block.MaxBytes = 5242880
	gd.ConsensusParams.Block.MaxGas = -1
	gd.ConsensusParams.Evidence.MaxAgeNumBlocks = 100000
	gd.ConsensusParams.Evidence.MaxAgeDuration = 10000
	gd.ConsensusParams.Validator.PubKeyTypes = []string{"ed25519"}

	// Create validators
	gd.Validators = []tmtypes.GenesisValidator{}
	appStateValidators := []GenesisValidator{}
	for i := 0; i < validators; i++ {
		nodeDir := filepath.Join(dir, fmt.Sprintf("node%d", i))
		if err := os.MkdirAll(nodeDir, 0o700); err != nil {
			return err
		}
		privKey := util.RandomHex(64)
		pv, err := NewPrivateValidator(privKey,
			filepath.Join(nodeDir, "priv_validator_key.json"),
			filepath.Join(nodeDir, "priv_validator_state.json"),
		)
		if err != nil {
			return fmt.Errorf("cannot create validator key and state: (%v)", err)
		}
		pv.Save()
		if err := os.WriteFile(filepath.Join(nodeDir, "hex_priv_key"), []byte(privKey), 0o600); err != nil {
			return err
		}
		gd.Validators = append(gd.Validators, tmtypes.GenesisValidator{
			Address: pv.Key.Address,
			PubKey:  pv.Key.PubKey,
			Power:   10,
		})
		appStateValidators = append(appStateValidators, GenesisValidator{
			Address: pv.Key.Address.Bytes(),
			PubKey: TendermintPubKey{
				Type:  "tendermint/PubKeyEd25519",
				Value: pv.Key.PubKey.Bytes(),
			},
			Power: "10",
		})
	}

	// Generate oracle, treasurer and faucet accounts
	oracle := ethereum.SignKeys{}
	if err := oracle.Generate(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "oracle_hex_key"),
		[]byte(fmt.Sprintf("%x", oracle.PrivateKey())), 0o600); err != nil {
		return err
	}
	treasurer := ethereum.SignKeys{}
	if err := treasurer.Generate(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "treasurer_hex_key"),
		[]byte(fmt.Sprintf("%x", treasurer.PrivateKey())), 0o600); err != nil {
		return err
	}
	faucet := ethereum.SignKeys{}
	if err := faucet.Generate(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "faucet_hex_key"),
		[]byte(fmt.Sprintf("%x", faucet.PrivateKey())), 0o600); err != nil {
		return err
	}

	// Create seed node
	seedKey := util.RandomHex(64)
	seedDir := filepath.Join(dir, "seed")
	if err := os.MkdirAll(seedDir, 0o700); err != nil {
		return err
	}
	seedNodeKey, err := NewNodeKey(seedKey, filepath.Join(seedDir, "node_key.json"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(
		seedDir, "seed_address"),
		[]byte(seedNodeKey.ID.AddressString("seed1.foo.bar:26656")),
		0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(seedDir, "hex_seed_key"), []byte(seedKey), 0o600); err != nil {
		return err
	}

	// Build genesis app state and create genesis file
	appState := GenesisAppState{
		Validators: appStateValidators,
		Oracles:    []types.HexBytes{oracle.Address().Bytes()},
		Treasurer:  types.HexBytes(treasurer.Address().Bytes()),
		Accounts: []GenesisAccount{
			{
				Address: faucet.Address().Bytes(),
				Balance: 100000,
			},
		},
		TxCost: TransactionCosts{},
	}
	appStateBytes, err := json.Marshal(appState)
	if err != nil {
		return err
	}
	gd.AppState = appStateBytes
	return gd.SaveAs(filepath.Join(dir, "genesis.json"))
}
