package main

import (
	"fmt"
	"os"
	"time"

	"flag"

	"github.com/ethereum/go-ethereum/log"
	swarm "github.com/vocdoni/go-dvote/swarm"
)

func main() {
	kind := flag.String("encryption", "raw", "pss encryption key schema")
	key := flag.String("key", "vocdoni", "pss encryption key")
	topic := flag.String("topic", "vocdoni_test", "pss topic")
	addr := flag.String("address", "", "pss address")
	logLevel := flag.String("log", "crit", "pss node log level")
	flag.Parse()

	sn := new(swarm.SimpleSwarm)
	err := sn.InitPSS()
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	err = sn.SetLog(*logLevel)
	if err != nil {
		fmt.Printf("Cannot set loglevel %v\n", err)
	}

	sn.PssSub(*kind, *key, *topic, "")
	defer sn.PssTopics[*topic].Unregister()

	fmt.Printf("My PSS pubKey is %s\n", sn.PssPubKey)

	go func() {
		for {
			pmsg := <-sn.PssTopics[*topic].Delivery
			fmt.Printf("\n%s\n", pmsg.Msg)
		}
	}()

	hostname, _ := os.Hostname()
	var msg string
	for {
		msg = "Hello world"
		currentTime := int64(time.Now().Unix())
		msg = fmt.Sprintf("[%d][%s] %s\n", currentTime, hostname, msg)
		err := sn.PssPub(*kind, *key, *topic, msg, *addr)
		log.Info("pss sent", "err", err)
		time.Sleep(10 * time.Second)
	}
}
