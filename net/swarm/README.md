# SimpleSwarm

A library to facilitate the usage of Swarm as a standalone tool.

## SimplePss

PSS is the swarm protocol for sending messages, similar to IPFs/PubSub and Ethereum/Whisper.
It shares the Whisper cryptography and adds a routing layer based on Kademlia DHT.

Using Swarm/PSS as a devp2p standalone protocol is hard, so this library aims to abstract the implementation and make it easy to integrate with any GoLang project. 

Download the library.
```
go get -u github.com/vocdoni/go-dvote/net/swarm
```

Import it from your GoLang code.

```
import ( 
  swarm "github.com/vocdoni/go-dvote/net/swarm" 
)
```

#### Basic usage

Create a simplePSS instance
```
sn := new(swarm.SimplePss)
```

Initialize and stard the swarm process. This will start the p2p protocol and automatically connect to the swarm network.
```
err := sn.Init()
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
		log.Info("pss sent", "err", err)
		time.Sleep(10 * time.Second)
	}
}
```

