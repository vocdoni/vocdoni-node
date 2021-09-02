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
	vkHash, err := hex.DecodeString("de3b651d4d47a956eb8e27cbc2e7d145e7677d68fddac275604dbc697690e751")
	qt.Assert(t, err, qt.IsNil)

	path := t.TempDir()
	c := CircuitsConfig{
		URL:          ts.URL,
		CircuitsPath: "/zkcensusproof/test/1024/",
		LocalDir:     path,
		WitnessHash:  witnessHash,
		ZKeyHash:     zkeyHash,
		VKHash:       vkHash,
	}

	ctx := context.Background()

	// 0. delete files if exist in tmp test path
	_ = os.Remove(filepath.Join(c.LocalDir, c.CircuitsPath, FilenameWitness))
	_ = os.Remove(filepath.Join(c.LocalDir, c.CircuitsPath, FilenameZKey))
	_ = os.Remove(filepath.Join(c.LocalDir, c.CircuitsPath, FilenameVK))

	// 1. Files don't exist yet, call DownloadCircuitFiles, then check
	// expected hashes
	err = DownloadCircuitFiles(ctx, c)
	qt.Assert(t, err, qt.IsNil)

	// 2. Remove one file and call DownloadCircuitFiles, check expected
	// hashes
	err = os.Remove(filepath.Join(c.LocalDir, c.CircuitsPath, FilenameWitness))
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
