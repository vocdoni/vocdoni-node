package main

import (
	  swarm "github.com/vocdoni/go-dvote/net/swarm" 
	  "os"
	  "fmt"
	  "time"
	  "github.com/ethereum/go-ethereum/log"
)
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Use <sym|asym> <key>")
		return
	}

	sn := new(swarm.SimplePss)
	err := sn.Init()
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	err = sn.SetLog("crit")
	if err != nil {
		fmt.Printf("Cannot set loglevel %v\n", err)
	}

	kind := os.Args[1]
	topic := "vocdoni_test"
	key := ""

	if kind == "sym" || kind == "asym" || kind == "raw" {
		if kind != "raw" {
			key = os.Args[2]
		}
		sn.PssSub(kind, key, topic, "")
		defer sn.PssTopics[topic].Unregister()
	} else {
		fmt.Println("First parameter must be sym or asym")
		return
	}
	fmt.Printf("My PSS pubKey is %s\n", sn.PssPubKey)
	go func() {
		for {
			pmsg := <-sn.PssTopics[topic].Delivery
			fmt.Printf("<- Pss received msg:{%s}\n", pmsg.Msg)
		}
	}()

	hostname, _ := os.Hostname()

	for {
		fmt.Printf("-> Sending %s pss to [%s]\n", kind, key)
		currentTime := int64(time.Now().Unix())
		err := sn.PssPub(kind, key, topic, fmt.Sprintf("Hello world from %s at %d", hostname, currentTime), "")
		log.Info("pss send", "err", err)
		time.Sleep(10 * time.Second)
	}
}
