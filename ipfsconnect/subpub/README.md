# SubPub

SubPub is a simplified PubSub protocol using libp2p

minimal usage example:
```go
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
	groupKey := []byte("test")
	port := 6543
	privKey := util.RandomHex(32)

	sp := subpub.NewSubPub(privKey, groupKey, int32(port), false)
	sp.Start(context.Background(), messages)
}
```

* Creates a libp2p Host, with a PubSub service attached that uses the GossipSub router.
* method SendBroadcast() uses this PubSub service for routing broadcast messages to peers.
* method SendUnicast() sends a unicast packet directly to a single peer.
* The topic used for p2p discovery is determined from the groupKey (hash)
* Incoming messages (both unicasts and broadcasts) are passed to the `messages` channel indicated in Start()
