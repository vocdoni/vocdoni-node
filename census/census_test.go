package census

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/proto/build/go/models"
)

func TestCompressor(t *testing.T) {
	t.Parallel()

	comp := newCompressor()
	input := []byte(strings.Repeat("foo bar baz", 10))

	// First, check that "decompressing" non-compressed bytes is a no-op,
	// for backwards compatibility with gateways, and to have a sane
	// fallback.
	qt.Assert(t, comp.decompressBytes(input), qt.DeepEquals, input)

	// Compressing should give a smaller size, at least by 50%.
	compressed := comp.compressBytes(input)
	qt.Assert(t, len(compressed) < len(input)/2, qt.IsTrue, qt.Commentf("expected size of 50%% at most, got %d out of %d", len(compressed), len(input)))

	// Decompressing should give us the original input back.
	qt.Assert(t, comp.decompressBytes(compressed), qt.DeepEquals, input)
}

func TestMemoryUsage(t *testing.T) {
	var cm Manager
	qt.Assert(t, cm.Start(t.TempDir(), ""), qt.IsNil)
	defer func() { qt.Assert(t, cm.Stop(), qt.IsNil) }()

	const n = 16
	var start, end runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&start)
	for i := 0; i < n; i++ {
		_, err := cm.LoadTree(fmt.Sprintf("%v", i), models.Census_ARBO_BLAKE2B)
		qt.Assert(t, err, qt.IsNil)
	}
	runtime.ReadMemStats(&end)
	_, _, loaded := cm.Count()
	qt.Assert(t, loaded, qt.Equals, n)
	alloc := end.Alloc - start.Alloc
	// Assert that loading 16 censuses doesn't consume more than 1 GiB of memory
	const maxAlloc = 1 * 1024 * 1024 * 1024
	qt.Assert(t, alloc < maxAlloc, qt.Equals, true,
		qt.Commentf("(got) %v MiB > (max) %v MiB", alloc/1024/1024, maxAlloc/1024/1024))
}

var nsConfigJSON1 = `
{
  "rootKey": "",
  "namespaces": [
    {
      "type": 0,
      "name": "dB5Cfb98c52a0a396F50087a882dCB10d26cB984/39f8da5676cadec08999ac5ddd72a2b13649c4438fe24a57329fb5229a09cdb9",
      "keys": [
        "0x04044abfd3f1cf573f4c9fb0eaf3bb1b1e1fd0f89dde12dce494ae69d7589feebfbe243efb8d2258902bea75ebcb470a3eb3e577300a896507047af91b90dfab5f"
      ]
    },
    {
      "type": 0,
      "name": "44f90c48101a69aa5bcaea4a548585c5a2e853c988a12e720f74915d31c3c7a5",
      "keys": null
    }
  ]
}
	`

func TestOutdatedCensuses(t *testing.T) {
	// Create a structure of 2 outdated census types, saved in namespaces.json
	storageDir := t.TempDir()
	time.Sleep(1 * time.Second)
	nsConfig := filepath.Join(storageDir, "namespaces.json")
	qt.Assert(t, ioutil.WriteFile(nsConfig, []byte(nsConfigJSON1), 0o600), qt.IsNil)

	var namespaces Namespaces
	qt.Assert(t, json.Unmarshal([]byte(nsConfigJSON1), &namespaces), qt.IsNil)
	for _, v := range namespaces.Namespaces {
		qt.Assert(t, os.MkdirAll(filepath.Join(storageDir, v.Name), 0o700), qt.IsNil)
	}

	// Init Manager, and expect the outdated censuses to be moved,
	// namespaces.json to be backed up and cleaned
	var cm Manager
	qt.Assert(t, cm.Start(storageDir, ""), qt.IsNil)
	defer func() { qt.Assert(t, cm.Stop(), qt.IsNil) }()

	// Assert correct namespaces.json backup
	matches, err := filepath.Glob(filepath.Join(storageDir, "namespaces.*.json"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(matches), qt.Equals, 1)

	nsConfigBackupJSON, err := os.ReadFile(matches[0])
	qt.Assert(t, err, qt.IsNil)
	var namespacesBackup Namespaces
	qt.Assert(t, json.Unmarshal([]byte(nsConfigBackupJSON), &namespacesBackup), qt.IsNil)
	qt.Assert(t, namespacesBackup, qt.DeepEquals, namespaces)

	// Assert correnct namespaces.json cleaned
	nsConfigJSON, err := os.ReadFile(nsConfig)
	qt.Assert(t, err, qt.IsNil)
	var namespacesClean Namespaces
	qt.Assert(t, json.Unmarshal([]byte(nsConfigJSON), &namespacesClean), qt.IsNil)
	qt.Assert(t, namespacesClean, qt.DeepEquals, Namespaces{})

	// Assert outdated censuses moved to outdated folder
	matches, err = filepath.Glob(filepath.Join(storageDir, "outdated.*/v0"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(matches), qt.Equals, 1)
	_, err = os.Stat(filepath.Join(matches[0], namespaces.Namespaces[0].Name))
	qt.Assert(t, err, qt.IsNil)
	_, err = os.Stat(filepath.Join(matches[0], namespaces.Namespaces[1].Name))
	qt.Assert(t, err, qt.IsNil)
}

func TestManager(t *testing.T) {
	storageDir := t.TempDir()
	var cm Manager
	qt.Assert(t, cm.Start(storageDir, ""), qt.IsNil)

	// Add some censues with some keys
	const numCensuses = 3
	const numKeys = 4
	keyValues := [numCensuses][numKeys][2][]byte{}
	for i := 0; i < numCensuses; i++ {
		tree, err := cm.AddNamespace(fmt.Sprintf("%v", i), models.Census_ARBO_BLAKE2B, []string{})
		qt.Assert(t, err, qt.IsNil)
		for j := 0; j < numKeys; j++ {
			key := ethcrypto.Keccak256([]byte(fmt.Sprintf("key%v.%v", i, j)))
			value := ethcrypto.Keccak256([]byte(fmt.Sprintf("value%v.%v", i, j)))
			qt.Assert(t, tree.Add(key, value), qt.IsNil)
			keyValues[i][j] = [2][]byte{key, value}
		}
	}

	// Close and open again the Manager, and read back the censuses and keys
	qt.Assert(t, cm.Stop(), qt.IsNil)
	cm = Manager{}
	qt.Assert(t, cm.Start(storageDir, ""), qt.IsNil)

	for i := 0; i < numCensuses; i++ {
		tree, err := cm.LoadTree(fmt.Sprintf("%v", i), models.Census_ARBO_BLAKE2B)
		qt.Assert(t, err, qt.IsNil)
		kvs := make([][2][]byte, 0)
		tree.IterateLeaves(func(key, value []byte) bool {
			kvs = append(kvs, [2][]byte{key, value})
			return false
		})
		qt.Assert(t, kvs, qt.ContentEquals, keyValues[i][:])
	}
}
