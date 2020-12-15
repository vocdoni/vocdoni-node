package censustree

type Tree interface {
	Init(name, storage string) error
	MaxClaimSize() (size int)
	LastAccess() (timestamp int64)
	Publish()
	UnPublish()
	IsPublic() bool
	AddClaim(index, value []byte) error
	GenProof(index, value []byte) (mproof []byte, err error)
	CheckProof(index, value, root, mproof []byte) (included bool, err error)
	Root() []byte
	Dump(root []byte) (claims []string, err error)
	DumpPlain(root []byte, responseBase64 bool) ([]string, []string, error)
	ImportDump(claims []string) error
	Size(root []byte) (int64, error)
	Snapshot(root []byte) (Tree, error)
	HashExist(hash []byte) (bool, error)
}
