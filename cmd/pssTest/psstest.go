package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	swarm "github.com/vocdoni/go-dvote/net/swarm"
)

func main() {
	sn := new(swarm.SwarmNet)
	err := sn.Init()
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	key := os.Args[1]
	topic := "vocdoni_test"

	sn.PssSub("asym", key, topic, "")
	go func() {
		for {
			msg := <-sn.PssTopics[topic].Delivery
			fmt.Printf("Pss received: %s\n", msg)
		}
	}()

	hostname, _ := os.Hostname()
	for {
		err := sn.PssPub("asym", key, topic, fmt.Sprintf("Hello world from %s", hostname), "")
		log.Info("pss sent", "err", err)
		time.Sleep(10 * time.Second)
	}
}
