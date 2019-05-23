# SimpleSwarm

A library to facilitate the usage of Swarm as a standalone tool.

## PSS

PSS is the swarm protocol for sending messages, similar to IPFs/PubSub and Ethereum/Whisper.
It shares the Whisper cryptography and adds a routing layer based on Kademlia DHT.

Using Swarm/PSS as a standalone protocol is hard, so this library aims to abstract the implementation and make it easy to integrate with any Go Lang project. 

Download the library.
```
go get -u github.com/vocdoni/go-dvote/swarm
```

Import it from your GoLang code.

```
import ( 
  swarm "github.com/vocdoni/go-dvote/swarm" 
)
```

#### Basic usage

Create a simpleSwarm instance
```
sn := new(swarm.SimpleSwarm)
```

Initialize and start the swarm process. This will start the p2p protocol and automatically connect to the swarm network.
```
err := sn.InitPSS()
```

Lets subscribe to a specific topic "aTopic" and add a symetric key which will be used to decrypt incoming messages (if encrypted).

```
// sn.PssSub(kind, key, topic, address)
sn.PssSub("sym", "aSymetricKey", "aTopic, "")
```

There are three kind of messages which can be used:

+ raw: not encrypted
+ sym: encrypted with a symetric key
+ asym: encrypted with asymetric key


Now we can address to struct `sn.PssTopics[topic]` for read received messages. The incoming messages will be sent through a go channel.

```
pmsg := <-sn.PssTopics[topic].Delivery
fmt.Printf("<- Pss received msg:{%s}\n", pmsg.Msg)
```
The channel read is blocking so it might be executed in a separate gorutine.

Sending a message is as easy as follows:

```
// sn.PssPub(kind, key, topic, message, address)
err := sn.PssPub("sym", "aSymetricKey", "atopic", "Hello world", "")
```
Address can be used for sending more direct messages (using Kademlia routing). As more address bits specified as more direct will be the message. If address is empty the message will reach all nodes (whisper mode).

#### Full example

```
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
		err := sn.PssPub(*kind, *key, *topic, msg, *addr)
		log.Info("pss sent", "err", err)
	}
}

```

