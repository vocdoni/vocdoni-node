package api

import (
	"fmt"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/httprouter"
)

// TestNewAPIPurgesWalletKeys checks that NewAPI deletes every key under the
// custodial wallet prefixes of the API database, keeps the other keys, and
// that a second start deletes nothing.
func TestNewAPIPurgesWalletKeys(t *testing.T) {
	c := qt.New(t)
	dataDir := t.TempDir()

	walletKeys := [][]byte{}
	for i := range 3 {
		walletKeys = append(walletKeys,
			fmt.Appendf(nil, "%s%032x", walletDBprefixEncryptedPrivateKey, i),
			fmt.Appendf(nil, "%s%032x", walletDBprefixPrivateKeyHash, i))
	}
	otherKeys := [][]byte{[]byte("other"), []byte("wp_x"), []byte("wpkx")}

	// the database NewAPI opens, with the same prefix
	mdb, err := metadb.New(db.TypePebble, filepath.Join(dataDir, "db"))
	c.Assert(err, qt.IsNil)
	apiDB := prefixeddb.NewPrefixedDatabase(mdb, []byte("api/"))
	wtx := apiDB.WriteTx()
	for _, key := range append(append([][]byte{}, walletKeys...), otherKeys...) {
		c.Assert(wtx.Set(key, []byte("v")), qt.IsNil)
	}
	c.Assert(wtx.Commit(), qt.IsNil)
	// a key with a wallet prefix outside api/ belongs to someone else
	wtx = mdb.WriteTx()
	c.Assert(wtx.Set([]byte(walletDBprefixEncryptedPrivateKey+"outside"), []byte("v")), qt.IsNil)
	c.Assert(wtx.Commit(), qt.IsNil)
	c.Assert(mdb.Close(), qt.IsNil)

	newAPI := func() *API {
		router := httprouter.HTTProuter{}
		c.Assert(router.Init("127.0.0.1", 0), qt.IsNil)
		api, err := NewAPI(&router, "/", dataDir, db.TypePebble)
		c.Assert(err, qt.IsNil)
		return api
	}

	api := newAPI()
	for _, key := range walletKeys {
		_, err := api.db.Get(key)
		c.Assert(err, qt.ErrorIs, db.ErrKeyNotFound, qt.Commentf("key %s", key))
	}
	for _, key := range otherKeys {
		_, err := api.db.Get(key)
		c.Assert(err, qt.IsNil, qt.Commentf("key %s", key))
	}
	purged, err := purgeWalletKeys(api.db)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 0)
	c.Assert(api.db.Close(), qt.IsNil)

	// a second start finds nothing to purge and keeps the rest
	api = newAPI()
	for _, key := range otherKeys {
		_, err := api.db.Get(key)
		c.Assert(err, qt.IsNil, qt.Commentf("key %s", key))
	}
	c.Assert(api.db.Close(), qt.IsNil)

	mdb, err = metadb.New(db.TypePebble, filepath.Join(dataDir, "db"))
	c.Assert(err, qt.IsNil)
	_, err = mdb.Get([]byte(walletDBprefixEncryptedPrivateKey + "outside"))
	c.Assert(err, qt.IsNil)
	c.Assert(mdb.Close(), qt.IsNil)
}
