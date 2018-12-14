package main

import (
	"fmt"
	"time"
	"github.com/vocdoni/dvote-relay/batch"
	"github.com/vocdoni/dvote-relay/net"
)

var dbPath = "$HOME/.dvote/relay.db"
var bdbPath = "$HOME/.dvote/batch.db"
var batchSeconds = 30 //seconds
var batchSize = 3 //packets

var err error
var batchTimer *time.Ticker
var batchSignal chan bool
var signal bool


func setup() {
	batchTimer = time.NewTicker(time.Second * time.Duration(batchSeconds))
	batchSignal = make(chan bool)

	batch.DBPath = dbPath
	batch.BDBPath = bdbPath
	batch.BatchSignal = batchSignal
	batch.BatchSize = batchSize
}

func main() {
	setup()
//	batch.Setup()
	fmt.Println("Entering main loop")
	go net.Listen("8080")
	for {
		select {
		case <- batchTimer.C:
			fmt.Println("Timer triggered")
			fmt.Println(batch.Create())
			//replace with chain link
		case signal := <-batchSignal:
			if signal == true {
				fmt.Println("Signal triggered")
				fmt.Println(batch.Create())
			}
		default:
			continue
		}
	}
}
