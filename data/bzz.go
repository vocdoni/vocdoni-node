package data

import (
	"github.com/vocdoni/go-dvote/swarm"
	"github.com/vocdoni/go-dvote/types"
)

type BZZHandle struct {
	d *types.DataStore
	s *swarm.SimpleSwarm
}

func (b *BZZHandle) Init(d *types.DataStore) error {
	b.d = d
	sn := new(swarm.SimpleSwarm)
	err := sn.Init()
	if err != nil {
		return err
	}
	err = sn.SetLog("crit")
	if err != nil {
		return err
	}
	b.s = sn
	return nil
}

func (b *BZZHandle) Publish(object []byte) string {
	//publish, return hash
	//return cid
	//b.a.Store()
	return ""
}

func (b *BZZHandle) Retrieve(hash string) []byte {
	//fetch content by cid
	//return content
	//b.a.Rretrieve()
	var dummy []byte
	return dummy
}
