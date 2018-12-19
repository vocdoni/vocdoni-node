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
	rdb = l.WithPrefix([]byte("relay_"))
	bdb = l.WithPrefix([]byte("batch_"))
}


//add (queue for counting)
func Add(p types.Ballot) error {
	err := bdb.Put([]byte(fmt.Sprintf("%v", p.Nullifier)),[]byte(fmt.Sprintf("%v", p)))
	if err != nil {
		return err
	}

	//this actually needs to see if it was added
	currentBatchSize++
	fmt.Println(bdb.Info())
	if currentBatchSize >= BatchSize {
		BatchSignal <- true
	}
	return nil
}

//create (return batch)
//k is []byte nullifier
//v is []byte package
func Create() []byte {
	var b []byte
	var i int
	iter := bdb.Iter()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		err := iter.Error()
		if err != nil {
			panic(err)
		}
		fmt.Println(i)
		fmt.Println(string(k))
		fmt.Println(string(v))
		i++
		//put p in batch
		//rdb.Put(iter.Key(), iter.Val(), nil)
		//bdb.Delete(iter.Key(), nil)
	}
	iter.Release()
	return b
}
