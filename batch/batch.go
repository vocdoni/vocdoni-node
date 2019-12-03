package batch

import (
	"encoding/json"

	"gitlab.com/vocdoni/go-dvote/db"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

var (
	rdb         *db.LevelDbStorage
	bdb         *db.LevelDbStorage
	BatchSignal chan bool
	BatchSize   int
	err         error
)

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
			// log error
		}

		err = json.Unmarshal(e.Ballot, &b)
		if err != nil {
			// log error
		}

		err = Add(b)
		if err != nil {
			// log error
		}

		log.Infof("Recieved payload: %v", payload)
	}
}

// add (queue for counting)
func Add(ballot types.Ballot) error {
	// this is probably adding []
	// err := bdb.Put(fmt.Sprintf("%v", p.Nullifier)),[]byte(fmt.Sprintf("%v", p)))
	b, err := json.Marshal(ballot)
	if err != nil {
		return err
	}
	n, err := json.Marshal(ballot.Nullifier)
	if err != nil {
		return err
	}
	if err := bdb.Put(n, b); err != nil {
		return err
	}

	// this actually needs to see if it was added
	if bdb.Count() >= BatchSize {
		BatchSignal <- true
	}
	return nil
}

// create (return batch)
// k is []byte 'batch_' + nullifier
// v is []byte package
// returns slice of nullifiers, batch json
func Fetch() (n, b []string) {
	iter := bdb.Iter()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		err := iter.Error()
		if err != nil {
			log.Panic(err)
		}
		n = append(n, string(k[6:]))
		b = append(b, string(v))
	}
	iter.Release()
	//	jn, err := json.Marshal(n)
	//	if err != nil {
	//		log.Panic(err)
	//	}
	//	jb, err := json.Marshal(b)
	//	if err != nil {
	//		log.Panic(err)
	//	}
	return n, b
}

// move from bdb to rdb once pinned
func Compact(n []string) {
	for _, k := range n {
		v, err := bdb.Get([]byte(k))
		if err != nil {
			log.Error(err)
		}
		rdb.Put([]byte(k), v)
		bdb.Delete([]byte(k))
	}
}
