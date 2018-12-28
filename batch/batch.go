package batch

import (
	"fmt"
	"github.com/vocdoni/dvote-relay/types"
	"github.com/vocdoni/dvote-relay/db"
	"encoding/json"
)

var rdb *db.LevelDbStorage
var bdb *db.LevelDbStorage
var BatchSignal chan bool
var BatchSize int
var err error

func Setup(l *db.LevelDbStorage) {
	rdb = l.WithPrefix([]byte("relay_"))
	bdb = l.WithPrefix([]byte("batch_"))
}


//add (queue for counting)
func Add(ballot types.Ballot) error {
	//this is probably adding []
	//err := bdb.Put(fmt.Sprintf("%v", p.Nullifier)),[]byte(fmt.Sprintf("%v", p)))
	var b []byte
	var n []byte
	var err error
	b, err = json.Marshal(ballot)
	n, err = json.Marshal(ballot.Nullifier)
	err = bdb.Put(n,b)
	if err != nil {
		return err
	}

	//this actually needs to see if it was added
	if bdb.Count() >= BatchSize {
		BatchSignal <- true
	}
	return nil
}

//create (return batch)
//k is []byte 'batch_' + nullifier
//v is []byte package
//returns slice of nullifiers, batch json
func Fetch() ([]string, []string) {
	var n []string
	var b []string
	iter := bdb.Iter()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		err := iter.Error()
		if err != nil {
			panic(err)
		}
		n = append(n, string(k[6:]))
		b = append(b, string(v))
	}
	iter.Release()
//	jn, err := json.Marshal(n)
//	if err != nil {
//		panic(err)
//	}
//	jb, err := json.Marshal(b)
//	if err != nil {
//		panic(err)
//	}
	return n, b
}

//move from bdb to rdb once pinned
func Compact(n []string) {
	for _, k := range n {
		//fmt.Println(k)
		v, err := bdb.Get([]byte(k))
		if err != nil {
			fmt.Println(err.Error())
		}
		rdb.Put([]byte(k), v)
		bdb.Delete([]byte(k))
	}
}
