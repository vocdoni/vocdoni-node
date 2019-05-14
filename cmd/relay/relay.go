package main

import (
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/vocdoni/go-dvote/batch"
	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/db"
	"github.com/vocdoni/go-dvote/net"
	"github.com/vocdoni/go-dvote/types"
)

var dbPath = "~/.dvote/relay.db"
var batchSeconds = 10 //seconds
var batchSize = 3     //packets

var err error
var batchTimer *time.Ticker
var batchSignal chan bool
var signal bool
var transportType net.TransportID
var storageType data.StorageID

func main() {

	db, err := db.NewLevelDbStorage(dbPath, false)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	batch.Setup(db)

	//gather transport type flag
	var transportIDString string
	var storageIDString string
	flag.StringVar(&transportIDString, "transport", "PSS", "Transport must be one of: PSS, PubSub")
	flag.StringVar(&storageIDString, "storage", "IPFS", "Transport must be one of: BZZ, IPFS")
	flag.Parse()

	transportType = net.TransportIDFromString(transportIDString)
	storageType = data.StorageIDFromString(storageIDString)

	batchTimer = time.NewTicker(time.Second * time.Duration(batchSeconds))
	batchSignal = make(chan bool)

	batch.BatchSignal = batchSignal
	batch.BatchSize = batchSize

	listenerOutput := make(chan types.Message)
	listenerErrors := make(chan error)

	routerOutput := make(chan types.Message)

	fmt.Println("initializing transport:")
	transport, err := net.InitDefault(transportType)
	if err != nil {
		os.Exit(1)
	}

	fmt.Println("initializing storage:")
	storage, err := data.InitDefault(storageType)
	if err != nil {
		os.Exit(1)
	}
	_ = storage

	fmt.Println("initializing websockets:")
	ws, err := net.InitDefault(net.TransportIDFromString("Websocket"))
	if err != nil {
		os.Exit(1)
	}

	go batch.Recieve(routerOutput)
	go transport.Listen(listenerOutput, listenerErrors)
	go ws.Listen(listenerOutput, listenerErrors)
	go net.Route(listenerOutput, routerOutput, listenerErrors, storage, ws)

	fmt.Println("Entering main loop")
	for {
		select {
		case <-batchTimer.C:
			fmt.Println("Timer triggered")
			//			fmt.Println(batch.Create())
			//replace with chain link
		case signal := <-batchSignal:
			if signal == true {
				fmt.Println("Signal triggered")
				_, bs := batch.Fetch()
				buf := &bytes.Buffer{}
				gob.NewEncoder(buf).Encode(bs)
				bb := buf.Bytes()
				cid, err := storage.Publish(bb)
				if err != nil {
					fmt.Printf("Storage error: %s", err)
				}
				fmt.Printf("Batch published at: %s \n", cid)

				// add to chain
				// announce to pubsub
				//fmt.Println("Nullifiers:")
				//fmt.Println(n)
				//fmt.Println("Batch:")
				//fmt.Println(b)
				//batch.Compact(ns)
			}
		case listenError := <-listenerErrors:
			fmt.Println(listenError)
		default:
			continue
		}
	}
}
