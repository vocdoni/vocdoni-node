package indexer

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/klauspost/compress/zstd"
	"go.vocdoni.io/dvote/vochain"
)

func TestRestoreBackupAndMigrate(t *testing.T) {
	app := vochain.TestBaseApplication(t)
	idx, err := New(app, Options{DataDir: t.TempDir(), ExpectBackupRestore: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := idx.Close(); err != nil {
			t.Error(err)
		}
	})

	backupPath := filepath.Join(t.TempDir(), "backup.sql")
	backupFile, err := os.Create(backupPath)
	qt.Assert(t, err, qt.IsNil)
	t.Cleanup(func() { backupFile.Close() })

	backupZstdPath := filepath.Join("testdata", "sqlite-backup-0009.sql.zst")
	backupZstdFile, err := os.Open(backupZstdPath)
	qt.Assert(t, err, qt.IsNil)
	t.Cleanup(func() { backupZstdFile.Close() })

	// The testdata backup file is compressed with zstd -15.
	decoder, err := zstd.NewReader(backupZstdFile)
	qt.Assert(t, err, qt.IsNil)
	_, err = io.Copy(backupFile, decoder)
	qt.Assert(t, err, qt.IsNil)
	err = backupFile.Close()
	qt.Assert(t, err, qt.IsNil)

	// Restore the backup.
	// Note that the indexer prepares all queries upfront,
	// which means sqlite will fail if any of them reference missing columns or tables.
	err = idx.RestoreBackup(backupPath)
	qt.Assert(t, err, qt.IsNil)

	// Sanity check that the data is there, and can be used.
	// TODO: do "get all columns" queries on important tables like processes and votes,
	// to sanity check that the data types match up as well.
	totalProcs := idx.CountTotalProcesses()
	qt.Assert(t, totalProcs, qt.Equals, uint64(445))
	totalVotes, _ := idx.CountTotalVotes()
	qt.Assert(t, totalVotes, qt.Equals, uint64(5159))
}
