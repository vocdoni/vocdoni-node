package subpub_test

import (
	"context"
	"testing"

	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/ipfsconnect/subpub"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

func Test_example(t *testing.T) {
	log.Init("info", "stdout")

	messages := make(chan *subpub.Message)
	groupKey := util.Random32()

	handler := ipfs.MockIPFS(t)
	sp := subpub.NewSubPub(groupKey, handler.Node)
	sp.Start(context.Background(), messages)
}
