package data

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
)

// DataMockTest is a mock data provider for testing purposes.
type DataMockTest struct {
	files  map[string]string
	prefix string
	rnd    testutil.Random
}

func (d *DataMockTest) Init(ds *types.DataStore) error {
	d.files = make(map[string]string)
	d.prefix = "mock://"
	d.rnd = testutil.NewRandom(0)
	return nil
}

func (d *DataMockTest) Publish(ctx context.Context, o []byte) (string, error) {
	id := fmt.Sprintf("%x", ethereum.HashRaw(o))
	d.files[id] = string(o)
	return d.prefix + id, nil
}

func (d *DataMockTest) Retrieve(ctx context.Context, id string, maxSize int64) ([]byte, error) {
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
	if _, ok := d.files[path]; ok {
		return nil
	}
	time.Sleep(200 * time.Millisecond)
	d.files[path] = string(d.rnd.RandomBytes(256))
	return nil
}

func (d *DataMockTest) Unpin(ctx context.Context, path string) error {
	if _, ok := d.files[path]; !ok {
		return os.ErrNotExist
	}
	delete(d.files, path)
	return nil
}

func (d *DataMockTest) ListPins(ctx context.Context) (map[string]string, error) {
	return d.files, nil
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
