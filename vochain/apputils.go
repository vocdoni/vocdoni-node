package vochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	crypto25519 "github.com/cometbft/cometbft/crypto/ed25519"
	crypto256k1 "github.com/cometbft/cometbft/crypto/secp256k1"
	cometp2p "github.com/cometbft/cometbft/p2p"
	cometprivval "github.com/cometbft/cometbft/privval"
	cometrpchttp "github.com/cometbft/cometbft/rpc/client/http"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

// NewPrivateValidator returns a tendermint file private validator (key and state)
// if tmPrivKey not specified, uses the existing one or generates a new one
func NewPrivateValidator(tmPrivKey, keyFilePath, stateFilePath string) (*cometprivval.FilePV, error) {
	pv := cometprivval.LoadOrGenFilePV(keyFilePath, stateFilePath)
	if len(tmPrivKey) > 0 {
		var privKey crypto256k1.PrivKey
		keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
		if err != nil {
			return nil, fmt.Errorf("cannot decode private key: (%s)", err)
		}
		privKey = make([]byte, crypto256k1.PrivKeySize)
		if n := copy(privKey[:], keyBytes); n != crypto256k1.PrivKeySize {
			return nil, fmt.Errorf("incorrect private key length (got %d, need %d)", n, crypto25519.PrivateKeySize)
		}
		pv.Key.Address = privKey.PubKey().Address()
		pv.Key.PrivKey = privKey
		pv.Key.PubKey = privKey.PubKey()
	}
	return pv, nil
}

// NewNodeKey returns and saves to the disk storage a tendermint node key.
// If tmPrivKey not specified, generates a new one
func NewNodeKey(tmPrivKey, nodeKeyFilePath string) (*cometp2p.NodeKey, error) {
	nodeKey := &cometp2p.NodeKey{}
	if tmPrivKey != "" {
		keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
		if err != nil {
			return nil, fmt.Errorf("cannot decode private key: (%s)", err)
		}
		// We need to use ed25519 curve for node key since tendermint does not support secp256k1
		nodeKey.PrivKey = crypto25519.PrivKey(keyBytes)
	} else {
		nodeKey.PrivKey = crypto25519.GenPrivKey()
	}
	// Write nodeKey to disk
	return nodeKey, nodeKey.SaveAs(nodeKeyFilePath)
}

// GenerateFaucetPackage generates a faucet package.
// The package is signed by the given `from` key (holder of the funds) and sent to the `to` address.
// The `amount` is the amount of tokens to be sent.
func GenerateFaucetPackage(from *ethereum.SignKeys, to ethcommon.Address, amount uint64) (*models.FaucetPackage, error) {
	nonce := util.RandomInt(0, math.MaxInt32)
	payload := &models.FaucetPayload{
		Identifier: uint64(nonce),
		To:         to.Bytes(),
		Amount:     amount,
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
// Also includes faucet account.
// The genesis document is returned.
func NewTemplateGenesisFile(dir string, validators int) (*genesis.Doc, error) {
	gd := genesis.HardcodedForNetwork("test")
	gd.ChainID = "test-chain-1"
	gd.GenesisTime = time.Now()
	gd.InitialHeight = 0

	// Faucet
	faucet := ethereum.SignKeys{}
	if err := faucet.Generate(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "faucet_hex_key"),
		[]byte(fmt.Sprintf("%x", faucet.PrivateKey())), 0o600); err != nil {
		return nil, err
	}

	// Create seed node
	seedDir := filepath.Join(dir, "seed")
	if err := os.MkdirAll(seedDir, 0o700); err != nil {
		return nil, err
	}
	seedNodeKey, err := NewNodeKey("", filepath.Join(seedDir, "node_key.json"))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(
		seedDir, "seed_address"),
		[]byte(fmt.Sprintf("%s@seed1.foo.bar:26656", seedNodeKey.ID())),
		0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(seedDir, "hex_seed_key"),
		[]byte(hex.EncodeToString(seedNodeKey.PrivKey.Bytes())),
		0o600); err != nil {
		return nil, err
	}

	// Build genesis app state and create genesis file
	appState := genesis.AppState{
		Accounts: []genesis.Account{
			{
				Address: faucet.Address().Bytes(),
				Balance: 100000,
			},
		},
		TxCost: genesis.TransactionCosts{},
	}
	appState.MaxElectionSize = 100000

	// Create validators
	for i := 0; i < validators; i++ {
		nodeDir := filepath.Join(dir, fmt.Sprintf("node%d", i))
		if err := os.MkdirAll(nodeDir, 0o700); err != nil {
			return nil, err
		}
		pk := crypto256k1.GenPrivKey()
		privKeyHex := hex.EncodeToString(pk.Bytes())
		pv, err := NewPrivateValidator(privKeyHex,
			filepath.Join(nodeDir, "priv_validator_key.json"),
			filepath.Join(nodeDir, "priv_validator_state.json"),
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create validator key and state: (%v)", err)
		}
		pv.Save()
		if err := os.WriteFile(filepath.Join(nodeDir, "hex_priv_key"), []byte(privKeyHex), 0o600); err != nil {
			return nil, err
		}
		signer := ethereum.SignKeys{}
		if err := signer.AddHexKey(hex.EncodeToString(pv.Key.PrivKey.Bytes())); err != nil {
			return nil, err
		}
		appState.Validators = append(appState.Validators, genesis.AppStateValidators{
			Address:  signer.Address().Bytes(),
			PubKey:   pv.Key.PubKey.Bytes(),
			Power:    10,
			Name:     fmt.Sprintf("validator%d", i),
			KeyIndex: uint8(i + 1), // zero is reserved for disabling validator key keeper capabilities
		})
	}

	appStateBytes, err := json.Marshal(appState)
	if err != nil {
		return nil, err
	}
	gd.AppState = appStateBytes
	return gd, gd.SaveAs(filepath.Join(dir, "genesis.json"))
}

// newCometRPCClient sets up a new cometbft RPC client
func newCometRPCClient(server string) (*cometrpchttp.HTTP, error) {
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	c, err := cometrpchttp.New(server)
	if err != nil {
		return nil, err
	}
	return c, nil
}
