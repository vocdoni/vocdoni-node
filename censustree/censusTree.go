package censustree

type Tree interface {
	Init(name, storage string) error
	MaxKeySize() (size int)
	LastAccess() (timestamp int64)
	Publish()       // A census tree must not be available for query until Publish() is called
	UnPublish()     // UnPublish will make the tree not available for queries
	IsPublic() bool // Check if the census tree is available for queries or not
	Add(key, value []byte) error
	GenProof(key, value []byte) (mproof []byte, err error)
	CheckProof(key, value, root, mproof []byte) (included bool, err error)
	Root() []byte
	Dump(root []byte) (data []byte, err error) // Dump must return a format compatible with ImportDump
	DumpPlain(root []byte) (keys [][]byte, values [][]byte, err error)
	ImportDump(data []byte) error
	Size(root []byte) (int64, error)
	Snapshot(root []byte) (Tree, error)
	HashExists(hash []byte) (bool, error)
}
