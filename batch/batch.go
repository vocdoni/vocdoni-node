package batch

import (
	"github.com/syndtr/goleveldb/leveldb"
//	"encoding/json"
	"fmt"
	"github.com/vocdoni/dvote-relay/types"
)

var DBPath string
var BDBPath string
var DB *leveldb.DB
var BDB *leveldb.DB
var BatchSignal chan bool
var BatchSize int
var currentBatchSize int
var err error

func Setup() {
	DB, err = leveldb.OpenFile(DBPath, nil)
	if err != nil {
		panic(err)
	}
	defer DB.Close()

	BDB, err = leveldb.OpenFile(BDBPath, nil)
	if err != nil {
		panic(err)
	}
	defer BDB.Close()
	fmt.Println(BDB)
}


//add (queue for counting)
func Add(p types.Packet) error {
	BDB, err = leveldb.OpenFile(BDBPath, nil)
	if err != nil {
		panic(err)
	}
	defer BDB.Close()

	err := BDB.Put([]byte(fmt.Sprintf("%v", p.Nullifier)),[]byte(fmt.Sprintf("%v", p)), nil)
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
	DB, err = leveldb.OpenFile(DBPath, nil)
	if err != nil {
		panic(err)
	}
	defer DB.Close()

	var b []byte

	iter := BDB.NewIterator(nil, nil)
	for iter.Next() {
		err := iter.Error()
		if err != nil {
			panic(err)
		}
		//put p in batch
		//db.Put(iter.Key(), iter.Val(), nil)
		//bdb.Delete(iter.Key(), nil)
	}
	iter.Release()
	return b
}
