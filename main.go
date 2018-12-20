package main

import (
	"fmt"
	"time"
	"github.com/vocdoni/dvote-relay/batch"
	"github.com/vocdoni/dvote-relay/net"
	"github.com/vocdoni/dvote-relay/db"
)

var dbPath = "~/.dvote/relay.db"
var batchSeconds = 10 //seconds
var batchSize = 3 //packets

var err error
var batchTimer *time.Ticker
var batchSignal chan bool
var signal bool


func main() {

	db, err := db.NewLevelDbStorage(dbPath, false)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	batch.Setup(db)

	batchTimer = time.NewTicker(time.Second * time.Duration(batchSeconds))
	batchSignal = make(chan bool)

	batch.BatchSignal = batchSignal
	batch.BatchSize = batchSize

	fmt.Println("Entering main loop")
	go net.Listen("8080")
	for {
		select {
		case <- batchTimer.C:
			fmt.Println("Timer triggered")
//			fmt.Println(batch.Create())
			//replace with chain link
		case signal := <-batchSignal:
			if signal == true {
				fmt.Println("Signal triggered")
				n, b := batch.Create()
				fmt.Println("Nullifiers:")
				fmt.Println(n)
				fmt.Println("Batch:")
				fmt.Println(b)
				batch.Compact(n)
			}
		default:
			continue
		}
	}
}
