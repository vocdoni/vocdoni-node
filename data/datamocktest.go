package data

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
)

// DataMockTest is a mock data provider for testing purposes.
type DataMockTest struct {
	files   map[string]string
	filesMu sync.RWMutex
	prefix  string
	rnd     testutil.Random
}

func (d *DataMockTest) Init(ds *types.DataStore) error {
	d.files = make(map[string]string)
	d.prefix = "mock://"
	d.rnd = testutil.NewRandom(0)
	return nil
}

func (d *DataMockTest) Publish(ctx context.Context, o []byte) (string, error) {
	d.filesMu.RLock()
	defer d.filesMu.RUnlock()
	id := fmt.Sprintf("%x", ethereum.HashRaw(o))
	d.files[id] = string(o)
	return d.prefix + id, nil
}

func (d *DataMockTest) Retrieve(ctx context.Context, id string, maxSize int64) ([]byte, error) {
	d.filesMu.RLock()
	defer d.filesMu.RUnlock()
	if data, ok := d.files[id]; ok {
		return []byte(data), nil
	}
	if d.rnd.RandomIntn(2) == 0 {
		return nil, os.ErrDeadlineExceeded
	}
	time.Sleep(200 * time.Millisecond)
	return d.rnd.RandomBytes(256), nil
}

func (d *DataMockTest) Pin(ctx context.Context, path string) error {
	d.filesMu.Lock()
	defer d.filesMu.Unlock()
	if _, ok := d.files[path]; ok {
		return nil
	}
	time.Sleep(200 * time.Millisecond)
	d.files[path] = string(d.rnd.RandomBytes(256))
	return nil
}

func (d *DataMockTest) Unpin(ctx context.Context, path string) error {
	d.filesMu.Lock()
	defer d.filesMu.Unlock()
	if _, ok := d.files[path]; !ok {
		return os.ErrNotExist
	}
	delete(d.files, path)
	return nil
}

func (d *DataMockTest) ListPins(ctx context.Context) (map[string]string, error) {
	d.filesMu.RLock()
	defer d.filesMu.RUnlock()
	filesCopy := make(map[string]string, len(d.files))
	for k, v := range d.files {
		filesCopy[k] = v
	}
	return filesCopy, nil
}

func (d *DataMockTest) URIprefix() string {
	return d.prefix
}

func (d *DataMockTest) Stats(ctx context.Context) (string, error) {
	return "", nil
}

func (d *DataMockTest) CollectMetrics(ctx context.Context, ma *metrics.Agent) error {
	return nil
}

func (d *DataMockTest) Stop() error {
	return nil
}
