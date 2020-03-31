package subpub

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func TestSubPub(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var bootNodes []*SubPub
	for i := 0; i < 5; i++ {
		// TODO(mvdan): use a random unused port instead. note that ports
		// 1-1024 are only available to root.
		port1 := 1025 + rand.Intn(50000)

		var sign0 signature.SignKeys
		if err := sign0.Generate(); err != nil {
			t.Fatal(err)
		}
		sp0 := NewSubPub(sign0.Private, "vocdoni", port1, false)
		sp0.NoBootStrap = true
		sp0.Connect(ctx)
		go sp0.Subscribe(ctx)
		defer sp0.Close()
		bootNodes = append(bootNodes, sp0)
	}

	var sign signature.SignKeys
	if err := sign.Generate(); err != nil {
		t.Fatal(err)
	}
	// TODO(mvdan): same port fix as above
	sp := NewSubPub(sign.Private, "vocdoni", 1025+rand.Intn(50000), false)
	for _, sp0 := range bootNodes {
		sp.BootNodes = append(sp.BootNodes, fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", sp0.Port, sp0.Host.ID()))
	}

	// We might discover peers before the first node has advertised. The
	// default retry period is 10s, far too long for the test. Lower it to
	// 100ms, so that we can find the other node quickly.
	sp.DiscoveryPeriod = 100 * time.Millisecond

	sp.CollectionPeriod = 100 * time.Millisecond

	sp.Connect(ctx)
	go sp.Subscribe(ctx)
	defer sp.Close()

	// TODO(mvdan): reimplement without polling/sleeping.
	t.Log("waiting for all peers to be connected")
	for {
		sp.PeersMu.Lock()
		n := len(sp.Peers)
		sp.PeersMu.Unlock()
		if n == 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
		if err := ctx.Err(); err != nil {
			t.Fatal(err)
		}
	}

	t.Log("sending a broadcast message")
	select {
	case sp.BroadcastWriter <- []byte("hello world"):
	case <-ctx.Done():
		t.Fatal("message not sent")
	}

	t.Log("waiting for all peers to receive the broadcast")
	var wg sync.WaitGroup
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

	t.Log("shutting down two random peers")
	for _, i := range rand.Perm(5)[:2] {
		sp0 := bootNodes[i]
		if err := sp0.Close(); err != nil {
			t.Fatalf("could not close node %d", i)
		}
	}

	// Removing peers used to panic, due to bad reslicing.
	// TODO(mvdan): reimplement without polling/sleeping.
	t.Log("waiting for the two shut down peers to be dropped")
	for {
		sp.PeersMu.Lock()
		n := len(sp.Peers)
		sp.PeersMu.Unlock()
		if n == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
		if err := ctx.Err(); err != nil {
			t.Fatal(err)
		}
	}
}
