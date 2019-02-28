package batch

import (
	"encoding/json"
	"fmt"

	"github.com/vocdoni/go-dvote/db"
	"github.com/vocdoni/go-dvote/types"
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

func Recieve(messages <-chan types.Message) {
	var message types.Message
	var payload []byte
	var e types.Envelope
	var b types.Ballot
	for {
		message = <-messages
		payload = message.Data

		err = json.Unmarshal(payload, &e)
		if err != nil {
			//log error
		}

		err = json.Unmarshal(e.Ballot, &b)
		if err != nil {
			//log error
		}

		err = Add(b)
		if err != nil {
			//log error
		}

		fmt.Println("Got > " + string(payload))
	}
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
	err = bdb.Put(n, b)
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
