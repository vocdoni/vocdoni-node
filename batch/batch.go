package batch

import (
	"github.com/syndtr/goleveldb/leveldb"
//	"encoding/json"
	"fmt"
	"github.com/vocdoni/dvote-relay/types"
)

//add (queue for counting)
func Add(p types.Packet, bdb *leveldb.DB) error {
	err := bdb.Put([]byte(fmt.Sprintf("%v", p.Nullifier)),[]byte(fmt.Sprintf("%v", p)), nil)
	if err != nil {
		return err
	}
	return nil
}

//create (return batch)
func Create(bdb *leveldb.DB) []byte {
	var b []byte

	iter := bdb.NewIterator(nil, nil)
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
