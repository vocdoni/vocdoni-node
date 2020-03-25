package subpub

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func TestSubPub(t *testing.T) {
	var s1 signature.SignKeys
	var s2 signature.SignKeys
	if err := s1.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := s2.Generate(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// TODO(mvdan): use a random unused port instead. note that ports
	// 1-1024 are only available to root.
	port1 := 1025 + rand.Intn(50000)
	port2 := 1025 + rand.Intn(50000)

	sp1 := NewSubPub(s1.Private, "vocdoni", port1, false)
	sp1.NoBootStrap = true
	sp1.Connect(ctx)
	go sp1.Subscribe(ctx)
	defer sp1.Close()

	sp2 := NewSubPub(s2.Private, "vocdoni", port2, false)
	bn := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port1, sp1.Host.ID())
	sp2.BootNodes = []string{bn}

	// We might discover peers before the first node has advertised. The
	// default retry period is 10s, far too long for the test. Lower it to
	// 100ms, so that we can find the other node quickly.
	sp2.DiscoveryPeriod = 100 * time.Millisecond

	sp2.Connect(ctx)
	go sp2.Subscribe(ctx)
	defer sp2.Close()

	t.Log("sending message...")
	select {
	case sp2.BroadcastWriter <- []byte("hello world"):
	case <-ctx.Done():
		t.Fatal("message not sent")
	}

	select {
	case msg := <-sp1.Reader:
		t.Logf("received: %s", msg)
		if fmt.Sprintf("%s", msg) != "hello world" {
			t.Fatalf("wrong message received %s", msg)
		}
	case <-ctx.Done():
		t.Fatal("message not received")
	}
}
