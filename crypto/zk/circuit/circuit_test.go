package circuit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

const (
	testProvingKey      = "proving_key.zkey"
	testVerificationKey = "verification_key.json"
	testWasm            = "circuit.wasm"
)

var testFiles = map[string][]byte{
	testProvingKey:      []byte("proving_key_content"),
	testVerificationKey: []byte("verification_key_content"),
	testWasm:            []byte("wasm_content"),
}

func testFileServer(files map[string][]byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := path.Base(r.URL.Path)
		if content, ok := files[filename]; ok {
			http.ServeContent(w, r, filename, time.Now(), bytes.NewReader(content))
			return
		}

		http.Error(w, "file not found", http.StatusNotFound)
	}))
}

func TestLoadZkCircuit(t *testing.T) {
	c := qt.New(t)

	server := testFileServer(testFiles)
	defer server.Close()

	config := &Config{
		URI:                     server.URL,
		CircuitPath:             "/test/",
		ProvingKeyFilename:      testProvingKey,
		VerificationKeyFilename: testVerificationKey,
		WasmFilename:            testWasm,
	}

	hashFn := sha256.New()
	hashFn.Write(testFiles[testProvingKey])
	config.ProvingKeyHash = hashFn.Sum(nil)

	hashFn.Reset()
	hashFn.Write(testFiles[testVerificationKey])
	config.VerificationKeyHash = hashFn.Sum(nil)

	hashFn.Reset()
	hashFn.Write(testFiles[testWasm])
	config.WasmHash = hashFn.Sum(nil)

	testCircuits := filepath.Join(BaseDir, config.CircuitPath)
	defer os.RemoveAll(testCircuits)

	circuit, err := LoadConfig(context.Background(), config)
	c.Assert(err, qt.IsNil)
	c.Assert(circuit.ProvingKey, qt.DeepEquals, testFiles[testProvingKey])
	c.Assert(circuit.VerificationKey, qt.DeepEquals, testFiles[testVerificationKey])
	c.Assert(circuit.Wasm, qt.DeepEquals, testFiles[testWasm])

	localProvingKey, err := os.ReadFile(filepath.Join(testCircuits, testProvingKey))
	c.Assert(err, qt.IsNil)
	c.Assert(localProvingKey, qt.DeepEquals, testFiles[testProvingKey])
	localVerificationKey, err := os.ReadFile(filepath.Join(testCircuits, testVerificationKey))
	c.Assert(err, qt.IsNil)
	c.Assert(localVerificationKey, qt.DeepEquals, testFiles[testVerificationKey])
	localWasm, err := os.ReadFile(filepath.Join(testCircuits, testWasm))
	c.Assert(err, qt.IsNil)
	c.Assert(localWasm, qt.DeepEquals, testFiles[testWasm])
}

func TestLoadLocal(t *testing.T) {
	c := qt.New(t)

	circuit := &ZkCircuit{
		Config: &Config{
			CircuitPath:             "/test/",
			ProvingKeyFilename:      testProvingKey,
			VerificationKeyFilename: testVerificationKey,
			WasmFilename:            testWasm,
		},
	}

	// Create local parent folder
	testCircuits := filepath.Join(BaseDir, circuit.Config.CircuitPath)
	err := os.MkdirAll(testCircuits, os.ModePerm)
	c.Assert(err, qt.IsNil)
	defer os.RemoveAll(testCircuits)

	// Try to get local files that not exists
	err = circuit.LoadLocal()
	c.Assert(err, qt.IsNotNil)

	// Write example files
	localProvingKey := filepath.Join(testCircuits, testProvingKey)
	err = os.WriteFile(localProvingKey, testFiles[testProvingKey], os.ModePerm)
	c.Assert(err, qt.IsNil)
	localVerificationKey := filepath.Join(testCircuits, testVerificationKey)
	err = os.WriteFile(localVerificationKey, testFiles[testVerificationKey], os.ModePerm)
	c.Assert(err, qt.IsNil)
	localWasm := filepath.Join(testCircuits, testWasm)
	err = os.WriteFile(localWasm, testFiles[testWasm], os.ModePerm)
	c.Assert(err, qt.IsNil)

	// Try to get local again
	err = circuit.LoadLocal()
	defer os.RemoveAll(testCircuits)
	c.Assert(err, qt.IsNil)
	c.Assert(circuit.ProvingKey, qt.DeepEquals, testFiles[testProvingKey])
	c.Assert(circuit.VerificationKey, qt.DeepEquals, testFiles[testVerificationKey])
	c.Assert(circuit.Wasm, qt.DeepEquals, testFiles[testWasm])
}

