package data

import (
	"context"
	"maps"
	"os"
	"sync"
	"time"

	"go.vocdoni.io/dvote/data/ipfs"
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

func (d *DataMockTest) Init(_ *types.DataStore) error {
	d.files = make(map[string]string)
	d.prefix = "ipfs://"
	d.rnd = testutil.NewRandom(0)
	return nil
}

func (d *DataMockTest) Publish(_ context.Context, o []byte) (string, error) {
	d.filesMu.RLock()
	defer d.filesMu.RUnlock()
	cid := ipfs.CalculateCIDv1json(o)
	d.files[cid] = string(o)
	return d.prefix + cid, nil
}

func (d *DataMockTest) Retrieve(_ context.Context, id string, _ int64) ([]byte, error) {
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

func (d *DataMockTest) RetrieveDir(_ context.Context, id string, _ int64) (map[string][]byte, error) {
	// TODO: Implement
	return nil, nil
}

func (d *DataMockTest) Pin(_ context.Context, path string) error {
	d.filesMu.Lock()
	defer d.filesMu.Unlock()
	if _, ok := d.files[path]; ok {
		return nil
	}
	time.Sleep(200 * time.Millisecond)
	d.files[path] = string(d.rnd.RandomBytes(256))
	return nil
}

func (d *DataMockTest) Unpin(_ context.Context, path string) error {
	d.filesMu.Lock()
	defer d.filesMu.Unlock()
	if _, ok := d.files[path]; !ok {
		return os.ErrNotExist
	}
	delete(d.files, path)
	return nil
}

func (d *DataMockTest) ListPins(_ context.Context) (map[string]string, error) {
	d.filesMu.RLock()
	defer d.filesMu.RUnlock()
	return maps.Clone(d.files), nil
}

func (d *DataMockTest) URIprefix() string {
	return d.prefix
}

func (*DataMockTest) Stats() map[string]any {
	return nil
}

func (*DataMockTest) Stop() error {
	return nil
}
