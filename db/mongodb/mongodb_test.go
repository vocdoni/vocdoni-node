package mongodb

import (
	"fmt"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/internal/dbtest"
	"go.vocdoni.io/dvote/db/prefixeddb"
	"go.vocdoni.io/dvote/util"
)

func TestWriteTx(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		t.Skip("the mongodb driver isn't complete")
	}
	database, err := New(db.Options{Path: util.RandomHex(16)})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestWriteTx(t, database)
}

func TestIterate(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		t.Skip("the mongodb driver isn't complete")
	}
	database, err := New(db.Options{Path: util.RandomHex(16)})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestIterate(t, database)
}

func TestWriteTxApply(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		t.Skip("the mongodb driver isn't complete")
	}
	database, err := New(db.Options{Path: util.RandomHex(16)})
	qt.Assert(t, err, qt.IsNil)

	dbtest.TestWriteTxApply(t, database)
}

func TestWriteTxApplyPrefixed(t *testing.T) {
	if os.Getenv("MONGODB_URL") == "" {
		t.Skip("the mongodb driver isn't complete")
	}
	database, err := New(db.Options{Path: util.RandomHex(16)})
	qt.Assert(t, err, qt.IsNil)

	prefix := []byte("one")
	dbWithPrefix := prefixeddb.NewPrefixedDatabase(database, prefix)

	dbtest.TestWriteTxApplyPrefixed(t, database, dbWithPrefix)
}

func BenchmarkWriteTx(b *testing.B) {
	database, err := New(db.Options{Path: b.TempDir()})
	if err != nil {
		b.Fatal(err)
	}
	defer database.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx := database.WriteTx()
		tx.Set([]byte("key"), []byte("value"))
		tx.Commit()
	}
}

func BenchmarkIterate(b *testing.B) {
	database, err := New(db.Options{Path: util.RandomHex(16)})
	if err != nil {
		b.Fatal(err)
	}
	defer database.Close()

	tx := database.WriteTx()
	for i := 0; i < 100000; i++ {
		tx.Set([]byte(fmt.Sprintf("key%d", i)), []byte("value"))
	}
	tx.Commit()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		database.Iterate([]byte("key"), func(k, v []byte) bool {
			return true
		})
	}
}

func BenchmarkWriteTxApply(b *testing.B) {
	database, err := New(db.Options{Path: util.RandomHex(16)})
	if err != nil {
		b.Fatal(err)
	}
	defer database.Close()

	tx1 := database.WriteTx()
	tx1.Set([]byte("key1"), []byte("value1"))

	tx2 := database.WriteTx()
	tx2.Set([]byte("key2"), []byte("value2"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx1.Apply(tx2)
		tx1.Commit()
	}
}
