package censustree

import "go.vocdoni.io/proto/build/go/models"

type Tree interface {
	// Type returns the numeric identifier for the censusTree implementation
	Type() models.Census_Type
	// TypeString returns a human readable name for the censusTree
	// implementation
	TypeString() string
	// Init initializes the Tree using the given storage directory.
	Init(name, storage string) error
	// MaxKeySize returns the maximum key size supported by the Tree
	MaxKeySize() (size int)
	// LastAccess returns the last time the Tree was accessed, in the form
	// of a unix timestamp.
	LastAccess() (timestamp int64)
	// Publish makes a merkle tree available for queries.  Application layer
	// should check IsPublish() before considering the Tree available. A
	// census tree must not be available for query until Publish() is
	// called.
	Publish()
	// UnPublish makes a merkle tree not available for queries
	UnPublish()
	// IsPublic returns true if the tree is available (published)
	IsPublic() bool
	// Add adds a new leaf into the merkle tree
	Add(key, value []byte) error
	// AddBatch adds a batch of indexes and values to the tree.  Must
	// support values=nil.
	AddBatch(keys, values [][]byte) (failedIndexes []int, err error)
	// GenProof generates a merkle tree proof that can be later used on
	// CheckProof() to validate it
	GenProof(key []byte) (mproof []byte, err error)
	// CheckProof validates a merkle proof and its data for the Tree hash function
	// GetValue returns the value for a key
	GetValue(key []byte) []byte
	CheckProof(key, value, root, mproof []byte) (included bool, err error)
	// Root returns the current merkle tree root
	Root() []byte
	// Dump exports all the Tree leafs in a byte array, which can later be
	// imported using the ImportDump method
	Dump(root []byte) (data []byte, err error)
	// DumpPlain returns all the Tree leafs in two arrays, one for the keys,
	// and another for the values.
	DumpPlain(root []byte) (keys [][]byte, values [][]byte, err error)
	// ImportDump imports the leafs (that have been exported with the Dump
	// method) in the Tree.
	ImportDump(data []byte) error
	// Size returns the number of leafs of the Tree
	Size(root []byte) (int64, error)
	// Snapshot returns a censustree.Tree instance of the Tree
	Snapshot(root []byte) (Tree, error)
	// HashExists checks if a hash exists as a key of a node in the merkle
	// tree
	HashExists(hash []byte) (bool, error)
}
