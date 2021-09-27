package artifacts

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func newTestServer(t *testing.T) *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathSplit := strings.Split(r.URL.Path, "/")
		filename := pathSplit[len(pathSplit)-1]
		data := []byte("File test content of " + filename)
		readSeeker := bytes.NewReader(data)
		http.ServeContent(w, r, filename, time.Now(), readSeeker)
	}))
	return ts
}

func TestDownloadCircuitFiles(t *testing.T) {
	// create a test server that serves test files
	ts := newTestServer(t)
	defer ts.Close()

	witnessHash, err := hex.DecodeString("2ba5070e6fb17b920c1f938903c39867a2561900aaa2de52ec2265d46d41eb12")
	qt.Assert(t, err, qt.IsNil)
	zkeyHash, err := hex.DecodeString("279433966c3d258fa747121207b02baad715e12877473f9c2326da7f3a17597b")
	qt.Assert(t, err, qt.IsNil)
	vkHash, err := hex.DecodeString("3ec7355a7be53019b66563d02a1e2ce689e6dfac207a5f9ce503fb8ac915acf0")
	qt.Assert(t, err, qt.IsNil)

	path := t.TempDir()
	c := CircuitConfig{
		URL:         ts.URL,
		CircuitPath: "/zkcensusproof/test/1024/",
		LocalDir:    path,
		WitnessHash: witnessHash,
		ZKeyHash:    zkeyHash,
		VKHash:      vkHash,
	}

	ctx := context.Background()

	// 0. delete files if exist in tmp test path
	_ = os.Remove(filepath.Join(c.LocalDir, c.CircuitPath, FilenameWitness))
	_ = os.Remove(filepath.Join(c.LocalDir, c.CircuitPath, FilenameZKey))
	_ = os.Remove(filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK))

	// 1. Files don't exist yet, call DownloadCircuitFiles, then check
	// expected hashes
	err = DownloadCircuitFiles(ctx, c)
	qt.Assert(t, err, qt.IsNil)

	// 2. Remove one file and call DownloadCircuitFiles, check expected
	// hashes
	err = os.Remove(filepath.Join(c.LocalDir, c.CircuitPath, FilenameWitness))
	qt.Assert(t, err, qt.IsNil)
	err = DownloadCircuitFiles(ctx, c)
	qt.Assert(t, err, qt.IsNil)

	// checkHashes without download, as where download in the last step (2)
	err = checkHashes(c)
	qt.Assert(t, err, qt.IsNil)

	// 3. Call again DownloadCircuitFiles, expect no new download and check
	// expected hashes.
	// Stop the TestServer and call DownloadCircuitFiles, should not give
	// any error as the files already exist locally and match the expected
	// hashes
	ts.Close()
	err = DownloadCircuitFiles(ctx, c)
	qt.Assert(t, err, qt.IsNil)

	// run again the TestServer for the next step
	ts = newTestServer(t)
	defer ts.Close()

	// 4. Call DownloadCircuitFiles, but this time change one of the
	// expected hashes to not match.
	// change the expected hash of witness.wasm
	c.WitnessHash[0] += 1
	err = DownloadCircuitFiles(ctx, c)
	qt.Assert(t, err, qt.IsNotNil)
	errExpected := strings.Split(err.Error(), "wasm, ")[1]
	qt.Assert(t, errExpected, qt.Equals,
		"expected hash: 2ca5070e6fb17b920c1f938903c39867a2561900aaa2de52ec2265d46d41eb12, computed hash: 2ba5070e6fb17b920c1f938903c39867a2561900aaa2de52ec2265d46d41eb12")
}

func TestErrorDownloading(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	path := t.TempDir()
	c := CircuitConfig{
		URL:         ts.URL,
		CircuitPath: "/zkcensusproof/test/1024/",
		LocalDir:    path,
	}
	ctx := context.Background()
	err := DownloadCircuitFiles(ctx, c)
	qt.Assert(t, err, qt.IsNotNil)
	// qt.Assert(t, strings.Contains(err.Error(), "error on download file"), qt.IsTrue)
	qt.Assert(t, err, qt.ErrorMatches, "error on download file .* 404")
}

func TestDownloadVKFile(t *testing.T) {
	// create a test server that serves test files
	ts := newTestServer(t)
	defer ts.Close()

	vkHash, err := hex.DecodeString("3ec7355a7be53019b66563d02a1e2ce689e6dfac207a5f9ce503fb8ac915acf0")
	qt.Assert(t, err, qt.IsNil)

	path := t.TempDir()
	c := CircuitConfig{
		URL:         ts.URL,
		CircuitPath: "/zkcensusproof/test/1024/",
		LocalDir:    path,
		VKHash:      vkHash,
	}

	ctx := context.Background()

	// 0. delete files if exist in tmp test path
	_ = os.Remove(filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK))

	// 1. Files don't exist yet, call DownloadVKFile, then
	// check expected hashes
	err = DownloadVKFile(ctx, c)
	qt.Assert(t, err, qt.IsNil)

	// 2. Remove the VK file and call DownloadVKFile, check
	// expected hashes
	err = os.Remove(filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK))
	qt.Assert(t, err, qt.IsNil)
	err = DownloadVKFile(ctx, c)
	qt.Assert(t, err, qt.IsNil)

	// checkHashe without download, as was download in the last step (2)
	err = checkHash(filepath.Join(c.LocalDir, c.CircuitPath, FilenameVK), c.VKHash)
	qt.Assert(t, err, qt.IsNil)

	// 3. Call again DownloadVKFile, expect no new download
	// and check expected hashes.
	// Stop the TestServer and call DownloadVKFile, should not
	// give any error as the file already exist locally and match the
	// expected hash
	ts.Close()
	err = DownloadVKFile(ctx, c)
	qt.Assert(t, err, qt.IsNil)

	// run again the TestServer for the next step
	ts = newTestServer(t)
	defer ts.Close()

	// 4. Call DownloadVKFile, but this time change one of the
	// expected hashes to not match.
	// change the expected hash of witness.wasm
	c.VKHash[0] += 1
	err = DownloadVKFile(ctx, c)
	qt.Assert(t, err, qt.IsNotNil)
	errExpected := strings.Split(err.Error(), FilenameVK+", ")[1]
	qt.Assert(t, errExpected, qt.Equals,
		"expected hash: 3fc7355a7be53019b66563d02a1e2ce689e6dfac207a5f9ce503fb8ac915acf0, computed hash: 3ec7355a7be53019b66563d02a1e2ce689e6dfac207a5f9ce503fb8ac915acf0")
}
