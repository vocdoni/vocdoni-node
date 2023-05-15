// Package ipfsconnect provides a service to maintain persistent connections (PersistPeers) between two or more IPFS nodes
package ipfsconnect

import (
	"context"
	"time"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/subpub"
)

// IPFSConnect is the service that discover and maintains persistent connections between IPFS nodes.
type IPFSConnect struct {
	IPFS      *ipfs.Handler
	Transport *subpub.SubPub
	GroupKey  [32]byte
}

// New creates a new IPFSConnect instance
func New(groupKey string, ipfsHandler *ipfs.Handler) *IPFSConnect {
	is := &IPFSConnect{
		IPFS: ipfsHandler,
	}
	keyHash := ethereum.HashRaw([]byte(groupKey))
	copy(is.GroupKey[:], keyHash[:])
	return is
}

// Start initializes and starts an IPFSConnect instance.
func (is *IPFSConnect) Start() {
	is.Transport = subpub.NewSubPub(is.GroupKey, is.IPFS.Node)
	is.Transport.OnlyDiscover = true
	// Do nothing with the messages, we just want to keep the connections alive.
	ch := make(chan *subpub.Message)
	go func() {
		for {
			select {
			case <-ch:
			default:
				time.Sleep(time.Second * 10)
			}
		}
	}()
	is.Transport.Start(context.TODO(), ch)
}
