package data

import (
	"bytes"
	"errors"
	"io/ioutil"

	"gitlab.com/vocdoni/go-dvote/swarm"
	"gitlab.com/vocdoni/go-dvote/types"
)

type BZZHandle struct {
	d *types.DataStore
	s *swarm.SimpleSwarm
	c *BZZConfig
}

type BZZConfig struct {
}

func (c *BZZConfig) Type() StorageID {
	return BZZ
}

func BZZNewConfig() *types.DataStore {
	cfg := new(types.DataStore)
	return cfg
}

func (b *BZZHandle) Init(d *types.DataStore) error {
	b.d = d
	sn := new(swarm.SimpleSwarm)
	err := sn.InitBZZ()
	if err != nil {
		return err
	}
	b.s = sn
	return nil
}

func (b *BZZHandle) GetURIprefix() string {
	return "bzz://"
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

//STUB -- NEEDS IMPLEMENTATION
func (b *BZZHandle) Pin(path string) error {
	return errors.New("Not yet implemented in BZZ")
}

func (b *BZZHandle) Stats() (string, error) {
	return "", errors.New("Not yet implemented in BZZ")
}

//STUB -- NEEDS IMPLEMENTATION
func (b *BZZHandle) Unpin(path string) error {
	return errors.New("Not yet implemented in BZZ")
}

//STUB -- NEEDS IMPLEMENTATION
func (b *BZZHandle) ListPins() (map[string]string, error) {
	return nil, errors.New("Not yet implemented in BZZ")
}
