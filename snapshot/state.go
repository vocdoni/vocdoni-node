package snapshot

import (
	"go.vocdoni.io/dvote/statedb"
)

// DumpTree dumps a tree to the snapshot.
func (s *Snapshot) DumpTree(name, parent string, key []byte, tr statedb.TreeViewer) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	root, err := tr.Root()
	if err != nil {
		return err
	}

	s.header.Blobs = append(s.header.Blobs, SnapshotBlobHeader{
		Type:   snapshotBlobType_Tree,
		Name:   name,
		Parent: parent,
		Key:    key,
		Root:   root,
		Size:   0,
	})

	return tr.Dump(s)
}
