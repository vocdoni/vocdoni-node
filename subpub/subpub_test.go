package subpub

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	libpeer "github.com/libp2p/go-libp2p-core/peer"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func TestSubPub(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	key := []byte("vocdoni")
	const numPeers = 5
	var bootNodes []*SubPub
	var wg sync.WaitGroup
	for i := 0; i < numPeers; i++ {
		// TODO(mvdan): use a random unused port instead. note that ports
		// 1-1024 are only available to root.
		port1 := 1025 + rand.Intn(50000)

		var sign0 signature.SignKeys
		if err := sign0.Generate(); err != nil {
			t.Fatal(err)
		}
		sp0 := NewSubPub(sign0.Private, key, port1, false)
		sp0.NoBootStrap = true
		bootNodes = append(bootNodes, sp0)
		wg.Add(1)
		go func() {
			defer wg.Done()
			sp0.Start(ctx)
			go sp0.Subscribe(ctx)
		}()
		defer sp0.Close()
	}

	var sign signature.SignKeys
	if err := sign.Generate(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	// TODO(mvdan): same port fix as above
	sp := NewSubPub(sign.Private, key, 1025+rand.Intn(50000), false)
	for _, sp0 := range bootNodes {
		sp.BootNodes = append(sp.BootNodes, fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", sp0.Port, sp0.Host.ID()))
	}

	// We might discover peers before the first node has advertised. The
	// default retry period is 10s, far too long for the test. Lower it to
	// 50ms, so that we can find the other node quickly.
	sp.DiscoveryPeriod = 50 * time.Millisecond
	sp.CollectionPeriod = 50 * time.Millisecond

	peerAdded := make(chan libpeer.ID, numPeers)
	sp.onPeerAdd = func(id libpeer.ID) { peerAdded <- id }
	peerRemoved := make(chan libpeer.ID, numPeers)
	sp.onPeerRemove = func(id libpeer.ID) { peerRemoved <- id }

	sp.Start(ctx)
	go sp.Subscribe(ctx)
	defer sp.Close()

	t.Log("waiting for all peers to be connected")
	for i := 0; i < numPeers; i++ {
		select {
		case <-peerAdded:
		case <-ctx.Done():
			t.Fatal("timed out waiting for peers")
		}
	}

	t.Log("sending a broadcast message")
	select {
	case sp.BroadcastWriter <- []byte("hello world"):
	case <-ctx.Done():
		t.Fatal("message not sent")
	}

	t.Log("waiting for all peers to receive the broadcast")
	for i, sp0 := range bootNodes {
		wg.Add(1)
		i, sp0 := i, sp0 // copy the variables for the goroutine
		go func() {
			select {
			case msg := <-sp0.Reader:
				t.Logf("received on node %d: %s", i, msg)
				if fmt.Sprintf("%s", msg) != "hello world" {
					t.Errorf("wrong message received on node %d: %s", i, msg)
				}
			case <-ctx.Done():
				t.Errorf("message not received on node %d", i)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	const numPeersShutdown = 2
	t.Logf("shutting down %d random peers", numPeersShutdown)
	for _, i := range rand.Perm(5)[:numPeersShutdown] {
		sp0 := bootNodes[i]
		if err := sp0.Close(); err != nil {
			t.Fatalf("could not close node %d", i)
		}
	}

	// Removing peers used to panic, due to bad reslicing.
	t.Log("waiting for the two shut down peers to be dropped")
	for i := 0; i < numPeersShutdown; i++ {
		select {
		case <-peerRemoved:
		case <-ctx.Done():
			t.Fatal("timed out waiting for peers to disappear")
		}
	}
}
