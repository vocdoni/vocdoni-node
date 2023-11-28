package circuit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/log"
)

const downloadCircuitsTimeout = time.Minute * 5

// BaseDir is where the artifact cache is expected to be found.
// If the artifacts are not found there, they will be downloaded and stored.
//
// Defaults to ~/.cache/vocdoni/zkCircuits/
//
// In any case, the LocalDir path associated with the circuit config will be appended at the end
var BaseDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(home, ".cache", "vocdoni", "zkCircuits")
}()

// Global circuit
var (
	mtx sync.Mutex

	globalCircuit = &ZkCircuit{
		Config: CircuitsConfigurations[DefaultZkCircuitVersion],
	}
)

// ZkCircuit struct wraps the circuit configuration and contains the file
// content of the circuit artifacts (provingKey, verificationKey and wasm)
type ZkCircuit struct {
	ProvingKey      []byte
	VerificationKey []byte
	Wasm            []byte
	Config          *Config
}

// Global returns the global ZkCircuit
func Global() *ZkCircuit {
	mtx.Lock()
	defer mtx.Unlock()
	return globalCircuit
}

// SetGlobal will LoadVersion into the global ZkCircuit
//
// If current version is already equal to the passed version, and the artifacts are loaded into memory,
// it returns immediately
func SetGlobal(version string) error {
	mtx.Lock()
	defer mtx.Unlock()
	if globalCircuit.Version() == version && globalCircuit.IsLoaded() {
		return nil
	}
	circuit, err := LoadVersion(version)
	if err != nil {
		return fmt.Errorf("could not load zk verification keys: %w", err)
	}
	globalCircuit = circuit
	return nil
}

// Version returns the version of the global ZkCircuit
func Version() string {
	return Global().Version()
}

// IsLoaded returns true if all needed keys (Proving, Verification and Wasm) are loaded into memory
func IsLoaded() bool {
	return Global().IsLoaded()
}

// Init will load (or download) the default circuit artifacts into memory, ready to be used globally.
func Init() error {
	return SetGlobal(DefaultZkCircuitVersion)
}

// DownloadDefaultArtifacts ensures the default circuit is cached locally
func DownloadDefaultArtifacts() error {
	_, err := LoadVersion(DefaultZkCircuitVersion)
	if err != nil {
		return fmt.Errorf("could not load zk verification keys: %w", err)
	}
	return nil
}

// DownloadArtifactsForChainID ensures all circuits needed for chainID are cached locally
func DownloadArtifactsForChainID(chainID string) error {
	if config.ForksForChainID(chainID).VoceremonyForkBlock > 0 {
		_, err := LoadVersion(PreVoceremonyForkZkCircuitVersion)
		if err != nil {
			return fmt.Errorf("could not load zk verification keys: %w", err)
		}
	}
	return DownloadDefaultArtifacts()
}

// LoadVersion loads the circuit artifacts based on the version provided.
// First, tries to load the artifacts from local storage, if they are not
// available, tries to download from their remote location.
//
// Stores the loaded circuit in the global variable, and returns it as well
func LoadVersion(version string) (*ZkCircuit, error) {
	circuitConf := GetCircuitConfiguration(version)
	ctx, cancel := context.WithTimeout(context.Background(), downloadCircuitsTimeout)
	defer cancel()
	return LoadConfig(ctx, circuitConf)
}

// LoadConfig loads the circuit artifacts based on the configuration provided.
// First, tries to load the artifacts from local storage, if they are not
// available, tries to download from their remote location.
//
// Stores the loaded circuit in the global variable, and returns it as well
func LoadConfig(ctx context.Context, config *Config) (*ZkCircuit, error) {
	circuit := &ZkCircuit{Config: config}
	// load the artifacts of the provided circuit from the local storage
	if err := circuit.LoadLocal(); err == nil {
		// tries to verify the loaded artifacts, if it success, returns the
		// loaded circuit, else continue.
		correct, err := circuit.VerifiedCircuitArtifacts()
		if err == nil && correct {
			return circuit, nil
		}
	}
	// if the circuit is not available locally, tries to download from its
	// remote location
	if err := circuit.LoadRemote(ctx); err != nil {
		return nil, err
	}
	// checks hashes of current files
	correct, err := circuit.VerifiedCircuitArtifacts()
	if err != nil {
		return nil, err
	}
	if !correct {
		return nil, fmt.Errorf("hashes from downloaded artifacts don't match the expected ones")
	}
	globalCircuit = circuit
	return circuit, nil
}

// Version returns the version of the ZkCircuit
func (circuit *ZkCircuit) Version() string {
	return circuit.Config.Version
}

// IsLoaded returns true if all needed keys (Proving, Verification and Wasm) are loaded into memory
func (circuit *ZkCircuit) IsLoaded() bool {
	return (circuit.ProvingKey != nil &&
		circuit.VerificationKey != nil &&
		circuit.Wasm != nil)
}

