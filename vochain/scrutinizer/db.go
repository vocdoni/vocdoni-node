package scrutinizer

import (
	"fmt"
	"strings"
	"time"

	"github.com/timshannon/badgerhold/v3"
)

// The string to search on the KV database error to identify a transaction conflict.
// If the KV (currently badger) returns this error, it is considered non fatal and the
// transaction will be retried until it works.
// This check is made comparing string in order to avoid importing a specific KV
// implementation.
const kvErrorStringForRetry = "Transaction Conflict"

// InitDB initializes a badgerhold db at the location given by dataDir
func InitDB(dataDir string) (*badgerhold.Store, error) {
	opts := badgerhold.DefaultOptions

	// Note that these "With" options return a modified copy.
	opts.Options = opts.WithCompression(0)
	opts.Options = opts.WithBlockCacheSize(0)
	opts.Options = opts.WithVerifyValueChecksum(false)
	opts.Options = opts.WithDetectConflicts(true)

	opts.SequenceBandwith = 10000
	opts.Dir = dataDir
	opts.ValueDir = dataDir

	// TODO: support configurable logging
	opts.Options = opts.WithLogger(nil)
	opts.Logger = nil

	return badgerhold.Open(opts)
}

func (s *Scrutinizer) queryWithRetries(query func() error) error {
	maxTries := 1000
	for {
		if err := query(); err != nil {
			if strings.Contains(err.Error(), kvErrorStringForRetry) {
				maxTries--
				if maxTries == 0 {
					return fmt.Errorf("cannot update record: max retires reached")
				}
				time.Sleep(time.Millisecond * 5)
				continue
			}
			return fmt.Errorf("cannot update record: %w, ", err)
		}
		return nil
	}
}
