package vochain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"

	"github.com/cavaliergopher/grab/v3"
	ethcommon "github.com/ethereum/go-ethereum/common"

	cfg "github.com/tendermint/tendermint/config"
	crypto25519 "github.com/tendermint/tendermint/crypto/ed25519"
	tmjson "github.com/tendermint/tendermint/libs/json"

	// tmtime "github.com/tendermint/tendermint/libs/time" TENDERMINT 0.35
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"go.vocdoni.io/proto/build/go/models"
)

// GenerateNullifier generates the nullifier of a vote (hash(address+processId))
// This function assumes address and processID are correct.
func GenerateNullifier(address ethcommon.Address, processID []byte) []byte {
	nullifier := bytes.Buffer{}
	nullifier.Write(address.Bytes())
	nullifier.Write(processID)
	return ethereum.HashRaw(nullifier.Bytes())
}

/* TENDERMINT 0.35
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
*/

// NewPrivateValidator returns a tendermint file private validator (key and state)
// if tmPrivKey not specified, uses the existing one or generates a new one
func NewPrivateValidator(tmPrivKey string, tconfig *cfg.Config) (*privval.FilePV, error) {
	stateFile := &privval.FilePVLastSignState{}
	f, err := os.OpenFile(tconfig.PrivValidatorStateFile(), os.O_RDONLY|os.O_CREATE, 0o666)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Error(err)
		}
	}()

	stateJSONBytes, err := os.ReadFile(tconfig.PrivValidatorStateFile())
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(stateJSONBytes, stateFile)
	if err != nil || len(stateJSONBytes) == 0 {
		log.Debugf("priv_validator_state.json is empty, adding default values")
		jsonBytes, err := tmjson.MarshalIndent(stateFile, "", "  ")
		if err != nil {
			log.Fatalf("cannot create priv_validator_state.json: %s", err)
		}
		err = os.WriteFile(tconfig.PrivValidatorStateFile(), jsonBytes, 0o600)
		if err != nil {
			log.Fatalf("cannot create priv_validator_state.json: %s", err)
		}
	}

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
			return nil, fmt.Errorf("incorrect private key length (got %d, need 64)", n)
		}
		pv.Key.Address = privKey.PubKey().Address()
		pv.Key.PrivKey = privKey
		pv.Key.PubKey = privKey.PubKey()
	}
	return pv, nil
}

/* TENDERMINT 0.35
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
*/

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
func NewGenesis(cfg *config.VochainCfg, chainID string, consensusParams *ConsensusParams,
	validators []privval.FilePV, oracles []string, treasurer string) ([]byte, error) {
	// default consensus params
	appState := new(GenesisAppState)
	appState.Validators = make([]GenesisValidator, len(validators))
	for idx, val := range validators {
		pubk, err := val.GetPubKey()
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
	tb, err := hex.DecodeString(util.TrimHex(treasurer))
	if err != nil {
		return nil, err
	}
	appState.Treasurer = tb
	appStateBytes, err := tmjson.Marshal(appState)
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

// verifySignatureAgainstOracles verifies that a signature match with one of the oracles
func verifySignatureAgainstOracles(oracles []ethcommon.Address, message,
	signature []byte) (bool, ethcommon.Address, error) {
	signKeys := ethereum.NewSignKeys()
	for _, oracle := range oracles {
		signKeys.AddAuthKey(oracle)
	}
	return signKeys.VerifySender(message, signature)
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

func printPrettierDelegates(delegates [][]byte) []string {
	prettierDelegates := make([]string, len(delegates))
	for _, delegate := range delegates {
		prettierDelegates = append(prettierDelegates, ethcommon.BytesToAddress(delegate).String())
	}
	return prettierDelegates
}

// Untar decompress a tar archived file into a given destination
func Untar(src string, destination string) error {
	r, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open file %s: %w", src, err)
	}
	uncompressedStream, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("cannot read file %s: %w", src, err)
	}
	// untar
	tr := tar.NewReader(uncompressedStream)
	// uncompress each element
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := header.Name
		// validate name against path traversal
		if header.Name == "" || strings.Contains(header.Name, `\`) ||
			strings.HasPrefix(header.Name, "/") || strings.Contains(header.Name, "../") {
			return fmt.Errorf("tar contained invalid name error %q", target)
		}
		// add dst + re-format slashes according to system
		target = filepath.Join(destination, header.Name)
		// check the type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it (with 0755 permission)
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		// if it's a file create it (with same permission)
		case tar.TypeReg:
			fileToWrite, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			// copy over contents
			if _, err := io.Copy(fileToWrite, tr); err != nil {
				return err
			}
			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			fileToWrite.Close()
		}
	}
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}

// FetchFile downloads a file from a given URL
func FetchFile(destDir, url string) (string, error) {
	if err := os.MkdirAll(destDir, 0o777); err != nil {
		return "", fmt.Errorf("cannot create destination directory: %w", err)
	}
	client := grab.NewClient()
	req, _ := grab.NewRequest(destDir, url)
	log.Infof("downloading from %v...", req.URL())
	resp := client.Do(req)
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()
loop:
	for {
		select {
		case <-t.C:
			log.Infof("transferred %d / %d bytes (%.2f%%)",
				resp.BytesComplete(),
				resp.Size(),
				100*resp.Progress(),
			)

		case <-resp.Done:
			// download complete
			break loop
		}
	}
	if err := resp.Err(); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	log.Infof("%s saved at %s", filepath.Base(resp.Filename), destDir)
	return resp.Filename, nil
}
