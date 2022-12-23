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

var testFiles = map[string][]byte{
	FilenameProvingKey:      []byte("proving_key_content"),
	FilenameVerificationKey: []byte("verification_key_content"),
	FilenameWasm:            []byte("wasm_content"),
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

	config := ZkCircuitConfig{
		URI:         server.URL,
		CircuitPath: "/test/",
		LocalDir:    "./test-files",
	}

	hashFn := sha256.New()
	hashFn.Write(testFiles[FilenameProvingKey])
	config.ProvingKeyHash = hashFn.Sum(nil)

	hashFn.Reset()
	hashFn.Write(testFiles[FilenameVerificationKey])
	config.VerificationKeyHash = hashFn.Sum(nil)

	hashFn.Reset()
	hashFn.Write(testFiles[FilenameWasm])
	config.WasmHash = hashFn.Sum(nil)

	testCircuits := filepath.Join(config.LocalDir, config.CircuitPath)
	defer os.RemoveAll(testCircuits)

	circuit, err := LoadZkCircuit(context.Background(), config)
	c.Assert(err, qt.IsNil)
	c.Assert(circuit.ProvingKey, qt.DeepEquals, testFiles[FilenameProvingKey])
	c.Assert(circuit.VerificationKey, qt.DeepEquals, testFiles[FilenameVerificationKey])
	c.Assert(circuit.Wasm, qt.DeepEquals, testFiles[FilenameWasm])

	localProvingKey, err := os.ReadFile(filepath.Join(testCircuits, FilenameProvingKey))
	c.Assert(err, qt.IsNil)
	c.Assert(localProvingKey, qt.DeepEquals, testFiles[FilenameProvingKey])
	localVerificationKey, err := os.ReadFile(filepath.Join(testCircuits, FilenameVerificationKey))
	c.Assert(err, qt.IsNil)
	c.Assert(localVerificationKey, qt.DeepEquals, testFiles[FilenameVerificationKey])
	localWasm, err := os.ReadFile(filepath.Join(testCircuits, FilenameWasm))
	c.Assert(err, qt.IsNil)
	c.Assert(localWasm, qt.DeepEquals, testFiles[FilenameWasm])
}

func TestLoadLocal(t *testing.T) {
	c := qt.New(t)

	circuit := &ZkCircuit{
		Config: ZkCircuitConfig{
			CircuitPath: "/test/",
			LocalDir:    "./test-files",
		},
	}

	// Create local parent folder
	testCircuits := filepath.Join(circuit.Config.LocalDir, circuit.Config.CircuitPath)
	err := os.MkdirAll(testCircuits, os.ModePerm)
	c.Assert(err, qt.IsNil)
	defer os.RemoveAll(testCircuits)

	// Try to get local files that not exists
	err = circuit.LoadLocal()
	c.Assert(err, qt.IsNotNil)

	// Write example files
	localProvingKey := filepath.Join(testCircuits, FilenameProvingKey)
	err = os.WriteFile(localProvingKey, testFiles[FilenameProvingKey], os.ModePerm)
	c.Assert(err, qt.IsNil)
	localVerificationKey := filepath.Join(testCircuits, FilenameVerificationKey)
	err = os.WriteFile(localVerificationKey, testFiles[FilenameVerificationKey], os.ModePerm)
	c.Assert(err, qt.IsNil)
	localWasm := filepath.Join(testCircuits, FilenameWasm)
	err = os.WriteFile(localWasm, testFiles[FilenameWasm], os.ModePerm)
	c.Assert(err, qt.IsNil)

	// Try to get local again
	err = circuit.LoadLocal()
	defer os.RemoveAll(testCircuits)
	c.Assert(err, qt.IsNil)
	c.Assert(circuit.ProvingKey, qt.DeepEquals, testFiles[FilenameProvingKey])
	c.Assert(circuit.VerificationKey, qt.DeepEquals, testFiles[FilenameVerificationKey])
	c.Assert(circuit.Wasm, qt.DeepEquals, testFiles[FilenameWasm])
}

func TestLoadRemote(t *testing.T) {
	c := qt.New(t)

	server := testFileServer(testFiles)
	defer server.Close()

	circuit := &ZkCircuit{
		Config: ZkCircuitConfig{
			URI:         server.URL,
			CircuitPath: "/test/",
			LocalDir:    "./test-files",
		},
	}

	// Success case
	ctx := context.Background()
	err := circuit.LoadRemote(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(circuit.ProvingKey, qt.DeepEquals, testFiles[FilenameProvingKey])
	c.Assert(circuit.VerificationKey, qt.DeepEquals, testFiles[FilenameVerificationKey])
	c.Assert(circuit.Wasm, qt.DeepEquals, testFiles[FilenameWasm])

	// Compare with the local copies
	testCircuits := filepath.Join(circuit.Config.LocalDir, circuit.Config.CircuitPath)
	localProvingKey, err := os.ReadFile(filepath.Join(testCircuits, FilenameProvingKey))
	c.Assert(err, qt.IsNil)
	c.Assert(localProvingKey, qt.DeepEquals, testFiles[FilenameProvingKey])
	localVerificationKey, err := os.ReadFile(filepath.Join(testCircuits, FilenameVerificationKey))
	c.Assert(err, qt.IsNil)
	c.Assert(localVerificationKey, qt.DeepEquals, testFiles[FilenameVerificationKey])
	localWasm, err := os.ReadFile(filepath.Join(testCircuits, FilenameWasm))
	c.Assert(err, qt.IsNil)
	c.Assert(localWasm, qt.DeepEquals, testFiles[FilenameWasm])

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
	delete(newFiles, FilenameProvingKey)
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
		ProvingKey:      testFiles[FilenameProvingKey],
		VerificationKey: testFiles[FilenameVerificationKey],
		Wasm:            testFiles[FilenameWasm],
		Config:          ZkCircuitConfig{},
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
	fileUrl := fmt.Sprintf("%s/%s", server.URL, FilenameProvingKey)
	expected := testFiles[FilenameProvingKey]
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
