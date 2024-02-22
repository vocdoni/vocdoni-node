package metadb

import (
	"cmp"
	"fmt"
	"os"
	"testing"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/goleveldb"
	"go.vocdoni.io/dvote/db/mongodb"
	"go.vocdoni.io/dvote/db/pebbledb"
)

func New(typ, dir string) (db.Database, error) {
	var database db.Database
	var err error
	opts := db.Options{Path: dir}
	switch typ {
	case db.TypePebble:
		database, err = pebbledb.New(opts)
		if err != nil {
			return nil, err
		}
	case db.TypeLevelDB:
		database, err = goleveldb.New(opts)
		if err != nil {
			return nil, err
		}
	case db.TypeMongo:
		database, err = mongodb.New(opts)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid dbType: %q. Available types: %q %q %q",
			typ, db.TypePebble, db.TypeLevelDB, db.TypeMongo)
	}
	return database, nil
}

func ForTest() (typ string) {
	return cmp.Or(os.Getenv("DVOTE_DB_TYPE"), "pebble") // default to Pebble, just like vocdoninode
}

func NewTest(tb testing.TB) db.Database {
	database, err := New(ForTest(), tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { database.Close() })
	return database
}
