package data

import (
	"bytes"
	"io/ioutil"

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
	err := sn.InitBZZ()
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

func (b *BZZHandle) Publish(object []byte) (string, error) {
	hash, err := b.s.Client.UploadRaw(bytes.NewReader(object), int64(len(object)), false)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (b *BZZHandle) Retrieve(hash string) ([]byte, error) {
	reader, _, err := b.s.Client.DownloadRaw(hash)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return data, nil
}
