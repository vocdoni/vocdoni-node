package snapshot

import (
	"go.vocdoni.io/dvote/vochain/state"
)

// DumpNoStateDB dumps the NoStateDB to the snapshot. `Create` needs to be called first.
func (s *Snapshot) DumpNoStateDB(v *state.State) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.header.Blobs = append(s.header.Blobs, SnapshotBlobHeader{
		Type: snapshotBlobType_NoStateDB,
		Size: 0,
	})

	return v.ExportNoStateDB(s)
}
