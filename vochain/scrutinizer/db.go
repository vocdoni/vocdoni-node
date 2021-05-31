package scrutinizer

import (
	"github.com/timshannon/badgerhold/v3"
)

// InitDB initializes a badgerhold db at the location given by dataDir
func InitDB(dataDir string) (*badgerhold.Store, error) {
	opts := badgerhold.DefaultOptions
	opts.WithCompression(0)
	opts.WithBlockCacheSize(0)
	opts.SequenceBandwith = 10000
	opts.WithVerifyValueChecksum(false)
	opts.WithDetectConflicts(true)
	opts.Dir = dataDir
	opts.ValueDir = dataDir
	// TO-DO set custom logger
	return badgerhold.Open(opts)
}
