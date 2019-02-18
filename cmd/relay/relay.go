package main

import (
	"fmt"
	"time"
	"encoding/gob"
	"bytes"
	"os"
	"flag"
	"github.com/vocdoni/go-dvote/batch"
	"github.com/vocdoni/go-dvote/net"
	"github.com/vocdoni/go-dvote/db"
	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/chain"
)

var dbPath = "~/.dvote/relay.db"
var batchSeconds = 10 //seconds
var batchSize = 3 //packets

var err error
var batchTimer *time.Ticker
var batchSignal chan bool
var signal bool
var transportType net.TransportID


func main() {

	db, err := db.NewLevelDbStorage(dbPath, false)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	batch.Setup(db)

	//gather transport type flag
	var transportIDString string
	flag.StringVar(&transportIDString, "transport", "PubSub", "Transport must be one of: PubSub, HTTP")
	flag.Parse()
	transportType = net.TransportIDFromString(transportIDString)


	batchTimer = time.NewTicker(time.Second * time.Duration(batchSeconds))
	batchSignal = make(chan bool)

	batch.BatchSignal = batchSignal
	batch.BatchSize = batchSize

	node, err := chain.Init()
	node.Start()

	transport, err := net.Init(transportType)
	if err != nil {
		os.Exit(1)
	}
	fmt.Println("Entering main loop")
	go transport.Listen()
	for {
		select {
		case <- batchTimer.C:
			//fmt.Println("Timer triggered")
//			//fmt.Println(batch.Create())
			//replace with chain link
		case signal := <-batchSignal:
			if signal == true {
				fmt.Println("Signal triggered")
				ns, bs := batch.Fetch()
				buf := &bytes.Buffer{}
				gob.NewEncoder(buf).Encode(bs)
				bb := buf.Bytes()
				cid := data.Publish(bb)
				data.Pin(cid)
				fmt.Printf("Batch published at: %s \n", cid)
				node.LinkBatch([]byte(cid))
				// add to chain
				// announce to pubsub
				//fmt.Println("Nullifiers:")
				//fmt.Println(n)
				//fmt.Println("Batch:")
				//fmt.Println(b)
				batch.Compact(ns)
			}
		default:
			continue
		}
	}
}
