package subpub_test

import (
	"context"

	"go.vocdoni.io/dvote/ipfsconnect/subpub"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

func Example() {
	log.Init("info", "stdout")

	messages := make(chan *subpub.Message)
	port := 6543
	groupKey := util.Random32()

	sp := subpub.NewSubPub(groupKey, int32(port))
	sp.Start(context.Background(), messages)
}
