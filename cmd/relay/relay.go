package main

import (
	"bytes"
	"encoding/gob"
	"flag"
	"log"
	"os/user"
	"time"

	"github.com/vocdoni/go-dvote/batch"
	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/db"
	"github.com/vocdoni/go-dvote/net"
	"github.com/vocdoni/go-dvote/types"
)

var batchSeconds = 6000 //seconds
var batchSize = 10      //votes

var err error
var batchTimer *time.Ticker
var batchSignal chan bool
var signal bool
var transportType net.TransportID
var storageType data.StorageID

func main() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	dataDir := usr.HomeDir + "/.dvote"
	dbPath := dataDir + "/relay.db"

	db, err := db.NewLevelDbStorage(dbPath, false)
	if err != nil {
		log.Fatal(err)
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

	log.Println("Initializing transport")
	transport, err := net.InitDefault(transportType)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Initializing storage")
	storage, err := data.InitDefault(storageType)
	if err != nil {
		log.Fatal(err)
	}
	_ = storage

	log.Println("Initializing websockets")
	ws, err := net.InitDefault(net.TransportIDFromString("Websocket"))
	if err != nil {
		log.Fatal(err)
	}

	go batch.Recieve(routerOutput)
	go transport.Listen(listenerOutput, listenerErrors)
	go ws.Listen(listenerOutput, listenerErrors)
	go net.Route(listenerOutput, routerOutput, listenerErrors, storage, ws)

	log.Println("Entering main loop")
	for {
		select {
		case <-batchTimer.C:
			//fmt.Println("Timer triggered")
			//			fmt.Println(batch.Create())
			//replace with chain link
			continue
		case signal := <-batchSignal:
			if signal == true {
				_, bs := batch.Fetch()
				buf := &bytes.Buffer{}
				gob.NewEncoder(buf).Encode(bs)
				bb := buf.Bytes()
				cid, err := storage.Publish(bb)
				if err != nil {
					log.Printf("Storage error: %s", err)
				}
				log.Printf("Batch published at: %s \n", cid)

				// add to chain
				// announce to pubsub
				//fmt.Println("Nullifiers:")
				//fmt.Println(n)
				//fmt.Println("Batch:")
				//fmt.Println(b)
				//batch.Compact(ns)
			}
		case listenError := <-listenerErrors:
			log.Println(listenError)
		default:
			continue
		}
	}
}
