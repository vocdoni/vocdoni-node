package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	flag "github.com/spf13/pflag"
	"github.com/tcnksm/go-input"
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
	ui := &input.UI{
		Writer: os.Stdout,
		Reader: os.Stdin,
	}
	var msg string
	for {
		msg, err = ui.Ask("", &input.Options{
			Default:  "",
			Required: true,
			Loop:     false,
		})
		if len(msg) < 1 {
			continue
		}
		if msg == "exit" {
			break
		}
		currentTime := int64(time.Now().Unix())
		msg = fmt.Sprintf("[%d][%s] %s\n", currentTime, hostname, msg)
		//fmt.Printf("-> Sending %s pss to [%s]\n", *kind, *key)
		//err := sn.PssPub(*kind, *key, *topic,
		//	fmt.Sprintf("Hello I am %s, time is %d, my pubKey is %s and my PSS addr is %x", hostname, currentTime, sn.PssPubKey, sn.PssAddr), "b117308447d0b07f42565c45b2929ef0574519cedcb607191ead40c2e1")
		err := sn.PssPub(*kind, *key, *topic, msg, *addr)
		log.Info("pss sent", "err", err)
		//time.Sleep(10 * time.Second)
	}
}
