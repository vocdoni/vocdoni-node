package main

import (
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/ethereum/go-ethereum/log"
	swarm "github.com/vocdoni/go-dvote/swarm"
)

func main() {
	kind := flag.String("encryption", "raw", "pss encryption key schema")
	key := flag.String("key", "vocdoni", "pss encryption key")
	topic := flag.String("topic", "vocdoni_test", "pss topic")
	_ = flag.String("address", "", "pss address")
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
			fmt.Printf("<- Pss received msg:{%s}\n", pmsg.Msg)
		}
	}()

	hostname, _ := os.Hostname()

	for {
		fmt.Printf("-> Sending %s pss to [%s]\n", *kind, *key)
		currentTime := int64(time.Now().Unix())
		err := sn.PssPub(*kind, *key, *topic,
			fmt.Sprintf("Hello I am %s, time is %d, my pubKey is %s and my PSS addr is %x", hostname, currentTime, sn.PssPubKey, sn.PssAddr), "b117308447d0b07f42565c45b2929ef0574519cedcb607191ead40c2e1")
		log.Info("pss sent", "err", err)
		time.Sleep(10 * time.Second)
	}
}
