package pebbledb

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db/internal/dbtest"
)

func TestWriteTx(t *testing.T) {
	database, err := New(Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestWriteTx(t, database)
}

func TestIterate(t *testing.T) {
	database, err := New(Options{Path: t.TempDir()})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestIterate(t, database)
}
