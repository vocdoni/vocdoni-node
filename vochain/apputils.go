package vochain

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"

	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/tree"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"

	amino "github.com/tendermint/go-amino"
	cfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	cryptoAmino "github.com/tendermint/tendermint/crypto/encoding/amino"
	"github.com/tendermint/tendermint/libs/tempfile"
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
func verifySignatureAgainstOracles(oracles []string, message []byte, signHex string) (bool, string, error) {
	signKeys := signature.SignKeys{}
	for _, oracle := range oracles {
		if err := signKeys.AddAuthKey(oracle); err != nil {
			return false, "", err
		}
	}
	return signKeys.VerifySender(message, signHex)
}

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
func GenerateNullifier(address, processID string) (string, error) {
	var err error
	addrBytes, err := hex.DecodeString(address)
	if err != nil {
		return "", err
	}
	pidBytes, err := hex.DecodeString(processID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", signature.HashRaw([]byte(fmt.Sprintf("%s%s", addrBytes, pidBytes)))), nil
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

// NewNodeKey returns a tendermint node key
// if tmPrivKey not specified, uses the existing one or generates a new one
func NewNodeKey(tmPrivKey string, tconfig *cfg.Config) (*p2p.NodeKey, error) {
	nodeKey, err := p2p.LoadOrGenNodeKey(tconfig.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load node's key: (%s)", err)
	}
	if len(tmPrivKey) > 0 {
		var privKey crypto25519.PrivKeyEd25519
		keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
		if err != nil {
			return nil, fmt.Errorf("cannot decode private key: (%s)", err)
		}
		if n := copy(privKey[:], keyBytes[:]); n != 64 {
			return nil, fmt.Errorf("incorrect private key lenght (got %d, need 64)", n)
		}
		nodeKey.PrivKey = privKey
	}
	return nodeKey, nil
}

// NodeKeySave save a p2p node key on disk
func NodeKeySave(filePath string, nodeKey *p2p.NodeKey) error {
	outFile := filePath
	if outFile == "" {
		return fmt.Errorf("cannot save NodeKey key: filePath not set")
	}

	aminoPrivKey, _, err := HexKeyToAmino(fmt.Sprintf("%x", nodeKey.PrivKey))
	if err != nil {
		return err
	}
	err = tempfile.WriteFileAtomic(outFile, []byte(fmt.Sprintf(`{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"%s"}}`, aminoPrivKey)), 0600)
	if err != nil {
		return err
	}
	return nil
}

// NewGenesis creates a new genesis and return its bytes
func NewGenesis(cfg *config.VochainCfg, chainID string, consensusParams *tmtypes.ConsensusParams, validators []privval.FilePV, oracles []string) ([]byte, error) {
	// default consensus params
	appState := new(types.GenesisAppState)
	appState.Validators = make([]tmtypes.GenesisValidator, len(validators))
	for idx, val := range validators {
		appState.Validators[idx] = tmtypes.GenesisValidator{
			Address: val.GetAddress(),
			PubKey:  val.GetPubKey(),
			Power:   10,
			Name:    strconv.Itoa(rand.Int()),
		}
	}

	appState.Oracles = oracles
	cdc := amino.NewCodec()
	cryptoAmino.RegisterAmino(cdc)

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
