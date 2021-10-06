package metadb

import (
	"fmt"

	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/badgerdb"
	"go.vocdoni.io/dvote/db/pebbledb"
)

func New(typ, dir string) (db.Database, error) {
	var database db.Database
	var err error
	opts := db.Options{Path: dir}
	switch typ {
	case db.TypePebble:
		database, err = badgerdb.New(opts)
		if err != nil {
			return nil, err
		}
	case db.TypeBadger:
		database, err = pebbledb.New(opts)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid dbType: %q. Available types: %q, %q", typ, db.TypePebble, db.TypeBadger)
	}
	return database, nil
}
