package data

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	ipfscore "github.com/ipfs/kubo/core"
	ipfsapi "github.com/ipfs/kubo/core/coreapi"
)

// MockStorage returns a IPFSHandle with a (offline, nilrepo) IPFS node
// with a functional CoreAPI
func MockStorage(t *testing.T) IPFSHandle {
	storage := IPFSHandle{}
	n, err := ipfscore.NewNode(context.Background(), &ipfscore.BuildCfg{
		Online:    false,
		Permanent: false,
		NilRepo:   false,
	})
	qt.Assert(t, err, qt.IsNil)
	api, err := ipfsapi.NewCoreAPI(n)
	qt.Assert(t, err, qt.IsNil)
	storage.CoreAPI = api
	return storage
}
