package vochain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"strconv"

	"go.vocdoni.io/dvote/censustree/gravitontree"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	amino "github.com/tendermint/go-amino"
	cfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtime "github.com/tendermint/tendermint/types/time"
	"github.com/vocdoni/dvote-protobuf/build/go/models"
	"github.com/vocdoni/eth-storage-proof/ethstorageproof"
)

// hexproof is the hexadecimal a string. leafData is the claim data in byte format
func checkMerkleProof(proof *models.Proof, censusOrigin models.CensusOrigin, censusRootHash, leafData []byte) (bool, error) {
	switch censusOrigin {
	case models.CensusOrigin_OFF_CHAIN:
		switch proof.Payload.(type) {
		case *models.Proof_Graviton:
			p := proof.GetGraviton()
			return gravitontree.CheckProof(leafData, []byte{}, censusRootHash, p.Siblings)
		case *models.Proof_Iden3:
			// NOT IMPLEMENTED
		}
	case models.CensusOrigin_ERC20:
		p := proof.GetEthereumStorage()
		if !bytes.Equal(p.Key, leafData) {
			return false, nil
		}
		hexproof := []string{}
		for _, s := range p.Siblings {
			hexproof = append(hexproof, fmt.Sprintf("%x", s))
		}
		amount := big.Int{}
		amount.SetBytes(p.Value)
		hexamount := hexutil.Big(amount)
		log.Debugf("validating erc20 storage proof for key %x and amount %s", p.Key, amount.String())
		return ethstorageproof.VerifyEthStorageProof(
			&ethstorageproof.StorageResult{Key: fmt.Sprintf("%x", p.Key), Proof: hexproof, Value: &hexamount},
			ethcommon.BytesToHash(censusRootHash),
		)
	}
	return false, fmt.Errorf("proof type not supported for census origin %d", censusOrigin)
}

// VerifySignatureAgainstOracles verifies that a signature match with one of the oracles
func verifySignatureAgainstOracles(oracles []ethcommon.Address, message []byte, signHex string) (bool, ethcommon.Address, error) {
	signKeys := ethereum.NewSignKeys()
	for _, oracle := range oracles {
		signKeys.AddAuthKey(oracle)
	}
	return signKeys.VerifySender(message, signHex)
}

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
func GenerateNullifier(address ethcommon.Address, processID []byte) []byte {
	return ethereum.HashRaw([]byte(fmt.Sprintf("%s%s", address.Bytes(), processID)))
}

// NewPrivateValidator returns a tendermint file private validator (key and state)
// if tmPrivKey not specified, uses the existing one or generates a new one
func NewPrivateValidator(tmPrivKey string, tconfig *cfg.Config) (*privval.FilePV, error) {
	pv, err := privval.LoadOrGenFilePV(
		tconfig.PrivValidatorKeyFile(),
		tconfig.PrivValidatorStateFile(),
	)
	if err != nil {
		return nil, err
	}
	if len(tmPrivKey) > 0 {
		var privKey crypto25519.PrivKey
		keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
		if err != nil {
			return nil, fmt.Errorf("cannot decode private key: (%s)", err)
		}
		privKey = make([]byte, 64)
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
	var privKey crypto25519.PrivKey
	keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
	if err != nil {
		return nil, fmt.Errorf("cannot decode private key: (%s)", err)
	}
	privKey = make([]byte, len(keyBytes))
	copy(privKey[:], keyBytes[:])
	nodeKey := &p2p.NodeKey{
		PrivKey: privKey,
	}

	cdc := AminoCodec()
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
func NewGenesis(cfg *config.VochainCfg, chainID string, consensusParams *types.ConsensusParams, validators []privval.FilePV, oracles []string) ([]byte, error) {
	// default consensus params
	appState := new(types.GenesisAppState)
	appState.Validators = make([]types.GenesisValidator, len(validators))
	for idx, val := range validators {
		pubk, err := val.GetPubKey()
		if err != nil {
			return []byte{}, err
		}
		appState.Validators[idx] = types.GenesisValidator{
			Address: val.GetAddress(),
			PubKey:  types.TendermintPubKey{Value: pubk.Bytes(), Type: "tendermint/PubKeyEd25519"},
			Power:   "10",
			Name:    strconv.Itoa(rand.Int()),
		}
	}
	appState.Oracles = oracles
	cdc := amino.NewCodec()
	appStateBytes, err := cdc.MarshalJSON(appState)
	if err != nil {
		return []byte{}, err
	}
	genDoc := types.GenesisDoc{
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
