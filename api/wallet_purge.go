package api

import (
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
			// Iterate passes the key without the prefix and only valid until
			// the callback returns: append copies it after the prefix.
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
	return len(keys), nil
}