func TestLoadRemote(t *testing.T) {
	c := qt.New(t)

	server := testFileServer(testFiles)
	defer server.Close()

	circuit := &ZkCircuit{
		Config: &Config{
			URI:                     server.URL,
			CircuitPath:             "/test/",
			ProvingKeyFilename:      testProvingKey,
			VerificationKeyFilename: testVerificationKey,
			WasmFilename:            testWasm,
		},
	}

	// Success case
	ctx := context.Background()
	err := circuit.LoadRemote(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(circuit.ProvingKey, qt.DeepEquals, testFiles[testProvingKey])
	c.Assert(circuit.VerificationKey, qt.DeepEquals, testFiles[testVerificationKey])
	c.Assert(circuit.Wasm, qt.DeepEquals, testFiles[testWasm])

	// Compare with the local copies
	testCircuits := filepath.Join(BaseDir, circuit.Config.CircuitPath)
	localProvingKey, err := os.ReadFile(filepath.Join(testCircuits, testProvingKey))
	c.Assert(err, qt.IsNil)
	c.Assert(localProvingKey, qt.DeepEquals, testFiles[testProvingKey])
	localVerificationKey, err := os.ReadFile(filepath.Join(testCircuits, testVerificationKey))
	c.Assert(err, qt.IsNil)
	c.Assert(localVerificationKey, qt.DeepEquals, testFiles[testVerificationKey])
	localWasm, err := os.ReadFile(filepath.Join(testCircuits, testWasm))
	c.Assert(err, qt.IsNil)
	c.Assert(localWasm, qt.DeepEquals, testFiles[testWasm])

	// Clean stored files
	err = os.RemoveAll(testCircuits)
	c.Assert(err, qt.IsNil)
	defer os.RemoveAll(testCircuits)

	// Server closed error
	server.Close()
	err = circuit.LoadRemote(ctx)
	c.Assert(err, qt.IsNotNil)

	newFiles := make(map[string][]byte)
	for k, v := range testFiles {
		newFiles[k] = v
	}
	delete(newFiles, testProvingKey)
	server = testFileServer(newFiles)
	defer server.Close()

	// Not found file error
	err = circuit.LoadRemote(ctx)
	c.Assert(err, qt.IsNotNil)
}

func TestVerifiedCircuitArtifacts(t *testing.T) {
	c := qt.New(t)

	circuit := new(ZkCircuit)
	res, err := circuit.VerifiedCircuitArtifacts()
	c.Assert(err, qt.IsNotNil)
	c.Assert(res, qt.IsFalse)

	circuit = &ZkCircuit{
		ProvingKey:      testFiles[testProvingKey],
		VerificationKey: testFiles[testVerificationKey],
		Wasm:            testFiles[testWasm],
		Config:          &Config{},
	}

	hashFn := sha256.New()
	hashFn.Write(circuit.ProvingKey)
	circuit.Config.ProvingKeyHash = hashFn.Sum(nil)

	hashFn.Reset()
	hashFn.Write(circuit.VerificationKey)
	circuit.Config.VerificationKeyHash = hashFn.Sum(nil)

	hashFn.Reset()
	hashFn.Write(circuit.Wasm)
	circuit.Config.WasmHash = hashFn.Sum(nil)

	res, err = circuit.VerifiedCircuitArtifacts()
	c.Assert(err, qt.IsNil)
	c.Assert(res, qt.IsTrue)

	hashFn.Write(circuit.Wasm)
	circuit.Config.WasmHash = hashFn.Sum(nil)
	res, err = circuit.VerifiedCircuitArtifacts()
	c.Assert(err, qt.IsNil)
	c.Assert(res, qt.IsFalse)
}

func Test_checkHash(t *testing.T) {
	c := qt.New(t)

	needle := []byte("test")
	s256 := sha256.New()
	s256.Write(needle)
	expected := s256.Sum(nil)

	// Success case
	equal, err := checkHash(needle, expected)
	c.Assert(err, qt.IsNil)
	c.Assert(equal, qt.IsTrue)

	// Content does not match with the hash provided
	equal, err = checkHash([]byte("wrong"), expected)
	c.Assert(err, qt.IsNil)
	c.Assert(equal, qt.IsFalse)

	// No content provided error
	equal, err = checkHash(nil, expected)
	c.Assert(err, qt.IsNotNil)
	c.Assert(equal, qt.IsFalse)

	// No expected hash provided error
	equal, err = checkHash(needle, nil)
	c.Assert(err, qt.IsNotNil)
	c.Assert(equal, qt.IsFalse)
}

func Test_downloadFile(t *testing.T) {
	c := qt.New(t)

	ctx := context.Background()
	server := testFileServer(testFiles)

	// Success case
	fileUrl := fmt.Sprintf("%s/%s", server.URL, testProvingKey)
	expected := testFiles[testProvingKey]
	res, err := downloadFile(ctx, fileUrl)
	c.Assert(err, qt.IsNil)
	c.Assert(res, qt.DeepEquals, expected)

	// No existing file
	fileUrl = fmt.Sprintf("%s/%s", server.URL, "no-found.file")
	_, err = downloadFile(ctx, fileUrl)
	c.Assert(err, qt.IsNotNil)

	// Server closed
	server.Close()
	_, err = downloadFile(ctx, fileUrl)
	c.Assert(err, qt.IsNotNil)
}

func Test_storeFile(t *testing.T) {
	c := qt.New(t)

	// Success case
	testContent, testPath := []byte("test_content"), filepath.Join(os.TempDir(), "test.txt")
	err := storeFile(testContent, testPath)
	c.Assert(err, qt.IsNil)

	result, err := os.ReadFile(testPath)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.DeepEquals, testContent)

	err = os.Remove(testPath)
	c.Assert(err, qt.IsNil)
	defer os.RemoveAll(testPath)

	// No content provided error
	err = storeFile(nil, testPath)
	c.Assert(err, qt.IsNotNil)

	// Non existing destination path error
	err = storeFile(testContent, "/test-files/test.txt")
	c.Assert(err, qt.IsNotNil)
	defer os.RemoveAll("./test-files")

	// Wrong destination path error
	err = storeFile(testContent, "/test.text")
	c.Assert(err, qt.IsNotNil)
	defer os.RemoveAll("./test.txt")
}
