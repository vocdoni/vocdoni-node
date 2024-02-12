package snapshot

import (
	"io"
	"sync"
)

// fnIndexer groups the funcs that export and import the Indexer
var fnIndexer struct {
	sync.Mutex
	exp func(w io.Writer) error
	imp func(r io.Reader) error
}

// SetFnExportIndexer sets the func that exports the Indexer
func SetFnExportIndexer(fn func(w io.Writer) error) {
	fnIndexer.Lock()
	defer fnIndexer.Unlock()
	fnIndexer.exp = fn
}

// FnExportIndexer returns the func that exports the Indexer
func FnExportIndexer() (fn func(w io.Writer) error) {
	fnIndexer.Lock()
	defer fnIndexer.Unlock()
	return fnIndexer.exp
}

// SetFnImportIndexer sets the func that imports the Indexer
func SetFnImportIndexer(fn func(r io.Reader) error) {
	fnIndexer.Lock()
	defer fnIndexer.Unlock()
	fnIndexer.imp = fn
}

// FnImportIndexer returns the func that imports the Indexer
func FnImportIndexer() (fn func(r io.Reader) error) {
	fnIndexer.Lock()
	defer fnIndexer.Unlock()
	return fnIndexer.imp
}

// DumpIndexer calls the passed fn to dump the Indexer to the snapshot.
func (s *Snapshot) DumpIndexer(fn func(w io.Writer) error) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.header.Blobs = append(s.header.Blobs, SnapshotBlobHeader{
		Type: snapshotBlobType_IndexerDB,
		Size: 0,
	})

	return fn(s)
}
