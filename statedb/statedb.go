package statedb

type StorageType string

const (
	StorageTypeDisk   StorageType = "disk"
	StorageTypeMemory StorageType = "mem"
)

type StateDB interface {
	Init(storagePath string, storageType StorageType) error
	Version() uint64
	LoadVersion(int64) error // zero means last version, -1 is the previous to the last version
	AddTree(name string) error
	Tree(name string) StateTree
	TreeWithRoot(root []byte) StateTree
	ImmutableTree(name string) StateTree // a tree version that won't change
	Commit() ([]byte, error)             // Returns New Hash
	Rollback() error
	KeyDiff(root1, root2 []byte) ([][]byte, error) // list of inserted keys on root2 that are not present in root1
	Hash() []byte
	Close() error
}

type StateTree interface {
	Get(key []byte) ([]byte, error)
	Add(key, value []byte) error
	Iterate(prefix []byte, callback func(key, value []byte) bool)
	Hash() []byte
	Count() uint64
	Version() uint64
	Proof(key []byte) ([]byte, error)
	Verify(key, value, proof, root []byte) bool
	Commit() error
}