// LoadLocal tries to read the content of current circuit artifacts from its
// local path (provingKey, verificationKey and wasm). If any of the read
// operations fails, returns an error.
func (circuit *ZkCircuit) LoadLocal() error {
	var err error
	log.Debugw("loading circuit locally...", "BaseDir", BaseDir, "version", circuit.Config.Version)
	files := map[string][]byte{
		circuit.Config.ProvingKeyFilename:      nil,
		circuit.Config.VerificationKeyFilename: nil,
		circuit.Config.WasmFilename:            nil,
	}
	for filename := range files {
		// compose files localpath
		localPath := filepath.Join(BaseDir,
			circuit.Config.CircuitPath, filename)
		// read file contents locally
		files[filename], err = os.ReadFile(localPath)
		if err != nil {
			return fmt.Errorf("error reading '%s' artifact locally: %w", filename, err)
		}
	}
	// store the content into ZkCircuit struct
	circuit.ProvingKey = files[circuit.Config.ProvingKeyFilename]
	circuit.VerificationKey = files[circuit.Config.VerificationKeyFilename]
	circuit.Wasm = files[circuit.Config.WasmFilename]
	return nil
}

// LoadRemote downloads the content of the current circuit artifacts from its
// remote location. If any of the downloads fails, returns an error.
func (circuit *ZkCircuit) LoadRemote(ctx context.Context) error {
	log.Debugw("circuit not downloaded yet, downloading...",
		"BaseDir", BaseDir, "version", circuit.Config.Version)
	baseUri, err := url.Parse(circuit.Config.URI)
	if err != nil {
		return err
	}
	remoteUri := baseUri.JoinPath(circuit.Config.CircuitPath)
	localPath := filepath.Join(BaseDir, circuit.Config.CircuitPath)
	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		return err
	}
	files := map[string][]byte{
		circuit.Config.ProvingKeyFilename:      nil,
		circuit.Config.VerificationKeyFilename: nil,
		circuit.Config.WasmFilename:            nil,
	}
	for filename := range files {
		// Compose the artifact uri and download it
		file, err := downloadFile(ctx, remoteUri.JoinPath(filename).String())
		if err != nil {
			return fmt.Errorf("error downloading '%s' artifact: %w", filename, err)
		}
		// Compose the local path for the artifact and store it
		if err := storeFile(file, filepath.Join(localPath, filename)); err != nil {
			return fmt.Errorf("error storing '%s' artifact: %w", filename, err)
		}
		// Also store its content into the map to update the ZkCircuit struct
		files[filename] = file
	}
	// Store the downloaded artifacts into the ZkCircuit struct
	circuit.ProvingKey = files[circuit.Config.ProvingKeyFilename]
	circuit.VerificationKey = files[circuit.Config.VerificationKeyFilename]
	circuit.Wasm = files[circuit.Config.WasmFilename]
	return nil
}

// VerifiedCircuitArtifacts checks that the computed hash of every circuit
// artifact matches with the expected hash, from the circuit config.
func (circuit *ZkCircuit) VerifiedCircuitArtifacts() (bool, error) {
	if circuit.ProvingKey == nil || circuit.Config.ProvingKeyHash == nil {
		return false, fmt.Errorf("provingKey or its hash are nil")
	}
	if circuit.VerificationKey == nil || circuit.Config.VerificationKeyHash == nil {
		return false, fmt.Errorf("verificationKey or its hash are nil")
	}
	if circuit.Wasm == nil || circuit.Config.WasmHash == nil {
		return false, fmt.Errorf("wasm or its hash are nil")
	}
	filesToCheck := []struct{ hash, content []byte }{
		{hash: circuit.Config.ProvingKeyHash, content: circuit.ProvingKey},
		{hash: circuit.Config.VerificationKeyHash, content: circuit.VerificationKey},
		{hash: circuit.Config.WasmHash, content: circuit.Wasm},
	}
	for _, file := range filesToCheck {
		verified, err := checkHash(file.content, file.hash)
		if err != nil {
			return false, err
		}
		if !verified {
			return false, nil
		}
	}
	return true, nil
}

// checkHash compute the hash of the content provided and compares it with the
// hash provided as expected result. It returns a boolean with the result of the
// comparation and with an error.
func checkHash(content, expected []byte) (bool, error) {
	if content == nil {
		return false, fmt.Errorf("no content provided to check")
	}
	if expected == nil {
		return false, fmt.Errorf("no hash provided to compare")
	}
	hash := sha256.New()
	if _, err := hash.Write(content); err != nil {
		return false, fmt.Errorf("error computing hash function of %s: %w", content, err)
	}
	return bytes.Equal(hash.Sum(nil), expected), nil
}

// downloadFile performs a GET request to the URL provided and returns the
// content of the received response. If something fails returns an error.
func downloadFile(ctx context.Context, fileUrl string) ([]byte, error) {
	if _, err := url.Parse(fileUrl); err != nil {
		return nil, fmt.Errorf("error parsing the file URL provided: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating the file request: %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Warnf("error closing body response %v", err)
		}
	}()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error on download file %s: http status: %d", fileUrl, res.StatusCode)
	}
	content, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading the file content from the http response: %w", err)
	}
	return content, nil
}

// storeFile helper function allows to write the file content provided into a
// new file created at the path provided.
func storeFile(content []byte, dstPath string) error {
	if content == nil {
		return fmt.Errorf("no content provided")
	}
	if _, err := os.Stat(filepath.Dir(dstPath)); err != nil {
		return fmt.Errorf("destination path parent folder does not exist")
	}
	fd, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("error creating the artifact file: %w", err)
	}
	if _, err := fd.Write(content); err != nil {
		return fmt.Errorf("error writing the artifact file: %w", err)
	}
	return nil
}
