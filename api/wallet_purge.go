package api

import (
	"bytes"
	"fmt"

	"go.vocdoni.io/dvote/db"
)

// Prefixes of the API database keys that hold custodial wallet data: per
// token, an encrypted private key, and per private key, its hash. The node
// does not store private keys; purgeWalletKeys deletes any left under these
// prefixes.
const (
	walletDBprefixEncryptedPrivateKey = "wpk_"
	walletDBprefixPrivateKeyHash      = "wph_"
)

// purgeWalletKeys deletes every key under the custodial wallet prefixes of
// database in a single transaction and returns how many it deleted. Running it
// again deletes nothing.
func purgeWalletKeys(database db.Database) (int, error) {
	var keys [][]byte
	for _, prefix := range []string{walletDBprefixEncryptedPrivateKey, walletDBprefixPrivateKeyHash} {
		if err := database.Iterate([]byte(prefix), func(key, _ []byte) bool {
			// The key is only valid until the callback returns, and backends
			// disagree on whether it includes the prefix (pebble strips it,
			// goleveldb and mongodb do not): copy it and rebuild the full key
			// either way.
			if bytes.HasPrefix(key, []byte(prefix)) {
				keys = append(keys, bytes.Clone(key))
				return true
			}
			keys = append(keys, append([]byte(prefix), key...))
			return true
		}); err != nil {
			return 0, err
		}
	}
	if len(keys) == 0 {
		return 0, nil
	}
	wtx := database.WriteTx()
	defer wtx.Discard()
	for _, key := range keys {
		if err := wtx.Delete(key); err != nil {
			return 0, err
		}
	}
	if err := wtx.Commit(); err != nil {
		return 0, err
	}
	// A delete of a missing key is a silent no-op, so verify the prefixes are
	// really empty rather than report a purge that did not happen.
	for _, prefix := range []string{walletDBprefixEncryptedPrivateKey, walletDBprefixPrivateKeyHash} {
		left := 0
		if err := database.Iterate([]byte(prefix), func(_, _ []byte) bool {
			left++
			return false
		}); err != nil {
			return 0, err
		}
		if left > 0 {
			return 0, fmt.Errorf("wallet keys remain under %q after purge", prefix)
		}
	}
	return len(keys), nil
}
