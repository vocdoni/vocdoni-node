package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"go.vocdoni.io/dvote/log"
)

/*
# Directories structure
=======================
Path: circuitname/network/parameters/file.extension
- Circuit Name: the circuit name. Example: `zkcensusproof`
- Network: The network name. Example: `main, dev, stage`
- Circuit Parameters: numbers in decimal separated by underscore. Example: `1024`, `1024_3_500`
- Files:
	- circuit.zkey: contains the provingKey & verificationKey (big size, linear with circuit
	size) [needed by the user]
	- verificationKey.json: extracted from circuit.zkey (small size, constant size) [needed
	by the vochain]
	- witness.wasm: (medium size, linear with circuit size) [needed by the user]
- Examples:
	- zkcensusproof/stage/1024/circuit.zkey
	- zkcensusproof/dev/128/witness.wasm

# Files hash
=============
Each file is hashed with sha256, which output is publicly known. The hash
values of each file are hardcoded in the VochainGenesis.

*/

const (
	// FilenameWitness defines the name of the file of the WitnessCalculator
	FilenameWitness = "witness.wasm"
	// FilenameZKey defines the name of the file of the circom ZKey
	FilenameZKey = "circuit.zkey"
	// FilenameVK defines the name of the verification_key.json
	FilenameVK = "verification_key.json"
)

// CircuitConfig defines the configuration of the files to be downloaded
type CircuitConfig struct {
	// TODO: Replace by URI
	// URL defines the URL from where to download the files
	URL string `json:"url"`
	// CircuitPath defines the path from where the files are downloaded
	CircuitPath string `json:"circuitPath"`
	// Parameters used for the circuit build
	Parameters []int64 `json:"parameters"`
	// LocalDir defines in which directory will be the files
	// downloaded, under that directory it will follow the CircuitPath
	// directories structure
	LocalDir string `json:"localDir"`

	// WitnessHash contains the expected hash for the file filenameWitness
	WitnessHash []byte `json:"witnessHash"` // witness.wasm
	// ZKeyHash contains the expected hash for the file filenameZKey
	ZKeyHash []byte `json:"zKeyHash"` // circuit.zkey
	// VKHash contains the expected hash for the file filenameVK
	VKHash []byte `json:"vKHash"` // verificationKey.json
}

// DownloadCircuitFiles will download the circuits in the specified path,
// checking the expected sha256 hash of each file. If the files already exist
// in the path, it will not download them again but it will check the hash of
// the existing files.
func DownloadCircuitFiles(ctx context.Context, c CircuitConfig) error {
	// check if files already exist, if files already exist, check hashes
	err := checkHashes(c)
	if err == nil {
		// files already exist, and match the expected hashes
		log.Info("files already exist and match the expected hashes")
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	// at this point, or files do not exist, or exist with a not expected
	// hash
	log.Info("files or don't exist locally or exist with a not expected hash. Proceed with fresh download")
	return downloadFiles(ctx, c)
}

// DownloadVKFile will download the circuit VerificationKey in the specified
// path, checking the expected sha256 hash of the file. If the file already
// exist in the path, it will not download it again but it will check the hash
// of the existing file.
func DownloadVKFile(ctx context.Context, c CircuitConfig) error {
	err := checkHash(filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK), c.VKHash)
	if err == nil {
		// VK file already exist, and match the expected hash
		log.Info("VK file already exist and match the expected hash")
		return err
	}
	if !os.IsNotExist(err) {
		return err
	}

	log.Info("VK file or does not exist locally or exist with a not expected hash. Proceed with fresh download")
	// proceed to download the VK file
	if err := os.MkdirAll(filepath.Join(c.LocalDir, c.CircuitPath), os.ModePerm); err != nil {
		return err
	}
	u, err := url.Parse(c.URL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, c.CircuitPath, FilenameVK)
	if err := downloadFile(ctx,
		u.String(),
		filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK),
	); err != nil {
		return err
	}
	if err := checkHash(filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK),
		c.VKHash); err != nil {
		return fmt.Errorf("error on download VerificationKey (VK) file from %s, %s", c.URL, err)
	}
	log.Info("VK file downloaded correctly & match the expected hash")
	return nil
}

func checkHashes(c CircuitConfig) error {
	err := checkHash(filepath.Join(c.LocalDir, c.CircuitPath, FilenameWitness), c.WitnessHash)
	if err != nil {
		return err
	}

	err = checkHash(filepath.Join(c.LocalDir, c.CircuitPath, FilenameZKey), c.ZKeyHash)
	if err != nil {
		return err
	}

	err = checkHash(filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK), c.VKHash)
	if err != nil {
		return err
	}

	return nil
}

func checkHash(path string, expected []byte) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	hash := h.Sum(nil)

	if !bytes.Equal(hash[:], expected) {
		return fmt.Errorf("path: %s, expected hash: %x, computed hash: %x",
			path, expected, hash)
	}
	return nil
}

// downloadFiles downloads the files, and once downloaded calls checkHashes
func downloadFiles(ctx context.Context, c CircuitConfig) error {
	if err := os.MkdirAll(filepath.Join(c.LocalDir, c.CircuitPath), os.ModePerm); err != nil {
		return err
	}

	u, err := url.Parse(c.URL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, c.CircuitPath, FilenameWitness)
	if err := downloadFile(ctx,
		u.String(),
		filepath.Join(c.LocalDir, c.CircuitPath, FilenameWitness),
	); err != nil {
		return err
	}

	u, err = url.Parse(c.URL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, c.CircuitPath, FilenameZKey)
	if err := downloadFile(ctx,
		u.String(),
		filepath.Join(c.LocalDir, c.CircuitPath, FilenameZKey),
	); err != nil {
		return err
	}

	u, err = url.Parse(c.URL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, c.CircuitPath, FilenameVK)
	if err := downloadFile(ctx,
		u.String(),
		filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK),
	); err != nil {
		return err
	}

	if err := checkHashes(c); err != nil {
		return fmt.Errorf("error on download files from %s, %s", c.URL, err)
	}

	log.Info("files downloaded correctly & match the expected hashes")
	return nil
}

func downloadFile(ctx context.Context, fromUrl, toPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fromUrl, nil)
	if err != nil {
		return err
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error on download file %s: http status: %d", fromUrl, resp.StatusCode)
	}

	out, err := os.Create(toPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return out.Close()
}
