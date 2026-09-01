package subpub

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/kubo/config"
	ipfscore "github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
)

// newTestSubPub spins up an online in-memory IPFS node with the very same
// routing configuration used in production (see data/ipfs.startNode) and
// starts a SubPub on top of it.
func newTestSubPub(t *testing.T, ctx context.Context, groupKey [32]byte) (*SubPub, chan *Message) {
	t.Helper()

	identity, err := config.CreateIdentity(io.Discard,
		[]options.KeyGenerateOption{options.Key.Type(options.Ed25519Key)})
	qt.Assert(t, err, qt.IsNil)
	cfg, err := config.InitWithIdentity(identity)
	qt.Assert(t, err, qt.IsNil)
	// Loopback on an ephemeral port: the default config binds 4001 on every
	// interface, so two nodes would collide and advertise routable addresses.
	cfg.Addresses.Swarm = []string{"/ip4/127.0.0.1/tcp/0"}
	cfg.Bootstrap = nil
	cfg.Discovery.MDNS.Enabled = false
	// kubo v0.43's sweeping DHT provider wants an on-disk levelds keystore,
	// which a mock repo has no plugin for. We wire peers by hand anyway.
	cfg.Provide.Enabled = config.False

	node, err := ipfscore.NewNode(ctx, &ipfscore.BuildCfg{
		Online:    true,
		Permanent: false,
		Repo: &repo.Mock{
			C: *cfg,
			D: dssync.MutexWrap(ds.NewMapDatastore()),
		},
		Routing: func(args libp2p.RoutingOptionArgs) (routing.Routing, error) {
			args.OptimisticProvide = true
			return libp2p.DHTOption(args)
		},
	})
	qt.Assert(t, err, qt.IsNil)
	t.Cleanup(func() { _ = node.Close() })

	// The DHT must be a live client, otherwise setupDiscovery would build a
	// RoutingDiscovery around a typed-nil and panic on the first advertise.
	qt.Assert(t, node.DHT, qt.IsNotNil)
	qt.Assert(t, node.HasActiveDHTClient(), qt.IsTrue)

	sp := NewSubPub(groupKey, node)
	// Nothing bootstraps the DHT here, so keep discovery rounds short and cheap.
	sp.DiscoveryPeriod = time.Second

	msgs := make(chan *Message, 8)
	sp.Start(ctx, msgs)
	t.Cleanup(sp.Close)
	return sp, msgs
}

// TestSubPubGossipAndUnicast is a smoke test of the two paths that matter:
// a gossipsub broadcast and a direct unicast stream, both encrypted with the
// group key. Peers are wired manually because there is no DHT to discover
// through in a test.
func TestSubPubGossipAndUnicast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	groupKey := [32]byte{}
	copy(groupKey[:], "vocdoni-subpub-test-group-key")

	sp1, msgs1 := newTestSubPub(t, ctx, groupKey)
	sp2, msgs2 := newTestSubPub(t, ctx, groupKey)

	// Connect sp2 -> sp1 at the transport layer.
	addrs := sp1.node.PeerHost.Addrs()
	qt.Assert(t, len(addrs) > 0, qt.IsTrue)
	var connected bool
	for _, a := range addrs {
		if err := sp2.AddPeer(fmt.Sprintf("%s/p2p/%s", a, sp1.NodeID)); err == nil {
			connected = true
			break
		}
	}
	qt.Assert(t, connected, qt.IsTrue)

	// gossipsub needs a moment to notice the new peer and graft the topic mesh.
	waitFor(t, ctx, func() bool {
		return len(sp1.gossip.topic.ListPeers()) > 0 && len(sp2.gossip.topic.ListPeers()) > 0
	})

	t.Run("broadcast", func(t *testing.T) {
		// gossipsub is best-effort: a publish issued before the mesh is
		// grafted is silently dropped, so keep publishing until it lands.
		deadline := time.After(30 * time.Second)
		for {
			qt.Assert(t, sp1.SendBroadcast([]byte("hello gossip")), qt.IsNil)
			select {
			case m := <-msgs2:
				qt.Assert(t, string(m.Data), qt.Equals, "hello gossip")
				qt.Assert(t, m.Peer, qt.Equals, sp1.NodeID)
				return
			case <-time.After(500 * time.Millisecond):
			case <-deadline:
				t.Fatal("timed out waiting for the broadcast")
			}
		}
	})

	t.Run("unicast", func(t *testing.T) {
		// discover() is what normally opens this stream; do it by hand.
		stream, err := sp2.node.PeerHost.NewStream(ctx, sp1.node.PeerHost.ID(), protocol.ID(sp2.Topic))
		qt.Assert(t, err, qt.IsNil)
		sp2.handleStream(stream)

		qt.Assert(t, sp2.SendUnicast(sp1.NodeID, []byte("hello unicast")), qt.IsNil)
		select {
		case m := <-msgs1:
			qt.Assert(t, string(m.Data), qt.Equals, "hello unicast")
			qt.Assert(t, m.Peer, qt.Equals, sp2.NodeID)
		case <-time.After(30 * time.Second):
			t.Fatal("timed out waiting for the unicast")
		}
	})
}

func waitFor(t *testing.T, ctx context.Context, cond func() bool) {
	t.Helper()
	for {
		if cond() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for condition")
		case <-time.After(100 * time.Millisecond):
		}
	}
}
