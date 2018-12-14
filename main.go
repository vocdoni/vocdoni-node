package main

import (
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"time"
	"github.com/vocdoni/dvote-relay/batch"
)

var dbPath = "$HOME/.dvote/relay.db"
var bdbPath = "$HOME/.dvote/batch.db"
var batchSeconds = 3 //seconds

var err error
var db *leveldb.DB
var bdb *leveldb.DB
var batchTimer *time.Ticker
var batchSignal chan int


func setup() {
	db, err = leveldb.OpenFile(dbPath, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	bdb, err = leveldb.OpenFile(bdbPath, nil)
	if err != nil {
		panic(err)
	}
	defer bdb.Close()

	batchTimer = time.NewTicker(time.Second * time.Duration(batchSeconds))
	//create batch signal channel
}

func main() {
	setup()
	fmt.Println("Entering main loop")
	for {
		select {
		case <- batchTimer.C:
			fmt.Println("Timer triggered")
			fmt.Println(batch.Create(bdb))
			//replace with chain link
		case <- batchSignal:
			fmt.Println("Signal triggered")
			fmt.Println(batch.Create(bdb))
//		case <- stopSignal:
//			return
		}
	}
}
