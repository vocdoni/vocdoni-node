package subpub

import (
	"fmt"
	"testing"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
)

func TestSubPub(t *testing.T) {
	log.InitLogger("error", "stdout")
	var s1 signature.SignKeys
	var s2 signature.SignKeys
	if err := s1.Generate(); err != nil {
		t.Fatal(err)
	}
	if err := s2.Generate(); err != nil {
		t.Fatal(err)
	}

	sp1 := NewSubPub(s1.Private, "vocdoni", 41000, false)
	sp2 := NewSubPub(s2.Private, "vocdoni", 41001, false)
	sp1.NoBootStrap = true
	sp1.Connect()
	go sp1.Subcribe()
	time.Sleep(time.Second * 1)

	bn := fmt.Sprintf("/ip4/127.0.0.1/tcp/41000/p2p/%s", sp1.Host.ID())
	sp2.BootNodes = []string{bn}
	sp2.Connect()
	go sp2.Subcribe()
	t.Log("sending message...")
	sp2.BroadcastWriter <- []byte("hello world")

	select {
	case msg := <-sp1.Reader:
		t.Logf("received: %s", msg)
		if fmt.Sprintf("%s", msg) != "hello world" {
			t.Fatalf("wrong message received %s", msg)
		}
	case <-time.After(time.Second * 5):
		t.Fatal("message not received")
	}
	sp1.Close()
	sp2.Close()
}
