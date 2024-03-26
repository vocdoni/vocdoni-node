package snapshot

import (
	"io"
	"sync"
)

// fnCensusDB groups the funcs that export and import the CensusDB
var fnCensusDB struct {
	sync.Mutex
	exp func(w io.Writer) error
	imp func(r io.Reader) error
}

// SetFnExportCensusDB sets the func that exports the CensusDB
func SetFnExportCensusDB(fn func(w io.Writer) error) {
	fnCensusDB.Lock()
	defer fnCensusDB.Unlock()
	fnCensusDB.exp = fn
}

// FnExportCensusDB returns the func that exports the CensusDB
func FnExportCensusDB() (fn func(w io.Writer) error) {
	fnCensusDB.Lock()
	defer fnCensusDB.Unlock()
	return fnCensusDB.exp
}

// SetFnImportCensusDB sets the func that imports the CensusDB
func SetFnImportCensusDB(fn func(r io.Reader) error) {
	fnCensusDB.Lock()
	defer fnCensusDB.Unlock()
	fnCensusDB.imp = fn
}

// FnImportCensusDB returns the func that imports the CensusDB
func FnImportCensusDB() (fn func(r io.Reader) error) {
	fnCensusDB.Lock()
	defer fnCensusDB.Unlock()
	return fnCensusDB.imp
}

// DumpCensusDB calls the passed fn to dump the CensusDB to the snapshot.
func (s *Snapshot) DumpCensusDB(fn func(w io.Writer) error) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.header.Blobs = append(s.header.Blobs, SnapshotBlobHeader{
		Type: snapshotBlobType_CensusDB,
		Size: 0,
	})

	return fn(s)
}
