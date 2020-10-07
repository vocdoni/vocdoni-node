package statedb

type StateDB interface {
	Init(storagePath, sorageType string) error
	Version() uint64
	LoadVersion(int64) error // zero means last version, -1 is the previous to the last version
	AddTree(name string) error
	Tree(name string) StateTree
	TreeWithRoot(root []byte) StateTree
	ImmutableTree(name string) StateTree // a tree version that won't change
	Commit() ([]byte, error)             // Returns New Hash
	Rollback() error
	Hash() []byte
	Close() error
}

type StateTree interface {
	Get(key []byte) []byte
	Add(key, value []byte) error
	Iterate(prefix, until []byte, callback func(key, value []byte) bool)
	Hash() []byte
	Count() uint64
	Version() uint64
	Proof(key []byte) ([]byte, error)
	Verify(key, proof, root []byte) bool
}
