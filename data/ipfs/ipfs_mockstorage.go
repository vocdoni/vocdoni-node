package ipfs

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/hashicorp/golang-lru/v2/simplelru"
	ipfscore "github.com/ipfs/kubo/core"
	ipfsapi "github.com/ipfs/kubo/core/coreapi"
)

// MockIPFS returns a Handler with a (offline, nilrepo) IPFS node
// with a functional CoreAPI
func MockIPFS(t testing.TB) *Handler {
	storage := Handler{}
	n, err := ipfscore.NewNode(context.Background(), &ipfscore.BuildCfg{
		Online:    false,
		Permanent: false,
		NilRepo:   false,
	})
	qt.Assert(t, err, qt.IsNil)
	storage.retrieveCache, err = simplelru.NewLRU[string, []byte](RetrievedFileCacheSize, nil)
	qt.Assert(t, err, qt.IsNil)
	storage.Node = n
	api, err := ipfsapi.NewCoreAPI(n)
	qt.Assert(t, err, qt.IsNil)
	storage.CoreAPI = api
	return &storage
}
