package vochain

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/tree"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"

	ethcommon "github.com/ethereum/go-ethereum/common"
	amino "github.com/tendermint/go-amino"
	cfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	cryptoamino "github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// hexproof is the hexadecimal a string. leafData is the claim data in byte format
func checkMerkleProof(rootHash, hexproof string, leafData []byte) (bool, error) {
	return tree.CheckProof(rootHash, hexproof, leafData, []byte{})
}

// VerifySignatureAgainstOracles verifies that a signature match with one of the oracles
func verifySignatureAgainstOracles(oracles []string, message []byte, signHex string) (bool, ethcommon.Address, error) {
	signKeys := ethereum.NewSignKeys()
	for _, oracle := range oracles {
		signKeys.AddAuthKey(ethcommon.HexToAddress(oracle))
	}
	return signKeys.VerifySender(message, signHex)
}

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
func GenerateNullifier(address ethcommon.Address, processID string) (string, error) {
	pidBytes, err := hex.DecodeString(util.TrimHex(processID))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", ethereum.HashRaw([]byte(fmt.Sprintf("%s%s", address.Bytes(), pidBytes)))), nil
}

// NewPrivateValidator returns a tendermint file private validator (key and state)
// if tmPrivKey not specified, uses the existing one or generates a new one
func NewPrivateValidator(tmPrivKey string, tconfig *cfg.Config) (*privval.FilePV, error) {
	pv := privval.LoadOrGenFilePV(
		tconfig.PrivValidatorKeyFile(),
		tconfig.PrivValidatorStateFile(),
	)
	if len(tmPrivKey) > 0 {
		var privKey crypto25519.PrivKeyEd25519
		keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
		if err != nil {
			return nil, fmt.Errorf("cannot decode private key: (%s)", err)
		}
		if n := copy(privKey[:], keyBytes[:]); n != 64 {
			return nil, fmt.Errorf("incorrect private key lenght (got %d, need 64)", n)
		}
		pv.Key.Address = privKey.PubKey().Address()
		pv.Key.PrivKey = privKey
		pv.Key.PubKey = privKey.PubKey()
	}
	return pv, nil
}

// NewNodeKey returns and saves to the disk storage a tendermint node key
func NewNodeKey(tmPrivKey string, tconfig *cfg.Config) (*p2p.NodeKey, error) {
	var privKey crypto25519.PrivKeyEd25519
	keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
	if err != nil {
		return nil, fmt.Errorf("cannot decode private key: (%s)", err)
	}
	copy(privKey[:], keyBytes[:])
	nodeKey := &p2p.NodeKey{
		PrivKey: privKey,
	}

	cdc := amino.NewCodec()
	cryptoamino.RegisterAmino(cdc)

	jsonBytes, err := cdc.MarshalJSON(nodeKey)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(tconfig.NodeKeyFile(), jsonBytes, 0600); err != nil {
		return nil, err
	}
	return nodeKey, nil
}

// NewGenesis creates a new genesis and return its bytes
func NewGenesis(cfg *config.VochainCfg, chainID string, consensusParams *tmtypes.ConsensusParams, validators []privval.FilePV, oracles []string) ([]byte, error) {
	// default consensus params
	appState := new(types.GenesisAppState)
	appState.Validators = make([]tmtypes.GenesisValidator, len(validators))
	for idx, val := range validators {
		pubk, err := val.GetPubKey()
		if err != nil {
			return []byte{}, err
		}
		appState.Validators[idx] = tmtypes.GenesisValidator{
			Address: val.GetAddress(),
			PubKey:  pubk,
			Power:   10,
			Name:    strconv.Itoa(rand.Int()),
		}
	}

	appState.Oracles = oracles
	cdc := amino.NewCodec()
	cryptoamino.RegisterAmino(cdc)

	appStateBytes, err := cdc.MarshalJSON(appState)
	if err != nil {
		return []byte{}, err
	}
	genDoc := tmtypes.GenesisDoc{
		ChainID:         chainID,
		GenesisTime:     tmtime.Now(),
		ConsensusParams: consensusParams,
		Validators:      appState.Validators,
		AppState:        appStateBytes,
	}

	genBytes, err := cdc.MarshalJSON(genDoc)
	if err != nil {
		return []byte{}, err
	}

	return genBytes, nil
}
