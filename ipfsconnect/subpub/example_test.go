package subpub_test

import (
	"context"

	"go.vocdoni.io/dvote/ipfsconnect/subpub"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

func Example() {
	log.Init("info", "stdout", nil)

	messages := make(chan *subpub.Message)
	groupKey := []byte("test")
	port := 6543
	privKey := util.RandomHex(32)

	sp := subpub.NewSubPub(privKey, groupKey, int32(port), false)
	sp.Start(context.Background(), messages)
}
