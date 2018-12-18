package batch

import (
	"fmt"
	"github.com/vocdoni/dvote-relay/types"
	"github.com/vocdoni/dvote-relay/db"
)

var rdb *db.LevelDbStorage
var bdb *db.LevelDbStorage
var BatchSignal chan bool
var BatchSize int
var currentBatchSize int
var err error

func Setup(l *db.LevelDbStorage) {
	rdb = l.WithPrefix([]byte("relay"))
	bdb = l.WithPrefix([]byte("batch"))
	fmt.Println("Batch storage:")
	fmt.Println(bdb)
}


//add (queue for counting)
func Add(p types.Packet) error {
	fmt.Println(bdb)
	err := bdb.Put([]byte(fmt.Sprintf("%v", p.Nullifier)),[]byte(fmt.Sprintf("%v", p)))
	if err != nil {
		return err
	}

	currentBatchSize++
	if currentBatchSize >= BatchSize {
		BatchSignal <- true
	}
	return nil
}

//create (return batch)
func Create() []byte {
	var b []byte

	iter := bdb.Iter()
	for iter.Next() {
		//k := iter.Key()
		//v := iter.Value()
		err := iter.Error()
		if err != nil {
			panic(err)
		}
		//put p in batch
		//rdb.Put(iter.Key(), iter.Val(), nil)
		//bdb.Delete(iter.Key(), nil)
	}
	iter.Release()
	return b
}
