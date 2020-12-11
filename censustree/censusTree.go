package censustree

type Tree interface {
	Init(name, storage string) error
	MaxClaimSize() int
	LastAccess() int64
	Publish()
	UnPublish()
	IsPublic() bool
	AddClaim(index, value []byte) error
	GenProof(index, value []byte) (string, error)
	CheckProof(index, value, root []byte, mpHex string) (bool, error)
	Root() []byte
	Dump(root []byte) (claims []string, err error)
	DumpPlain(root []byte, responseBase64 bool) ([]string, []string, error)
	ImportDump(claims []string) error
	Size(root []byte) (int64, error)
	Snapshot(root []byte) (Tree, error)
	HashExist(hash []byte) (bool, error)
}
