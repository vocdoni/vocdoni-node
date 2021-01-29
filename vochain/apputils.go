package vochain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"

	"go.vocdoni.io/dvote/censustree/gravitontree"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"google.golang.org/protobuf/proto"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	cfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtime "github.com/tendermint/tendermint/types/time"
	"github.com/vocdoni/storage-proofs-eth-go/ethstorageproof"
	"go.vocdoni.io/proto/build/go/models"
)

// checkProof checks the validity of a census proof (depending on the origin).
// key is the data to be proof in behalf the censusRoot.
// In case of weighted proof, this function will return the weight as second parameter.
func checkProof(proof *models.Proof, censusOrigin models.CensusOrigin, censusRoot, processID, key []byte) (bool, *big.Int, error) {
	switch censusOrigin {
	case models.CensusOrigin_OFF_CHAIN_TREE:
		switch proof.Payload.(type) {
		case *models.Proof_Graviton:
			p := proof.GetGraviton()
			if p == nil {
				return false, nil, fmt.Errorf("graviton proof is empty")
			}
			valid, err := gravitontree.CheckProof(key, []byte{}, censusRoot, p.Siblings)
			return valid, big.NewInt(1), err
		case *models.Proof_Iden3:
			// NOT IMPLEMENTED
			return false, nil, fmt.Errorf("iden3 proof not implemented")
		}
	case models.CensusOrigin_OFF_CHAIN_CA:
		p := proof.GetCa()
		if !bytes.Equal(p.Bundle.Address, key) {
			return false, nil, fmt.Errorf("CA bundle address and key do not match: %x != %x", key, p.Bundle.Address)
		}
		if !bytes.Equal(p.Bundle.ProcessId, processID) {
			return false, nil, fmt.Errorf("CA bundle processID do not match")
		}
		caBundle, err := proto.Marshal(p.Bundle)
		if err != nil {
			return false, nil, fmt.Errorf("cannot marshal ca bundle to protobuf: %w", err)
		}
		var caPubk []byte

		// depending on signature type, use a mechanism for extracting the ca publickey from signature
		switch p.GetType() {
		case models.SignatureType_ECDSA:
			caPubk, err = ethereum.PubKeyFromSignature(caBundle, p.GetSignature())
			if err != nil {
				return false, nil, fmt.Errorf("cannot fetch ca address from signature: %w", err)
			}
		case models.SignatureType_ECDSA_BLIND:
			// Blind CA check
		default:
			return false, nil, fmt.Errorf("ca proof %s type not supported", p.Type.String())
		}
		if !bytes.Equal(caPubk, censusRoot) {
			return false, nil, fmt.Errorf("ca bundle signature do not match")
		}
		return true, big.NewInt(1), nil

	case models.CensusOrigin_ERC20:
		p := proof.GetEthereumStorage()
		if p == nil {
			return false, nil, fmt.Errorf("ethereum proof is empty")
		}
		if !bytes.Equal(p.Key, key) {
			return false, nil, fmt.Errorf("proof key and leafData do not match (%x != %x)", p.Key, key)
		}
		hexproof := []string{}
		for _, s := range p.Siblings {
			hexproof = append(hexproof, fmt.Sprintf("%x", s))
		}
		amount := big.Int{}
		amount.SetBytes(p.Value)
		hexamount := hexutil.Big(amount)
		log.Debugf("validating erc20 storage proof for key %x and amount %s", p.Key, amount.String())
		valid, err := ethstorageproof.VerifyEthStorageProof(
			&ethstorageproof.StorageResult{Key: fmt.Sprintf("%x", p.Key), Proof: hexproof, Value: &hexamount},
			ethcommon.BytesToHash(censusRoot),
		)
		return valid, &amount, err
	}
	return false, nil, fmt.Errorf("proof type not supported for census origin %d", censusOrigin)
}

// VerifySignatureAgainstOracles verifies that a signature match with one of the oracles
func verifySignatureAgainstOracles(oracles []ethcommon.Address, message, signature []byte) (bool, ethcommon.Address, error) {
	signKeys := ethereum.NewSignKeys()
	for _, oracle := range oracles {
		signKeys.AddAuthKey(oracle)
	}
	return signKeys.VerifySender(message, signature)
}

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
func GenerateNullifier(address ethcommon.Address, processID []byte) []byte {
	return ethereum.HashRaw([]byte(fmt.Sprintf("%s%s", address.Bytes(), processID)))
}

// NewPrivateValidator returns a tendermint file private validator (key and state)
// if tmPrivKey not specified, uses the existing one or generates a new one
func NewPrivateValidator(tmPrivKey string, tconfig *cfg.Config) (*privval.FilePV, error) {
	pv := privval.LoadOrGenFilePV(
		tconfig.PrivValidatorKeyFile(),
		tconfig.PrivValidatorStateFile(),
	)
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
	keyBytes, err := hex.DecodeString(util.TrimHex(tmPrivKey))
	if err != nil {
		return nil, fmt.Errorf("cannot decode private key: (%s)", err)
	}
	nodeKey := &p2p.NodeKey{
		PrivKey: crypto25519.PrivKey(keyBytes),
	}
	// Write nodeKey to disk
	if err := nodeKey.SaveAs(tconfig.NodeKeyFile()); err != nil {
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
			Address: val.GetAddress().Bytes(),
			PubKey:  types.TendermintPubKey{Value: pubk.Bytes(), Type: "tendermint/PubKeyEd25519"},
			Power:   "10",
			Name:    strconv.Itoa(rand.Int()),
		}
	}
	appState.Oracles = oracles
	appStateBytes, err := tmjson.Marshal(appState)
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

	// Note that the genesis doc bytes are later consumed by tendermint,
	// which expects amino-flavored json. We can't use encoding/json.
	genBytes, err := tmjson.Marshal(genDoc)
	if err != nil {
		return []byte{}, err
	}

	return genBytes, nil
}
